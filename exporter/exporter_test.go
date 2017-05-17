// Copyright 2017 Apcera Inc. All rights reserved.

package exporter

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"strings"

	pet "github.com/nats-io/prometheus-nats-exporter/test"
)

const (
	clientCert = "../test/certs/client-cert.pem"
	clientKey  = "../test/certs/client-key.pem"
	serverCert = "../test/certs/server-cert.pem"
	serverKey  = "../test/certs/server-key.pem"
	caCertFile = "../test/certs/ca.pem"
)

func getDefaultExporterTestOptions() (opts *NATSExporterOptions) {
	o := GetDefaultExporterOptions()
	o.NATSServerTag = "test-server"
	o.NATSServerURL = fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	return o
}

func getSecure(t *testing.T, url string) (*http.Response, error) {
	tlsConfig := &tls.Config{}
	caCert, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		t.Fatalf("Got error reading RootCA file: %s", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig.RootCAs = caCertPool

	cert, err := tls.LoadX509KeyPair(
		clientCert,
		clientKey)
	if err != nil {
		t.Fatalf("Got error reading client certificates: %s", err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport}
	return httpClient.Get(url)
}

func checkExporter(t *testing.T, addr string, secure bool) error {
	var resp *http.Response
	var err error
	if secure {
		resp, err = getSecure(t, fmt.Sprintf("https://%s/metrics", addr))
	} else {
		resp, err = http.Get(fmt.Sprintf("http://%s/metrics", addr))
	}
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("expected a 200 response, got %d", resp.StatusCode)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("got an error reading the body: %v", err)
	}

	if !strings.Contains(string(body), "gnatsd_varz_connections") {
		return fmt.Errorf("response did not have NATS data")
	}
	return nil
}

func TestExporter(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetSubz = true
	opts.GetRoutez = true

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterHTTPS(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.CaFile = caCertFile
	opts.CertFile = serverCert
	opts.KeyFile = serverKey

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	// Check that we CANNOT connect with http
	if err := checkExporter(t, exp.http.Addr().String(), false); err == nil {
		t.Fatalf("Did not receive expected error.")
	}
	// Check that we CAN connect with https
	if err := checkExporter(t, exp.http.Addr().String(), true); err != nil {
		t.Fatalf("Received TLS error:  %v", err)
	}
}

func TestExporterHTTPSInvalidConfig(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()

	checkExporterStart := func() {
		exp := NewExporter(opts)
		if err := exp.Start(); err == nil {
			t.Fatalf("Did not receive expected error.")
			exp.Stop()
		}
	}

	// Test invalid certificate authority
	opts.CaFile = "garbage"
	opts.CertFile = serverCert
	opts.KeyFile = serverKey
	checkExporterStart()

	// test invalid server certificate
	opts.CaFile = caCertFile
	opts.CertFile = "garbage"
	opts.KeyFile = clientKey
	checkExporterStart()

	// test invalid server key
	opts.CaFile = caCertFile
	opts.CertFile = serverCert
	opts.KeyFile = "invalid"
	checkExporterStart()

	// test invalid server/key pair
	opts.CaFile = caCertFile
	opts.CertFile = serverCert
	opts.KeyFile = clientKey
	checkExporterStart()
}

func TestExporterDefaultOptions(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(nil)

	// test without a server configured
	if err := exp.Start(); err == nil {
		t.Fatalf("Did not recieve expected error.")
	}

	opts := GetDefaultExporterOptions()
	opts.GetVarz = true
	opts.NATSServerURL = fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	exp = NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterWait(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}

	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
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
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.RetryInterval = 1 * time.Second
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()

	if err := checkExporter(t, exp.http.Addr().String(), false); err == nil {
		t.Fatalf("Expected an error, received none.")
	}

	// allow for a few retries.
	time.Sleep(opts.RetryInterval * 2)

	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	time.Sleep(opts.RetryInterval + (time.Millisecond * 500))

	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterAPIIdempotency(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true

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

func TestExporterAddServerAfterStart(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true

	exp := NewExporter(opts)

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()
	if err := exp.AddServer("test-server2", fmt.Sprintf("http://localhost:%d", pet.MonitorPort)); err == nil {
		t.Fatalf("Did not get expected error.")
	}
}

func TestPortReuse(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()

	// attempt to start another exporter on the same port
	exp2 := NewExporter(opts)
	if err := exp2.Start(); err == nil {
		t.Fatalf("Did not recieve expected error.")
		exp2.Stop()
	}
}

func TestExporterBounce(t *testing.T) {
	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true

	exp := NewExporter(opts)

	// test start
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
		t.Fatalf("%v", err)
	}
	// test stop
	exp.Stop()
	if err := checkExporter(t, exp.http.Addr().String(), false); err == nil {
		t.Fatalf("Did not received expected error")
	}

	time.Sleep(500 * time.Millisecond)

	// restart
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()
	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
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
	// do not configure a server
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
	if err := checkExporter(t, exp.http.Addr().String(), false); err != nil {
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
		defer exp.Stop()
	}
}
