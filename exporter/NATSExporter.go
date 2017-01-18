// Copyright 2016 Apcera Inc. All rights reserved.

package exporter

import (
	"net"
	"net/http"
	"strconv"
	"strings"
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
	MonitorURLs   []string
	RetryInterval time.Duration
}

//NATSExporter collects NATS metrics
type NATSExporter struct {
	sync.Mutex
	opts       *NATSExporterOptions
	doneWg     sync.WaitGroup
	http       net.Listener
	collectors []*collector.NATSCollector
	running    bool
}

// Defaults
var (
	DefaultListenPort        = 7777
	DefaultListenAddress     = "0.0.0.0"
	DefaultMonitorURL        = "http://localhost:8222/varz"
	DefaultRetryIntervalSecs = 30
)

// GetDefaultExporterOptions returns the default set of exporter options
func GetDefaultExporterOptions() *NATSExporterOptions {
	u := []string{DefaultMonitorURL}
	opts := &NATSExporterOptions{
		ListenAddress: DefaultListenAddress,
		ListenPort:    DefaultListenPort,
		MonitorURLs:   u,
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
		opts:       o,
		http:       nil,
		collectors: make([]*collector.NATSCollector, 0),
	}
	return ne
}

func (ne *NATSExporter) scheduleRetry(urls []string) {
	time.Sleep(ne.opts.RetryInterval)
	ne.Lock()
	ne.createCollector(urls)
	ne.Unlock()
}

// Caller must lock.
func (ne *NATSExporter) createCollector(urls []string) {
	collector.Debugf("Registering server: %s", urls)
	nc := collector.NewCollector(urls)
	if err := prometheus.Register(nc); err != nil {
		collector.Debugf("Unable to register server %s (%v), Retrying.", urls, err)
		go ne.scheduleRetry(urls)
	} else {
		ne.collectors = append(ne.collectors, nc)
	}
}

// Start runs the exporter process
func (ne *NATSExporter) Start() error {
	var err error
	ne.Lock()
	defer ne.Unlock()
	if ne.running {
		return nil
	}

	ne.doneWg.Add(1)

	opts := ne.opts
	collector.ConfigureLogger(&opts.LoggerOptions)
	metrics := make(map[string][]string)

	// for each URL in args
	for _, arg := range ne.opts.MonitorURLs {
		// create a map of possible metric endpoints.
		endpoint := arg[strings.LastIndex(arg, "/")+1:] // TODO: not safe.
		// append the url to that endpoint.
		metrics[endpoint] = append(metrics[endpoint], arg)
	}

	for _, v := range metrics {
		ne.createCollector(v)
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
		collector.Fatalf("can't listen to the monitor port: %v", err)
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
		srv.Handler = nil
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
	ne.Lock()
	defer ne.Unlock()

	if !ne.running {
		return
	}

	ne.running = false
	ne.http.Close()
	for _, c := range ne.collectors {
		collector.Tracef("Unregistering servers: %s", c.URLs)
		prometheus.Unregister(c)
	}
	ne.collectors = nil
	ne.doneWg.Done()
}
