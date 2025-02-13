// Copyright 2017-2023 The NATS Authors
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
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/logger"
	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
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

// RunServerWithName runs the NATS server in a go routine
func RunServerWithName(name string) *server.Server {
	return RunServerWithPortsAndName(ClientPort, MonitorPort, name)
}

// RunGatewayzStaticServer starts an http server with static content
func RunGatewayzStaticServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: ":" + strconv.Itoa(StaticPort)}
	http.Handle("/gatewayz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, GatewayzTestResponse())
	}))

	go func() {
		defer wg.Done()
		srv.ListenAndServe()
	}()
	return srv
}

// RunAccstatzStaticServer starts an http server with static content
func RunAccstatzStaticServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: ":" + strconv.Itoa(StaticPort)}
	http.Handle("/accstatz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, AccstatzTestResponse())
	}))

	go func() {
		defer wg.Done()
		srv.ListenAndServe()
	}()
	return srv
}

// RunLeafzStaticServer runs a leafz static server.
func RunLeafzStaticServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: ":" + strconv.Itoa(StaticPort)}
	http.Handle("/leafz", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, leafzTestResponse())
	}))

	go func() {
		defer wg.Done()
		srv.ListenAndServe()
	}()
	return srv
}

// RunServerWithPortsAndName runs the NATS server with a monitor port and a name in a go routine
func RunServerWithPortsAndName(cport, mport int, serverName string) *server.Server {
	var enableLogging bool

	resetPreviousHTTPConnections()

	// To enable debug/trace output in the NATS server,
	// flip the enableLogging flag.
	// enableLogging = true

	opts := &server.Options{
		ServerName: serverName,
		Host:       "127.0.0.1",
		Port:       cport,
		HTTPHost:   "127.0.0.1",
		HTTPPort:   mport,
		NoLog:      !enableLogging,
		NoSigs:     true,
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

// RunServerWithPorts runs the NATS server with a monitor port in a go routine
func RunServerWithPorts(cport, mport int) *server.Server {
	return RunServerWithPortsAndName(cport, mport, "")
}

var tempRoot = filepath.Join(os.TempDir(), "exporter")

// RunJetStreamServerWithPorts starts a JetStream server.
func RunJetStreamServerWithPorts(port, monitorPort int, domain string) *server.Server {
	opts := natsserver.DefaultTestOptions
	opts.Port = port
	opts.JetStream = true
	opts.JetStreamDomain = domain
	tdir, _ := os.MkdirTemp(tempRoot, "js-storedir-")
	opts.StoreDir = filepath.Dir(tdir)
	opts.HTTPHost = "127.0.0.1"
	opts.HTTPPort = monitorPort

	return natsserver.RunServer(&opts)
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
	if _, err = nc.Subscribe("foo", func(_ *nats.Msg) { ch <- true }); err != nil {
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
