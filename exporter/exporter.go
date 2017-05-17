// Copyright 2017 Apcera Inc. All rights reserved.

package exporter

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NATSExporterOptions are options to configure the NATS collector
type NATSExporterOptions struct {
	collector.LoggerOptions
	ListenAddress string
	ListenPort    int
	GetConnz      bool
	GetVarz       bool
	GetSubz       bool
	GetRoutez     bool
	RetryInterval time.Duration
	CertFile      string
	KeyFile       string
	CaFile        string
	NATSServerURL string
	NATSServerTag string
}

//NATSExporter collects NATS metrics
type NATSExporter struct {
	sync.Mutex
	opts       *NATSExporterOptions
	doneWg     sync.WaitGroup
	http       net.Listener
	collectors []*collector.NATSCollector
	servers    []*collector.CollectedServer
	running    bool
}

// Defaults
var (
	DefaultListenPort        = 7777
	DefaultListenAddress     = "0.0.0.0"
	DefaultMonitorURL        = "http://localhost:8222"
	DefaultRetryIntervalSecs = 30
)

// GetDefaultExporterOptions returns the default set of exporter options
// The NATS server url must be set
func GetDefaultExporterOptions() *NATSExporterOptions {
	opts := &NATSExporterOptions{
		ListenAddress: DefaultListenAddress,
		ListenPort:    DefaultListenPort,
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
	ne := &NATSExporter{
		opts: o,
		http: nil,
	}
	if o.NATSServerURL != "" {
		_ = ne.AddServer(o.NATSServerTag, o.NATSServerURL) // nolint
	}
	return ne
}

func (ne *NATSExporter) scheduleRetry(endpoint string) {
	time.Sleep(ne.opts.RetryInterval)
	ne.Lock()
	ne.createCollector(endpoint)
	ne.Unlock()
}

// Caller must lock.
func (ne *NATSExporter) createCollector(endpoint string) {
	collector.Debugf("Creating a collector for endpoint: %s.", endpoint)
	nc := collector.NewCollector(endpoint, ne.servers)
	if err := prometheus.Register(nc); err != nil {
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			collector.Errorf("A collector for this server's metrics has already been registered.")
		} else {
			collector.Debugf("Unable to register collector %s (%v), Retrying.", endpoint, err)
			go ne.scheduleRetry(endpoint)
		}
	} else {
		collector.Debugf("Registered collector for endppoint %s.", endpoint)
		ne.collectors = append(ne.collectors, nc)
	}
}

// AddServer is an advanced API for Apcera's use; normally the NATS server
// should be set through the options.  Adding more than one server will
// violate Prometheus.io guidelines.
func (ne *NATSExporter) AddServer(id, url string) error {
	ne.Lock()
	defer ne.Unlock()

	if ne.running {
		return fmt.Errorf("servers cannot be added after the exporter is started")
	}
	cs := &collector.CollectedServer{ID: id, URL: url}
	if ne.servers == nil {
		ne.servers = make([]*collector.CollectedServer, 0)
	}
	ne.servers = append(ne.servers, cs)
	return nil
}

// initializeCollectors initializes the collectors for the exporter.
// Caller must lock
func (ne *NATSExporter) initializeCollectors() error {
	opts := ne.opts
	collector.ConfigureLogger(&opts.LoggerOptions)

	if len(ne.servers) == 0 {
		return fmt.Errorf("no servers configured to obtain metrics")
	}

	if !opts.GetConnz && !opts.GetRoutez && !opts.GetSubz && !opts.GetVarz {
		return fmt.Errorf("no collectors specfied")
	}
	if opts.GetSubz {
		ne.createCollector("subsz")
	}
	if opts.GetVarz {
		ne.createCollector("varz")
	}
	if opts.GetConnz {
		ne.createCollector("connz")
	}
	if opts.GetRoutez {
		ne.createCollector("routez")
	}
	return nil
}

// caller must lock
func (ne *NATSExporter) clearCollectors() {
	if ne.collectors != nil {
		for _, c := range ne.collectors {
			prometheus.Unregister(c)
		}
		ne.collectors = nil
	}
}

// Start runs the exporter process.
func (ne *NATSExporter) Start() error {
	ne.Lock()
	defer ne.Unlock()
	if ne.running {
		return nil
	}

	if err := ne.initializeCollectors(); err != nil {
		ne.clearCollectors()
		return err
	}

	if err := ne.startHTTP(ne.opts.ListenAddress, ne.opts.ListenPort); err != nil {
		ne.clearCollectors()
		return fmt.Errorf("Error serving http:  %v", err)
	}

	ne.doneWg.Add(1)
	ne.running = true

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
		rootPEM, err := ioutil.ReadFile(ne.opts.CaFile)
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

// startHTTP configures and starts the HTTP server for applications to poll data from
// exporter.
// caller must lock
func (ne *NATSExporter) startHTTP(listenAddress string, listenPort int) error {
	var hp string
	var err error
	var proto string
	var config *tls.Config

	hp = net.JoinHostPort(ne.opts.ListenAddress, strconv.Itoa(ne.opts.ListenPort))

	// If a certificate file has been specified, setup TLS with the
	// key provided.
	if ne.opts.CertFile != "" {
		proto = "https"
		collector.Debugf("Certificate file specfied; using https.")
		config, err = ne.generateTLSConfig()
		if err != nil {
			return err
		}
		ne.http, err = tls.Listen("tcp", hp, config)
	} else {
		proto = "http"
		collector.Debugf("No certificate file specified; using http.")
		ne.http, err = net.Listen("tcp", hp)
	}

	collector.Noticef("Prometheus exporter listening at %s://%s/metrics", proto, hp)

	if err != nil {
		collector.Errorf("can't start HTTP listener: %v", err)
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{
		Addr:           hp,
		Handler:        mux,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      config,
	}

	sHTTP := ne.http
	go func() {
		for i := 0; i < 10; i++ {
			var err error
			if err = srv.Serve(sHTTP); err != nil {
				// In a test environment, this can fail because the server is already running.
				collector.Debugf("Unable to start HTTP server (may already be running): %v", err)
			} else {
				collector.Debugf("Started HTTP server.")
			}
		}
	}()

	return nil
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

	if !ne.running {
		return
	}

	ne.running = false
	if err := ne.http.Close(); err != nil {
		collector.Debugf("Did not close HTTP: %v", err)
	}
	ne.clearCollectors()
	ne.doneWg.Done()
}
