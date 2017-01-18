// Copyright 2016 Apcera Inc. All rights reserved.

package collector

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/go-nats"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

func runMonitorServerNoHTTPPort() *server.Server {
	resetPreviousHTTPConnections()
	opts := DefaultMonitorOptions
	opts.HTTPPort = 0
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

func TestVarz(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MonitorPort)

	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()

	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port
	var urls []string
	urls = append(urls, url+"varz")

	cases := map[string]float64{
		"gnatsd_varz_total_connections": 2,
		"gnatsd_varz_connections":       1,
		"gnatsd_varz_in_msgs":           1,
		"gnatsd_varz_out_msgs":          1,
		"gnatsd_varz_in_bytes":          5,
		"gnatsd_varz_out_bytes":         5,
		"gnatsd_varz_subscriptions":     1,
	}

	verifyCollector(urls, cases, t)

}

// return fqName from parsing the Desc() field of a metric.
func parseDesc(desc string) string {
	// split on quotes.
	return strings.Split(desc, "\"")[1]
}

func TestConnz(t *testing.T) {
	s := runMonitorServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", MonitorPort)
	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port
	var urls []string
	urls = append(urls, url+"connz")

	cases := map[string]float64{
		"gnatsd_connz_total_connections": 0,
		"gnatsd_varz_connections":        0,
	}

	verifyCollector(urls, cases, t)

	// Test with connections.
	nc := createClientConnSubscribeAndPublish(t)
	defer nc.Close()
}

func verifyCollector(urls []string, cases map[string]float64, t *testing.T) {
	// create a new collector.
	coll := NewCollector(urls)
	// t.Fatalf("got body %s", coll)

	// now collect the metrics
	c := make(chan prometheus.Metric)
	go coll.Collect(c)

	for {
		select {
		case metric := <-c:
			pb := &dto.Metric{}
			metric.Write(pb)
			gauge := pb.GetGauge()
			val := gauge.GetValue()

			name := parseDesc(metric.Desc().String())
			fmt.Println("checking ", name)

			expected, ok := cases[name]
			if ok {
				if val != expected {
					t.Fatalf("Expected %s=%v, got %v", name, expected, val)
				}
			}
		case <-time.After(10 * time.Millisecond):
			return // pjm: there must be a smarter, safer, faster way to do this.
		}
	}
}

// Create a connection to test ConnInfo
func createClientConnSubscribeAndPublish(t *testing.T) *nats.Conn {
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", ClientPort))
	if err != nil {
		t.Fatalf("Error creating client: %v\n", err)
	}

	ch := make(chan bool)
	nc.Subscribe("foo", func(m *nats.Msg) { ch <- true })
	nc.Publish("foo", []byte("Hello"))
	// Wait for message
	<-ch
	return nc
}

func createClientConnWithName(t *testing.T, name string) *nats.Conn {
	natsURI := fmt.Sprintf("nats://localhost:%d", ClientPort)

	client := nats.DefaultOptions
	client.Servers = []string{natsURI}
	client.Name = name
	nc, err := client.Connect()
	if err != nil {
		t.Fatalf("Error creating client: %v\n", err)
	}

	return nc
}
