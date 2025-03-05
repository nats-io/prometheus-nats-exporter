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

package exporter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	pet "github.com/nats-io/prometheus-nats-exporter/test"
)

const (
	clientCert = "../test/certs/client.pem"
	clientKey  = "../test/certs/client.key"
	serverCert = "../test/certs/server.pem"
	serverKey  = "../test/certs/server.key"
	caCertFile = "../test/certs/ca.pem"
)

func getDefaultExporterTestOptions() (opts *NATSExporterOptions) {
	o := GetDefaultExporterOptions()
	o.NATSServerTag = "test-server"
	o.NATSServerURL = fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	return o
}

func getStaticExporterTestOptions() (opts *NATSExporterOptions) {
	o := GetDefaultExporterOptions()
	o.NATSServerTag = "test-server"
	o.NATSServerURL = fmt.Sprintf("http://localhost:%d", pet.StaticPort)
	return o
}

func httpGetSecure(url string) (*http.Response, error) {
	tlsConfig := &tls.Config{}
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("Got error reading RootCA file: %s", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	tlsConfig.RootCAs = caCertPool

	cert, err := tls.LoadX509KeyPair(
		clientCert,
		clientKey)
	if err != nil {
		return nil, fmt.Errorf("Got error reading client certificates: %s", err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport, Timeout: 30 * time.Second}
	return httpClient.Get(url)
}

func httpGet(url string) (*http.Response, error) {
	httpClient := &http.Client{Timeout: 30 * time.Second}
	return httpClient.Get(url)
}

func buildExporterURL(user, pass, addr string, path string, secure bool) string {
	proto := "http"
	if secure {
		proto = "https"
	}

	if user != "" {
		return fmt.Sprintf("%s://%s:%s@%s%s", proto, user, pass, addr, path)
	}

	return fmt.Sprintf("%s://%s%s", proto, addr, path)
}

func checkExporterFull(user, pass, addr, result, path string, secure bool, expectedRc int) (string, error) {
	var resp *http.Response
	var err error
	url := buildExporterURL(user, pass, addr, path, secure)

	if secure {
		resp, err = httpGetSecure(url)
	} else {
		resp, err = httpGet(url)
	}
	if err != nil {
		return "", fmt.Errorf("error from get: %v", err)
	}
	defer resp.Body.Close()

	rc := resp.StatusCode
	if rc != expectedRc {
		return "", fmt.Errorf("expected a %d response, got %d", expectedRc, rc)
	}
	if rc != 200 {
		return "", nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("got an error reading the body: %v", err)
	}
	results := string(body)
	if !strings.Contains(results, result) {
		return results, fmt.Errorf("response did not have NATS data")
	}
	return results, nil
}

func checkExporter(addr string, secure bool) error {
	_, err := checkExporterFull("", "", addr, "gnatsd_varz_connections", "/metrics", secure, http.StatusOK)
	return err
}

func checkExporterForResult(addr, result string) (string, error) {
	return checkExporterFull("", "", addr, result, "/metrics", false, http.StatusOK)
}

func TestExporter(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetHealthz = true
	opts.GetSubz = true
	opts.GetGatewayz = true
	opts.GetLeafz = true
	opts.GetRoutez = true

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterRestart(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetHealthz = true
	opts.GetSubz = true
	opts.GetGatewayz = true
	opts.GetLeafz = true
	opts.GetRoutez = true

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}

	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}
	exp.Stop()

	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()
	if err := checkExporter(exp.addr, false); err != nil {
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
	if err := checkExporter(exp.addr, false); err == nil {
		t.Fatalf("Did not receive expected error.")
	}
	// Check that we CAN connect with https
	if err := checkExporter(exp.addr, true); err != nil {
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
			exp.Stop()
			t.Fatalf("Did not receive expected error.")
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
		t.Fatalf("Did not receive expected error.")
	}

	opts := GetDefaultExporterOptions()
	opts.GetVarz = true

	// Travis CI errors on the default due to no ipv6 support, so
	// use locahost for the test.
	opts.ListenAddress = "localhost"

	opts.NATSServerURL = fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	exp = NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterScrapePathOption(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.ScrapePath = "/some/other/path/to/metrics"
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetHealthz = true
	opts.GetSubz = true
	opts.GetRoutez = true

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	_, err := checkExporterFull("", "", exp.addr,
		"gnatsd_varz_connections", "/some/other/path/to/metrics", false, http.StatusOK)
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterScrapePathOptionAddsSlash(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.ScrapePath = "elsewhere"
	opts.GetVarz = true

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	_, err := checkExporterFull("", "", exp.addr,
		"gnatsd_varz_connections", "/elsewhere", false, http.StatusOK)
	if err != nil {
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

	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}

	var mu sync.Mutex
	var didStop bool
	go func() {
		time.Sleep(time.Second * 1)
		mu.Lock()
		defer mu.Unlock()
		exp.Stop()
		didStop = true
	}()
	exp.WaitUntilDone()
	mu.Lock()
	defer mu.Unlock()
	if !didStop {
		t.Fatalf("did not wait until completed.")
	}
}

/* TODO(jaime): This test doesn't pass anymore. Not sure why. It might be
* related to prometheus.Register no longer returning an error if nothing gets
* sent on the Describe channel.
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

	if err := checkExporter(exp.addr, false); err == nil {
		t.Fatalf("Expected an error, received none.")
	}
	time.Sleep(2 * opts.RetryInterval)

	// start the server
	s := pet.RunServer()
	defer s.Shutdown()

	time.Sleep(opts.RetryInterval + (500 * time.Millisecond))

	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}
}
*/

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
		exp2.Stop()
		t.Fatalf("Did not receive expected error.")
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
	if err := checkExporter(exp.addr, false); err != nil {
		t.Fatalf("%v", err)
	}
	// test stop
	exp.Stop()
	if err := checkExporter(exp.addr, false); err == nil {
		t.Fatalf("Did not received expected error")
	}

	time.Sleep(500 * time.Millisecond)

	// restart
	if err := exp.Start(); err != nil {
		t.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	defer exp.Stop()
	if err := checkExporter(exp.addr, false); err != nil {
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
	if err := checkExporter(exp.addr, false); err != nil {
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
		exp.Stop()
		t.Fatalf("Did not receive expected error adding a server.")
	}
}

func testBasicAuth(opts *NATSExporterOptions, testuser, testpass string, expectedRc int) error {
	http.DefaultTransport.(*http.Transport).CloseIdleConnections()
	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		return err
	}
	defer exp.Stop()

	_, err := checkExporterFull(testuser, testpass, exp.addr,
		"gnatsd_varz_connections", "/metrics", false, expectedRc)
	return err
}

func TestExporterBasicAuth(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.GetConnz = true
	opts.GetHealthz = true
	opts.GetSubz = true
	opts.GetRoutez = true

	s := pet.RunServer()
	defer s.Shutdown()

	// first try user/pass with no auth.
	testBasicAuth(opts, "colin", "password", http.StatusOK)

	// now try user/pass
	opts.HTTPUser = "colin"
	opts.HTTPPassword = "password"
	if err := testBasicAuth(opts, "colin", "password", http.StatusOK); err != nil {
		t.Fatalf("%v", err)
	}

	// now failures...
	if err := testBasicAuth(opts, "colin", "garbage", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(opts, "garbage", "password", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(opts, "", "password", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(opts, "colin", "", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}

	// test bcrypt with a cost of 2 (use a low cost!).  Resolves to "password"
	opts.HTTPPassword = "$2a$10$H753p./UP9XNoEmbXDSWrOw7/XGIdVCM80SFAbBIQJeqICAJypJqa"
	if err := testBasicAuth(opts, "colin", "password", http.StatusOK); err != nil {
		t.Fatalf("%v", err)
	}
	if err := testBasicAuth(opts, "colin", "garbage", http.StatusUnauthorized); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterPrefix(t *testing.T) {
	opts := getDefaultExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetVarz = true
	opts.Prefix = "test"

	s := pet.RunServer()
	defer s.Shutdown()

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	if _, err := checkExporterForResult(exp.addr, "test_varz_connections"); err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterGatewayz(t *testing.T) {
	opts := getStaticExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetGatewayz = true

	serverExit := &sync.WaitGroup{}

	serverExit.Add(1)
	s := pet.RunGatewayzStaticServer(serverExit)
	defer s.Shutdown(context.TODO())

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	_, err := checkExporterForResult(exp.addr, "gnatsd_gatewayz_inbound_gateway_conn_in_msgs")
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterAccstatz(t *testing.T) {
	opts := getStaticExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetAccstatz = true

	serverExit := &sync.WaitGroup{}

	serverExit.Add(1)
	s := pet.RunAccstatzStaticServer(serverExit)
	defer s.Shutdown(context.TODO())

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	_, err := checkExporterForResult(exp.addr, "gnatsd_accstatz_current_connections")
	if err != nil {
		t.Fatalf("%v", err)
	}
}

func TestExporterLeafz(t *testing.T) {
	opts := getStaticExporterTestOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 0
	opts.GetLeafz = true

	serverExit := &sync.WaitGroup{}

	serverExit.Add(1)
	s := pet.RunLeafzStaticServer(serverExit)
	defer s.Shutdown(context.TODO())

	exp := NewExporter(opts)
	if err := exp.Start(); err != nil {
		t.Fatalf("%v", err)
	}
	defer exp.Stop()

	_, err := checkExporterForResult(exp.addr, "gnatsd_leafz_conn_in_msgs")
	if err != nil {
		t.Fatalf("%v", err)
	}
}
