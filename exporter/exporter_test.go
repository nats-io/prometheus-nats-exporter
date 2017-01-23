// Copyright 2016 Apcera Inc. All rights reserved.

package exporter

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"strings"

	pet "github.com/nats-io/prometheus-nats-exporter/test"
)

func checkExporter(url string) error {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", url))
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
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetSubz = true
	opts.GetRoutez = true

	exp := NewExporter(opts)
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server: %v", err)
	}
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if err := checkExporter(exp.http.Addr().String()); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterWait(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server: %v", err)
	}
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}

	if err := checkExporter(exp.http.Addr().String()); err != nil {
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
	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.RetryInterval = 1 * time.Second
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server: %v", err)
	}
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()

	if err := checkExporter(exp.http.Addr().String()); err == nil {
		t.Fatalf("Expected an error, received none.")
	}

	// allow for a few retries.
	time.Sleep(opts.RetryInterval * 2)

	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	time.Sleep(opts.RetryInterval + (time.Millisecond * 500))

	if err := checkExporter(exp.http.Addr().String()); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterAPIIdempotency(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server: %v", err)
	}

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
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server: %v", err)
	}

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	if err := checkExporter(exp.http.Addr().String()); err != nil {
		t.Fatalf("%v", err)
	}
	// test stop
	exp.Stop()
	if err := checkExporter(exp.http.Addr().String()); err == nil {
		t.Fatalf("Did not received expected error")
	}

	time.Sleep(500 * time.Millisecond)

	// restart
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()
	if err := checkExporter(exp.http.Addr().String()); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterStartNoServersConfigured(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true

	exp := NewExporter(opts)
	// do not configura a server
	if err := exp.Start(); err == nil {
		t.Fatalf("Did not receive expected start failure.")
	}

	// now add a server
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Error adding a server.")
	}

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()
	if err := checkExporter(exp.http.Addr().String()); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterStartNoMetricsSelected(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	// No metrics defined opts.GetVarz = true

	exp := NewExporter(opts)

	// now add a server
	if err := exp.AddServer("test-server", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if err := exp.Start(); err == nil {
		t.Fatalf("Did not receive expected error adding a server.")
	}

}
