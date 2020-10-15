/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package core

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// HTTP endpoints
const (
	RootPath    = "/"
	VarzPath    = "/varz"
	HealthzPath = "/healthz"
)

// startMonitoring starts the HTTP or HTTPs server if needed.
// expects the lock to be held
func (server *NATSReplicator) startMonitoring() error {
	config := server.config.Monitoring

	if config.HTTPPort != 0 && config.HTTPSPort != 0 {
		return fmt.Errorf("can't specify both HTTP (%v) and HTTPs (%v) ports", config.HTTPPort, config.HTTPSPort)
	}

	if config.HTTPPort == 0 && config.HTTPSPort == 0 {
		server.logger.Noticef("monitoring is disabled")
		return nil
	}

	secure := false

	if config.HTTPSPort != 0 {
		if config.TLS.Cert == "" || config.TLS.Key == "" {
			return fmt.Errorf("TLS cert and key required for HTTPS")
		}
		secure = true
	}

	// Used to track HTTP requests
	server.httpReqStats = map[string]int64{
		RootPath:    0,
		VarzPath:    0,
		HealthzPath: 0,
	}

	var (
		hp       string
		err      error
		listener net.Listener
		port     int
		cer      tls.Certificate
	)

	monitorProtocol := "http"

	if secure {
		monitorProtocol += "s"
		port = config.HTTPSPort
		if port == -1 {
			port = 0
		}
		hp = net.JoinHostPort(config.HTTPHost, strconv.Itoa(port))

		cer, err = tls.LoadX509KeyPair(config.TLS.Cert, config.TLS.Key)
		if err != nil {
			return err
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		config.ClientAuth = tls.NoClientCert
		listener, err = tls.Listen("tcp", hp, config)
	} else {
		port = config.HTTPPort
		if port == -1 {
			port = 0
		}
		hp = net.JoinHostPort(config.HTTPHost, strconv.Itoa(port))
		listener, err = net.Listen("tcp", hp)
	}

	if err != nil {
		return fmt.Errorf("can't listen to the monitor port: %v", err)
	}

	server.logger.Noticef("starting %s monitor on %s", monitorProtocol,
		net.JoinHostPort(config.HTTPHost, strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)))

	mhp := net.JoinHostPort(config.HTTPHost, strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
	if config.HTTPHost == "" {
		mhp = "localhost" + mhp
	}
	server.monitoringURL = fmt.Sprintf("%s://%s/", monitorProtocol, mhp)

	mux := http.NewServeMux()

	mux.HandleFunc(RootPath, server.HandleRoot)
	mux.HandleFunc(VarzPath, server.HandleVarz)
	mux.HandleFunc(HealthzPath, server.HandleHealthz)

	// Do not set a WriteTimeout because it could cause cURL/browser
	// to return empty response or unable to display page if the
	// server needs more time to build the response.
	srv := &http.Server{
		Addr:           hp,
		Handler:        mux,
		MaxHeaderBytes: 1 << 20,
	}

	server.listener = listener
	server.httpHandler = mux
	server.http = srv

	go func() {
		srv.Serve(listener)
		srv.Handler = nil
	}()

	return nil
}

// StopMonitoring shuts down the http server used for monitoring
// expects the server lock to be held
func (server *NATSReplicator) StopMonitoring() error {
	server.logger.Tracef("stopping monitoring")
	if server.http != nil && server.httpHandler != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
		defer cancel()

		if err := server.http.Shutdown(ctx); err != nil {
			return err
		}

		server.http = nil
		server.httpHandler = nil
	}

	if server.listener != nil {
		server.listener.Close() // ignore the error
		server.listener = nil
	}
	server.logger.Noticef("http monitoring stopped")

	return nil
}

// HandleRoot will show basic info and links to others handlers.
func (server *NATSReplicator) HandleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	server.statsLock.Lock()
	server.httpReqStats[RootPath]++
	server.statsLock.Unlock()
	fmt.Fprintf(w, `<html lang="en">
   <head>
    <link rel="shortcut icon" href="http://nats.io/img/favicon.ico">
    <style type="text/css">
      body { font-family: "Century Gothic", CenturyGothic, AppleGothic, sans-serif; font-size: 22; }
      a { margin-left: 32px; }
    </style>
  </head>
  <body>
    <img src="http://nats.io/img/logo.png" alt="NATS">
    <br/>
		<a href=/varz>varz</a><br/>
		<a href=/healthz>healthz</a><br/>
    <br/>
  </body>
</html>`)
}

// HandleVarz returns statistics about the server.
func (server *NATSReplicator) HandleVarz(w http.ResponseWriter, r *http.Request) {
	server.statsLock.Lock()
	server.httpReqStats[VarzPath]++
	server.statsLock.Unlock()

	compact := strings.ToLower(r.URL.Query().Get("compact")) == "true"

	stats := server.stats()

	var err error
	var varzJSON []byte

	if compact {
		varzJSON, err = json.Marshal(stats)
	} else {
		varzJSON, err = json.MarshalIndent(stats, "", "  ")
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(varzJSON)
}

// HandleHealthz returns status 200.
func (server *NATSReplicator) HandleHealthz(w http.ResponseWriter, r *http.Request) {
	server.statsLock.Lock()
	server.httpReqStats[HealthzPath]++
	server.statsLock.Unlock()
	w.WriteHeader(http.StatusOK)
}

// stats calculates the stats for the server and connectors
// assumes that the running lock is held by the caller
func (server *NATSReplicator) stats() BridgeStats {
	now := time.Now()

	stats := BridgeStats{}
	stats.StartTime = server.startTime.Unix()
	stats.UpTime = now.Sub(server.startTime).String()
	stats.ServerTime = now.Unix()

	for _, connector := range server.connectors {
		cstats := connector.Stats()
		stats.Connections = append(stats.Connections, cstats)
		stats.RequestCount += cstats.RequestCount
	}

	stats.HTTPRequests = map[string]int64{}

	server.statsLock.Lock()
	for k, v := range server.httpReqStats {
		stats.HTTPRequests[k] = int64(v)
	}
	server.statsLock.Unlock()

	return stats
}

// SafeStats grabs the lock then calls stats(), useful for tests
func (server *NATSReplicator) SafeStats() BridgeStats {
	server.Lock()
	defer server.Unlock()
	return server.stats()
}

// GetMonitoringRootURL returns the protocol://host:port for the monitoring server, useful for testing
func (server *NATSReplicator) GetMonitoringRootURL() string {
	server.Lock()
	defer server.Unlock()
	return server.monitoringURL
}
