// Copyright 2016 Apcera Inc. All rights reserved.

package test

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/nats-io/gnatsd/logger"
	"github.com/nats-io/gnatsd/server"
	"github.com/nats-io/go-nats"
)

// ClientPort is the default port for clients to connect
const ClientPort = 11224

// MonitorPort is the default monitor port
const MonitorPort = 11424

// RunServer runs the NATS server in a go routine
func RunServer() *server.Server {
	return RunServerWithPorts(ClientPort, MonitorPort)
}

// RunServerWithPorts runs the NATS server with a monitor port in a go routine
func RunServerWithPorts(cport, mport int) *server.Server {
	var enableLogging bool

	resetPreviousHTTPConnections()

	// To enable debug/trace output in the NATS server,
	// flip the enableLogging flag.
	// enableLogging = true

	opts := &server.Options{
		Host:     "localhost",
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
		_ = conn.Close() // nolint

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
