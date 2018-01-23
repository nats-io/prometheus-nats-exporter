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

func buildExporterURL(user, pass, addr string, secure bool) string {

	proto := "http"
	if secure {
		proto = "https"
	}

	if user != "" {
		return fmt.Sprintf("%s://%s:%s@%s/metrics", proto, user, pass, addr)
	}

	return fmt.Sprintf("%s://%s/metrics", proto, addr)
}

func checkExporterFull(t *testing.T, user, pass, addr string, secure bool, expectedRc int) error {
	var resp *http.Response
	var err error
	url := buildExporterURL(user, pass, addr, secure)

	if secure {
		resp, err = getSecure(t, url)
	} else {
		resp, err = http.Get(url)
	}
	if err != nil {
		return err
	}
	// Avoid EOF errors in Travis
	resp.Close = true

	rc := resp.StatusCode
	if rc != expectedRc {
		return fmt.Errorf("expected a %d response, got %d", expectedRc, rc)
	}

	// bail on auth error, etc.
	if rc != 200 {
		return nil
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

func checkExporter(t *testing.T, addr string, secure bool) error {
	return checkExporterFull(t, "", "", addr, secure, http.StatusOK)
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

func testBasicAuth(t *testing.T, opts *NATSExporterOptions, testuser, testpass string, expectedRc int) error {
	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		return err
	}
	defer exp.Stop()

	return checkExporterFull(t, testuser, testpass, exp.http.Addr().String(), false, expectedRc)
}

func TestExporterBasicAuth(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetSubz = true
	opts.GetRoutez = true
	opts.HTTPUser = "colin"
	opts.HTTPPassword = "password"

	s := pet.RunServer()
	defer s.Shutdown()

	// first try user/pass with no auth.
	testBasicAuth(t, opts, "colin", "password", http.StatusOK)

	// now try user/pass
	opts.HTTPUser = "colin"
	opts.HTTPPassword = "password"
	if err := testBasicAuth(t, opts, "colin", "password", http.StatusOK); err != nil {
		t.Fatalf("%v", err)
	}

	// now failures...
	if err := testBasicAuth(t, opts, "colin", "garbage", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(t, opts, "garbage", "password", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(t, opts, "", "password", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(t, opts, "colin", "", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}

	opts.HTTPPassword = "$2a$11$SITqoMbjeKK1y9iV6nhXa.fC8o/QXEqs7o7DMkFtVErp0aTn12o1y"
	if err := testBasicAuth(t, opts, "colin", "password", http.StatusOK); err != nil {
		t.Fatalf("%v", err)
	}
	opts.HTTPPassword = "$2a$11$SITqoMbjeKK1y9iV6nhXa.fC8o/QXEqs7o7DMkFtVErp0aTn12o1y"
	if err := testBasicAuth(t, opts, "colin", "garbage", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
}
