// Copyright 2017-2024 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package exporter is a Prometheus exporter for NATS.
package exporter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/crypto/bcrypt"
)

const (
	modeStarted uint8 = iota + 1
	modeStopped
)

// NATSExporterOptions are options to configure the NATS collector
type NATSExporterOptions struct {
	collector.LoggerOptions
	ListenAddress           string
	ListenPort              int
	ScrapePath              string
	GetHealthz              bool
	GetHealthzJsEnabledOnly bool
	GetHealthzJsServerOnly  bool
	GetConnz                bool
	GetConnzDetailed        bool
	GetVarz                 bool
	GetSubz                 bool
	GetRoutez               bool
	GetGatewayz             bool
	GetAccstatz             bool
	GetLeafz                bool
	GetReplicatorVarz       bool
	GetJszFilter            string
	RetryInterval           time.Duration
	CertFile                string
	KeyFile                 string
	CaFile                  string
	NATSServerURL           string
	NATSServerTag           string
	HTTPUser                string // User in metrics scrape by prometheus.
	HTTPPassword            string
	Prefix                  string
	UseInternalServerID     bool
	UseServerName           bool
}

// NATSExporter collects NATS metrics
type NATSExporter struct {
	sync.Mutex
	registry   *prometheus.Registry
	opts       *NATSExporterOptions
	doneWg     sync.WaitGroup
	http       *http.Server
	addr       string
	Collectors []prometheus.Collector
	servers    []*collector.CollectedServer
	mode       uint8
}

// Defaults
var (
	DefaultListenPort        = 7777
	DefaultListenAddress     = "0.0.0.0"
	DefaultScrapePath        = "/metrics"
	DefaultMonitorURL        = "http://localhost:8222"
	DefaultRetryIntervalSecs = 30

	// bcryptPrefix from gnatsd
	bcryptPrefix = "$2a$"
)

// GetDefaultExporterOptions returns the default set of exporter options
// The NATS server url must be set
func GetDefaultExporterOptions() *NATSExporterOptions {
	opts := &NATSExporterOptions{
		ListenAddress: DefaultListenAddress,
		ListenPort:    DefaultListenPort,
		ScrapePath:    DefaultScrapePath,
		RetryInterval: time.Duration(DefaultRetryIntervalSecs) * time.Second,
	}
	return opts
}

// NewExporter creates a new NATS exporter
func NewExporter(opts *NATSExporterOptions) *NATSExporter {
	o := opts
	if o == nil {
		o = GetDefaultExporterOptions()
		o.GetVarz = true
	}
	collector.ConfigureLogger(&o.LoggerOptions)
	ne := &NATSExporter{
		opts: o,
		http: nil,
	}
	if o.NATSServerURL != "" {
		_ = ne.AddServer(o.NATSServerTag, o.NATSServerURL)
	}
	return ne
}

func (ne *NATSExporter) createCollector(system, endpoint string) {
	ne.registerCollector(system, endpoint,
		collector.NewCollector(system, endpoint,
			ne.opts.Prefix,
			ne.servers))
}

func (ne *NATSExporter) registerCollector(system, endpoint string, nc prometheus.Collector) {
	if err := ne.registry.Register(nc); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			collector.Errorf("A collector for this server's metrics has already been registered.")
		} else {
			collector.Debugf("Unable to register collector %s (%v), Retrying.", endpoint, err)
			time.AfterFunc(ne.opts.RetryInterval, func() {
				collector.Debugf("Creating a collector for endpoint: %s", endpoint)
				ne.Lock()
				ne.createCollector(system, endpoint)
				ne.Unlock()
			})
		}
	} else {
		collector.Debugf("Registered collector for system %s, endpoint: %s", system, endpoint)
		ne.Collectors = append(ne.Collectors, nc)
	}
}

// AddServer is an advanced API; normally the NATS server should be set
// through the options.  Adding more than one server will
// violate Prometheus.io guidelines.
func (ne *NATSExporter) AddServer(id, url string) error {
	ne.Lock()
	defer ne.Unlock()

	if ne.mode == modeStarted {
		return fmt.Errorf("servers cannot be added after the exporter is started")
	}
	cs := &collector.CollectedServer{ID: id, URL: url}
	if ne.servers == nil {
		ne.servers = make([]*collector.CollectedServer, 0)
	}
	ne.servers = append(ne.servers, cs)
	return nil
}

// InitializeCollectors initializes the Collectors for the exporter.
// Caller must lock
func (ne *NATSExporter) InitializeCollectors() error {
	opts := ne.opts

	if len(ne.servers) == 0 {
		return fmt.Errorf("no servers configured to obtain metrics")
	}

	if opts.GetReplicatorVarz && opts.GetVarz {
		return fmt.Errorf("replicatorVarz cannot be used with varz")
	}
	if opts.GetSubz {
		ne.createCollector(collector.CoreSystem, "subsz")
	}
	if opts.GetVarz {
		ne.createCollector(collector.CoreSystem, "varz")
	}
	if opts.GetHealthz {
		ne.createCollector(collector.CoreSystem, "healthz")
	}
	if opts.GetHealthzJsEnabledOnly {
		ne.createCollector(collector.CoreSystem, "healthz_js_enabled_only")
	}
	if opts.GetHealthzJsServerOnly {
		ne.createCollector(collector.CoreSystem, "healthz_js_server_only")
	}
	if opts.GetConnzDetailed {
		ne.createCollector(collector.CoreSystem, "connz_detailed")
	} else if opts.GetConnz {
		ne.createCollector(collector.CoreSystem, "connz")
	}
	if opts.GetGatewayz {
		ne.createCollector(collector.CoreSystem, "gatewayz")
	}
	if opts.GetAccstatz {
		ne.createCollector(collector.CoreSystem, "accstatz")
	}
	if opts.GetLeafz {
		ne.createCollector(collector.CoreSystem, "leafz")
	}
	if opts.GetRoutez {
		ne.createCollector(collector.CoreSystem, "routez")
	}
	if opts.GetReplicatorVarz {
		ne.createCollector(collector.ReplicatorSystem, "varz")
	}
	if opts.GetJszFilter != "" {
		switch strings.ToLower(opts.GetJszFilter) {
		case "account", "accounts", "consumer", "consumers", "all", "stream", "streams":
		default:
			return fmt.Errorf("invalid jsz filter %q", opts.GetJszFilter)
		}
		ne.createCollector(collector.JetStreamSystem, opts.GetJszFilter)
	}
	if len(ne.Collectors) == 0 {
		return fmt.Errorf("no Collectors specified")
	}

	return nil
}

// ClearCollectors unregisters the collectors
// caller must lock
func (ne *NATSExporter) ClearCollectors() {
	if ne.Collectors != nil {
		for _, c := range ne.Collectors {
			ne.registry.Unregister(c)
		}
		ne.Collectors = nil
	}
}

// Start runs the exporter process.
func (ne *NATSExporter) Start() error {
	ne.Lock()
	defer ne.Unlock()
	if ne.mode == modeStarted {
		return nil
	}
	// Since we are adding metrics in runtime, we need to use a custom registry
	// instead of the default one. This is because the default registry is
	// global and collectors cannot be properly re-registered after being
	// modified.
	if ne.registry == nil {
		ne.registry = prometheus.NewRegistry()
	}
	if err := ne.registry.Register(collectors.NewGoCollector()); err != nil {
		if errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			collector.Debugf("GoCollector already registered")
		} else {
			return fmt.Errorf("error registering GoCollector: %v", err)
		}
	}
	if err := ne.registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})); err != nil {
		if errors.As(err, &prometheus.AlreadyRegisteredError{}) {
			collector.Debugf("ProcessCollector already registered")
		} else {
			return fmt.Errorf("error registering GoCollector: %v", err)
		}
	}

	if err := ne.InitializeCollectors(); err != nil {
		ne.ClearCollectors()
		return err
	}

	if err := ne.startHTTP(); err != nil {
		ne.ClearCollectors()
		return fmt.Errorf("error serving http:  %v", err)
	}

	ne.doneWg.Add(1)
	ne.mode = modeStarted

	return nil
}

// generates the TLS config for https
func (ne *NATSExporter) generateTLSConfig() (*tls.Config, error) {
	//  Load in cert and private key
	cert, err := tls.LoadX509KeyPair(ne.opts.CertFile, ne.opts.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing X509 certificate/key pair (%s, %s): %v",
			ne.opts.CertFile, ne.opts.KeyFile, err)
	}
	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate (%s): %v",
			ne.opts.CertFile, err)
	}
	// Create our TLS configuration
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}
	// Add in CAs if applicable.
	if ne.opts.CaFile != "" {
		rootPEM, err := os.ReadFile(ne.opts.CaFile)
		if err != nil || rootPEM == nil {
			return nil, fmt.Errorf("failed to load root ca certificate (%s): %v", ne.opts.CaFile, err)
		}
		pool := x509.NewCertPool()
		ok := pool.AppendCertsFromPEM(rootPEM)
		if !ok {
			return nil, fmt.Errorf("failed to parse root ca certificate")
		}
		config.ClientCAs = pool
	}
	return config, nil
}

// isBcrypt checks whether the given password or token is bcrypted.
func isBcrypt(password string) bool {
	return strings.HasPrefix(password, bcryptPrefix)
}

func (ne *NATSExporter) isValidUserPass(user, password string) bool {
	if user != ne.opts.HTTPUser {
		return false
	}
	exporterPassword := ne.opts.HTTPPassword
	if isBcrypt(exporterPassword) {
		if err := bcrypt.CompareHashAndPassword([]byte(exporterPassword), []byte(password)); err != nil {
			return false
		}
	} else if exporterPassword != password {
		return false
	}
	return true
}

// getScrapeHandler returns the default handler if no nttp
// auhtorization has been specificed.  Otherwise, it checks
// basic authorization.
func (ne *NATSExporter) getScrapeHandler() http.Handler {
	h := promhttp.InstrumentMetricHandler(
		ne.registry, promhttp.HandlerFor(ne.registry, promhttp.HandlerOpts{}),
	)

	if ne.opts.HTTPUser != "" {
		return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
			auth := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
			if len(auth) != 2 || auth[0] != "Basic" {
				http.Error(rw, "authorization failed", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(auth[1])
			if err != nil {
				http.Error(rw, "authorization failed", http.StatusBadRequest)
				return
			}
			pair := strings.SplitN(string(payload), ":", 2)

			if len(pair) != 2 || !ne.isValidUserPass(pair[0], pair[1]) {
				http.Error(rw, "authorization failed", http.StatusUnauthorized)
				return
			}

			h.ServeHTTP(rw, r)
		})
	}

	return h
}

// startHTTP configures and starts the HTTP server for applications to poll data from
// exporter.
// caller must lock
func (ne *NATSExporter) startHTTP() error {
	var hp string
	var path string
	var err error
	var proto string
	var config *tls.Config

	hp = net.JoinHostPort(ne.opts.ListenAddress, strconv.Itoa(ne.opts.ListenPort))
	path = ne.opts.ScrapePath

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	var listener net.Listener
	// If a certificate file has been specified, setup TLS with the
	// key provided.
	if ne.opts.CertFile != "" {
		proto = "https"
		collector.Debugf("Certificate file specfied; using https.")
		config, err = ne.generateTLSConfig()
		if err != nil {
			return err
		}
		listener, err = tls.Listen("tcp", hp, config)
	} else {
		proto = "http"
		collector.Debugf("No certificate file specified; using http.")
		listener, err = net.Listen("tcp", hp)
	}

	collector.Noticef("Prometheus exporter listening at %s://%s%s", proto, hp, path)

	if err != nil {
		collector.Errorf("can't start HTTP listener: %v", err)
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(path, ne.getScrapeHandler())

	srv := &http.Server{
		Addr:           hp,
		Handler:        mux,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      config,
	}
	ne.http = srv
	ne.addr = listener.Addr().String()

	go func() {
		for i := 0; i < 10; i++ {
			var err error
			if err = srv.Serve(listener); err != nil {
				// In a test environment, this can fail because the server is
				// already running.

				srvState := ne.getMode()
				if srvState == modeStopped {
					collector.Debugf("Server has been stopped, skipping reconnects")
					return
				}
				collector.Debugf("Unable to start HTTP server (mode=%d): %v", srvState, err)
			} else {
				collector.Debugf("Started HTTP server.")
			}
		}
	}()

	return nil
}

func (ne *NATSExporter) getMode() uint8 {
	ne.Lock()
	mode := ne.mode
	ne.Unlock()
	return mode
}

// WaitUntilDone blocks until the collector is stopped.
func (ne *NATSExporter) WaitUntilDone() {
	ne.Lock()
	wg := &ne.doneWg
	ne.Unlock()

	wg.Wait()
}

// Stop stops the collector.
func (ne *NATSExporter) Stop() {
	collector.Debugf("Stopping.")
	ne.Lock()
	defer ne.Unlock()

	if ne.mode == modeStopped {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ne.http.Shutdown(ctx); err != nil {
		collector.Debugf("Did not close HTTP: %v", err)
	}
	ne.ClearCollectors()
	ne.registry = nil
	ne.doneWg.Done()
	ne.mode = modeStopped
}
