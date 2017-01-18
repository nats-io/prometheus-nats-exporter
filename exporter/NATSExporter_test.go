// Copyright 2016 Apcera Inc. All rights reserved.

package exporter

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"strings"

	"github.com/nats-io/gnatsd/server"
)

const ClientPort = 11224
const MonitorPort = 11424

var DefaultMonitorOptions = server.Options{
	Host:     "localhost",
	Port:     ClientPort,
	HTTPHost: "127.0.0.1",
	HTTPPort: MonitorPort,
	NoLog:    true,
	NoSigs:   true,
}

func runMonitorServer() *server.Server {
	resetPreviousHTTPConnections()
	opts := DefaultMonitorOptions
	return RunServer(&opts)
}

// New Go Routine based server
func RunServer(opts *server.Options) *server.Server {
	if opts == nil {
		opts = &DefaultMonitorOptions
	}
	s := server.New(opts)
	if s == nil {
		panic("No NATS Server object returned.")
	}

	// Run server in Go routine.
	go s.Start()

	end := time.Now().Add(10 * time.Second)
	for time.Now().Before(end) {
		netAddr := s.Addr()
		if netAddr == nil {
			continue
		}
		addr := s.Addr().String()
		if addr == "" {
			time.Sleep(10 * time.Millisecond)
			// Retry. We might take a little while to open a connection.
			continue
		}
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			// Retry after 50ms
			time.Sleep(50 * time.Millisecond)
			continue
		}
		conn.Close()
		// Wait a bit to give a chance to the server to remove this
		// "client" from its state, which may otherwise interfere with
		// some tests.
		time.Sleep(25 * time.Millisecond)

		return s
	}
	panic("Unable to start NATS Server in Go Routine")

}

func resetPreviousHTTPConnections() {
	http.DefaultTransport = &http.Transport{}
}

func checkExporter() error {
	resp, err := http.Get("http://127.0.0.1:8888/metrics")
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("Expected a 200 response, got %d\n", resp.StatusCode)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Got an error reading the body: %v\n", err)
	}

	if !strings.Contains(string(body), "gnatsd_varz_connections") {
		return fmt.Errorf("Response did not have NATS data")
	}
	return nil
}

func TestExporter(t *testing.T) {
	var opts *NATSExporterOptions
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/varz", MonitorPort)
	opts = GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.MonitorURLs = []string{url}

	exp := NewExporter(opts)
	exp.Start()
	defer exp.Stop()

	if err := checkExporter(); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterWait(t *testing.T) {
	var opts *NATSExporterOptions
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/varz", MonitorPort)
	opts = &NATSExporterOptions{}
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888

	opts.MonitorURLs = []string{url}
	exp := NewExporter(opts)
	exp.Start()

	if err := checkExporter(); err != nil {
		t.Fatalf("%v", err)
	}

	var didStop int32
	go func() {
		time.Sleep(time.Second * 1)
		exp.Stop()
		atomic.AddInt32(&didStop, 1)
	}()
	exp.WaitUntilDone()
	if atomic.LoadInt32(&didStop) == 0 {
		t.Fatalf("did not wait until completed.")
	}
}

func TestExporterNoNATSServer(t *testing.T) {
	var opts *NATSExporterOptions

	url := fmt.Sprintf("http://localhost:%d/varz", MonitorPort)
	opts = GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.RetryInterval = 1 * time.Second
	opts.MonitorURLs = []string{url}
	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()

	if err := checkExporter(); err == nil {
		t.Fatalf("Expected an error, received none.")
	}

	// allow for a few retries.
	time.Sleep(opts.RetryInterval * 2)

	// start the server
	s := runMonitorServer()
	defer s.Shutdown()

	time.Sleep(opts.RetryInterval + (time.Millisecond * 500))

	if err := checkExporter(); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterAPIIdempotency(t *testing.T) {
	var opts *NATSExporterOptions

	// start the server
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/varz", MonitorPort)
	opts = GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.RetryInterval = 1 * time.Second

	opts.MonitorURLs = []string{url}

	exp := NewExporter(opts)

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}

	// test stop
	exp.Stop()
	exp.Stop()
}

func TestExporterBounce(t *testing.T) {
	var opts *NATSExporterOptions

	// start the server
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/varz", MonitorPort)
	opts = GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.RetryInterval = 1 * time.Second

	opts.MonitorURLs = []string{url}

	exp := NewExporter(opts)

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	if err := checkExporter(); err != nil {
		t.Fatalf("%v", err)
	}
	// test stop
	exp.Stop()
	if err := checkExporter(); err == nil {
		t.Fatalf("Did not received expected error")
	}

	time.Sleep(500 * time.Millisecond)

	// restart
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	if err := checkExporter(); err != nil {
		t.Fatalf("%v", err)
	}
}
