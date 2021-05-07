// Copyright 2017-2019 The NATS Authors
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

// Package test can be used to create NATS servers used in exporter and
// collector tests.
package test

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/logger"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"

	rconf "github.com/nats-io/nats-replicator/server/conf"
	rcore "github.com/nats-io/nats-replicator/server/core"
	nss "github.com/nats-io/nats-streaming-server/server"
)

// ClientPort is the default port for clients to connect
const ClientPort = 11224

// MonitorPort is the default monitor port
const MonitorPort = 11424

// StaticPort is where static nats metrics are served
const StaticPort = 11425

// RunServer runs the NATS server in a go routine
func RunServer() *server.Server {
	return RunServerWithPorts(ClientPort, MonitorPort)
}

// RunStreamingServer runs the STAN server in a go routine.
func RunStreamingServer() *nss.StanServer {
	return RunStreamingServerWithPorts(nss.DefaultClusterID, ClientPort, MonitorPort)
}

// RunGatewayzStaticServer starts an http server with static content
func RunGatewayzStaticServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: ":" + strconv.Itoa(StaticPort)}
	http.Handle("/gatewayz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, GatewayzTestResponse())
	}))

	go func() {
		defer wg.Done()
		srv.ListenAndServe()
	}()
	return srv
}

// RunStreamingServerWithPorts runs the STAN server in a go routine allowing
// the clusterID and ports to be specified..
func RunStreamingServerWithPorts(clusterID string, port, monitorPort int) *nss.StanServer {
	resetPreviousHTTPConnections()

	sopts := nss.GetDefaultOptions()
	sopts.ID = clusterID

	nopts := nss.DefaultNatsServerOptions
	nopts.NoLog = true
	nopts.NoSigs = true
	nopts.Host = "127.0.0.1"
	nopts.Port = port
	nopts.HTTPHost = "127.0.0.1"
	nopts.HTTPPort = monitorPort
	s, err := nss.RunServerWithOpts(sopts, &nopts)
	if err != nil {
		panic(err)
	}
	return s
}

// RunServerWithPorts runs the NATS server with a monitor port in a go routine
func RunServerWithPorts(cport, mport int) *server.Server {
	var enableLogging bool

	resetPreviousHTTPConnections()

	// To enable debug/trace output in the NATS server,
	// flip the enableLogging flag.
	// enableLogging = true

	opts := &server.Options{
		Host:     "127.0.0.1",
		Port:     cport,
		HTTPHost: "127.0.0.1",
		HTTPPort: mport,
		NoLog:    !enableLogging,
		NoSigs:   true,
	}

	s := server.New(opts)
	if s == nil {
		panic("No NATS Server object returned.")
	}

	if enableLogging {
		l := logger.NewStdLogger(true, true, true, false, true)
		s.SetLogger(l, true, true)
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
		_ = conn.Close()

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

// CreateClientConnSubscribeAndPublish creates a conn and publishes
func CreateClientConnSubscribeAndPublish(t *testing.T) *nats.Conn {
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", ClientPort))
	if err != nil {
		t.Fatalf("Error creating client: %v\n", err)
	}

	ch := make(chan bool)
	if _, err = nc.Subscribe("foo", func(m *nats.Msg) { ch <- true }); err != nil {
		t.Fatalf("unable to subscribe: %v", err)
	}
	if err := nc.Publish("foo", []byte("Hello")); err != nil {
		t.Fatalf("unable to publish: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush error: %v", err)
	}
	// Wait for message
	<-ch
	return nc
}

// RunTestReplicator starts an instance of the replicator
func RunTestReplicator(monitorPort, natsServerPort1, natsServerPort2 int) (*rcore.NATSReplicator, error) {
	config := rconf.DefaultConfig()
	config.Logging.Debug = false
	config.Logging.Trace = false
	config.Logging.Colors = false
	config.Monitoring = rconf.HTTPConfig{
		HTTPPort: monitorPort,
	}

	config.NATS = []rconf.NATSConfig{
		{
			Name:           "nats",
			Servers:        []string{fmt.Sprintf("127.0.0.1:%d", natsServerPort1)},
			ConnectTimeout: 2000,
			ReconnectWait:  2000,
			MaxReconnects:  5,
		},
		{
			Name:           "nats2",
			Servers:        []string{fmt.Sprintf("127.0.0.1:%d", natsServerPort2)},
			ConnectTimeout: 2000,
			ReconnectWait:  2000,
			MaxReconnects:  5,
		},
	}

	config.Connect = []rconf.ConnectorConfig{
		{
			Type:               "NatsToNats",
			IncomingConnection: "nats",
			OutgoingConnection: "nats2",
			IncomingSubject:    "bar",
			OutgoingSubject:    "bar.out",
		},
	}

	rep := rcore.NewNATSReplicator()
	err := rep.InitializeFromConfig(config)
	if err != nil {
		rep.Stop()
		return nil, err
	}
	err = rep.Start()
	if err != nil {
		rep.Stop()
		return nil, err
	}

	return rep, nil
}
