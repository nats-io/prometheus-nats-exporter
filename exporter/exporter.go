// Copyright 2016 Apcera Inc. All rights reserved.

package exporter

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
)

// NATSExporterOptions are options to set the NATS collector
type NATSExporterOptions struct {
	collector.LoggerOptions
	ListenAddress string
	ListenPort    int
	GetConnz      bool
	GetVarz       bool
	GetSubz       bool
	GetRoutez     bool
	RetryInterval time.Duration
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
func GetDefaultExporterOptions() *NATSExporterOptions {
	opts := &NATSExporterOptions{
		ListenAddress: DefaultListenAddress,
		ListenPort:    DefaultListenPort,
		RetryInterval: time.Duration(DefaultRetryIntervalSecs) * time.Second,
	}
	return opts
}

// NewExporter creates a new NATS exporte from a list of monitoring URLs.
// Each URL should be to a specific endpoint (e.g. /varz, /connz, subsz, or routez)
func NewExporter(opts *NATSExporterOptions) *NATSExporter {
	o := opts
	if o == nil {
		o = GetDefaultExporterOptions()
	}
	ne := &NATSExporter{
		opts: o,
		http: nil,
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

// AddServer adds a NATS server to the exporter.  The exporter cannot be
// running.
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

// Start runs the exporter process.  Servers should be addded with the
// AddServer API. before starting it.
func (ne *NATSExporter) Start() error {
	var err error
	ne.Lock()
	defer ne.Unlock()
	if ne.running {
		return nil
	}

	if len(ne.servers) == 0 {
		return fmt.Errorf("no servers configured to obtain metrics")
	}

	opts := ne.opts
	if !opts.GetConnz && !opts.GetRoutez && !opts.GetSubz && !opts.GetVarz {
		return fmt.Errorf("no collectors specfied")
	}

	ne.doneWg.Add(1)
	collector.ConfigureLogger(&opts.LoggerOptions)
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

	if err = ne.startHTTP(opts.ListenAddress, opts.ListenPort); err != nil {
		collector.Fatalf("Error serving http:  %v", err)
	}

	ne.running = true

	return err
}

// caller must lock
func (ne *NATSExporter) startHTTP(listenAddress string, listenPort int) error {
	var hp string
	var err error

	/* TODO:  setup TLS
	if secure {
		hp = net.JoinHostPort(s.opts.HTTPHost, strconv.Itoa(s.opts.HTTPSPort))
		config := opts.TLSConfig)
		config.ClientAuth = tls.NoClientCert
		s.http, err = tls.Listen("tcp", hp, config)

	} else {
		hp = net.JoinHostPort(s.opts.HTTPHost, strconv.Itoa(s.opts.HTTPPort))
		s.http, err = net.Listen("tcp", hp)
	}
	*/
	hp = net.JoinHostPort(ne.opts.ListenAddress, strconv.Itoa(ne.opts.ListenPort))
	collector.Noticef("Prometheus exporter listening at %s", hp)
	ne.http, err = net.Listen("tcp", hp)

	if err != nil {
		collector.Fatalf("can't listen to the export port: %v", err)
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", prometheus.Handler())

	srv := &http.Server{
		Addr:           hp,
		Handler:        mux,
		ReadTimeout:    2 * time.Second,
		WriteTimeout:   2 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	sHTTP := ne.http
	go func() {
		srv.Serve(sHTTP)
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

// Stop stops the collector
func (ne *NATSExporter) Stop() {
	collector.Debugf("Stopping.")
	ne.Lock()
	defer ne.Unlock()

	if !ne.running {
		return
	}

	ne.running = false
	ne.http.Close()
	for _, c := range ne.collectors {
		prometheus.Unregister(c)
	}
	ne.collectors = nil
	ne.doneWg.Done()
}
