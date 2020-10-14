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
 *
 */

package core

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/nats-io/nats-replicator/server/conf"
	"github.com/nats-io/nats-replicator/server/logging"
	nats "github.com/nats-io/nats.go"
	stan "github.com/nats-io/stan.go"
)

const version = "0.1.0"

// NATSReplicator is the core structure for the server.
type NATSReplicator struct {
	sync.Mutex
	running bool

	startTime time.Time

	logger logging.Logger
	config conf.NATSReplicatorConfig

	natsLock sync.RWMutex
	nats     map[string]*nats.Conn
	stan     map[string]stan.Conn

	connectorLock   sync.RWMutex
	connectors      []Connector
	needReconnect   map[string]Connector
	reconnectTicker *time.Ticker
	cancelReconnect chan bool

	statsLock     sync.Mutex
	httpReqStats  map[string]int64
	listener      net.Listener
	http          *http.Server
	httpHandler   *http.ServeMux
	monitoringURL string
}

// NewNATSReplicator creates a new account server with a default logger
func NewNATSReplicator() *NATSReplicator {
	return &NATSReplicator{
		logger: logging.NewNATSLogger(logging.Config{
			Colors: true,
			Time:   true,
			Debug:  true,
			Trace:  true,
		}),
		nats: map[string]*nats.Conn{},
		stan: map[string]stan.Conn{},
	}
}

// Logger hosts a shared logger
func (server *NATSReplicator) Logger() logging.Logger {
	return server.logger
}

func (server *NATSReplicator) checkRunning() bool {
	server.Lock()
	defer server.Unlock()
	return server.running
}

// InitializeFromFlags is called from main to configure the server, the server
// will decide what needs to happen based on the flags. On reload the same flags are
// passed
func (server *NATSReplicator) InitializeFromFlags(flags Flags) error {
	server.config = conf.DefaultConfig()

	// Always try to apply a config file, we can't run without one
	err := server.ApplyConfigFile(flags.ConfigFile)

	if err != nil {
		return err
	}

	if flags.Debug || flags.DebugAndVerbose {
		server.config.Logging.Debug = true
	}

	if flags.Verbose || flags.DebugAndVerbose {
		server.config.Logging.Trace = true
	}

	return nil
}

// ApplyConfigFile applies the config file to the server's config
func (server *NATSReplicator) ApplyConfigFile(configFile string) error {
	if configFile == "" {
		configFile = os.Getenv("NATS_REPLICATOR_CONFIG")
		if configFile != "" {
			server.logger.Noticef("using config specified in $NATS_REPLICATOR_CONFIG %q", configFile)
		}
	} else {
		server.logger.Noticef("loading configuration from %q", configFile)
	}

	if configFile == "" {
		return fmt.Errorf("no config file specified")
	}

	if err := conf.LoadConfigFromFile(configFile, &server.config, false); err != nil {
		return err
	}

	return nil
}

// InitializeFromConfig initialize the server's configuration to an existing config object, useful for tests
// Does not change the config at all, use DefaultServerConfig() to create a default config
func (server *NATSReplicator) InitializeFromConfig(config conf.NATSReplicatorConfig) error {
	server.config = config
	return nil
}

// Start the server, will lock the server, assumes the config is loaded
func (server *NATSReplicator) Start() error {
	server.Lock()
	defer server.Unlock()

	if server.logger != nil {
		server.logger.Close()
	}

	server.running = true
	server.startTime = time.Now()
	server.logger = logging.NewNATSLogger(server.config.Logging)
	server.connectors = []Connector{}
	server.needReconnect = map[string]Connector{}
	server.cancelReconnect = make(chan bool, 1)

	server.logger.Noticef("starting NATS-Replicator, version %s", version)
	server.logger.Noticef("server time is %s", server.startTime.Format(time.UnixDate))

	if err := server.connectToNATS(); err != nil {
		return err
	}

	if err := server.connectToSTAN(); err != nil {
		return err
	}

	if err := server.initializeConnectors(); err != nil {
		return err
	}

	if err := server.startConnectors(); err != nil {
		return err
	}

	if err := server.startMonitoring(); err != nil {
		return err
	}

	server.startReconnectTicker()

	return nil
}

// Stop the replicator
func (server *NATSReplicator) Stop() {
	server.Lock()
	if !server.running {
		server.Unlock()
		return // already stopped
	}
	server.logger.Noticef("stopping replicator")
	server.running = false
	server.Unlock()

	// cancel outside the lock
	server.logger.Noticef("cancelling reconnect timer")
	server.cancelReconnect <- true

	server.logger.Noticef("closing connectors")
	server.connectorLock.Lock()
	for _, c := range server.connectors {
		err := c.Shutdown()

		if err != nil {
			server.logger.Noticef("error shutting down connector %s", err.Error())
		}
	}
	server.connectorLock.Unlock()

	server.logger.Noticef("closing stan connections")
	server.natsLock.Lock()
	for name, sc := range server.stan {
		sc.Close()
		server.logger.Noticef("disconnected from NATS streaming connection named %s", name)
	}

	server.logger.Noticef("closing nats connections")
	for name, nc := range server.nats {
		nc.Close()
		server.logger.Noticef("disconnected from NATS connection named %s", name)
	}
	server.natsLock.Unlock()

	server.logger.Noticef("closing http server used for monitoring")
	server.Lock()
	err := server.StopMonitoring()
	if err != nil {
		server.logger.Noticef("error shutting down monitoring server %s", err.Error())
	}
	server.Unlock()
}

// assumes the server lock is held by the caller
func (server *NATSReplicator) initializeConnectors() error {
	connectorConfigs := server.config.Connect

	for _, c := range connectorConfigs {
		connector, err := CreateConnector(c, server)

		if err != nil {
			return err
		}

		server.connectors = append(server.connectors, connector)
	}
	return nil
}

// assumes the server lock is held by the caller
func (server *NATSReplicator) startConnectors() error {
	for _, c := range server.connectors {
		if err := c.Start(); err != nil {
			server.logger.Noticef("error starting %s, %s", c.String(), err.Error())
			return err
		}
	}
	return nil
}

// ConnectorError is called by a connector if it has a failure that requires a reconnect
func (server *NATSReplicator) ConnectorError(connector Connector, err error) {
	if !server.checkRunning() {
		return
	}

	server.connectorLock.Lock()
	defer server.connectorLock.Unlock()

	_, check := server.needReconnect[connector.ID()]

	if check {
		return // we already have that connector, no need to stop or pring any messages
	}

	server.needReconnect[connector.ID()] = connector

	description := connector.String()
	server.logger.Errorf("a connector error has occurred, replicator will try to restart %s, %s", description, err.Error())

	err = connector.Shutdown()

	if err != nil {
		server.logger.Warnf("error shutting down connector %s, replicator will try to restart, %s", description, err.Error())
	}
}

// checkConnections loops over the connections and has them each check check their requirements
func (server *NATSReplicator) checkConnections() {
	server.logger.Warnf("checking connector requirements and will restart as needed.")

	if !server.checkRunning() {
		return
	}

	server.connectorLock.Lock()
	defer server.connectorLock.Unlock()

	for _, connector := range server.connectors {
		_, check := server.needReconnect[connector.ID()]

		if check {
			continue // we already have that connector, no need to stop or pring any messages
		}

		err := connector.CheckConnections()

		if err == nil {
			continue // connector is happy
		}

		server.needReconnect[connector.ID()] = connector

		description := connector.String()
		server.logger.Errorf("a connector error has occurred, trying to restart %s, %s", description, err.Error())

		err = connector.Shutdown()

		if err != nil {
			server.logger.Warnf("error shutting down connector %s, trying to restart, %s", description, err.Error())
		}
	}
}

// requires the reconnect lock be held by the caller
// spawns a go routine that will acquire the lock for handling reconnect tasks
func (server *NATSReplicator) startReconnectTicker() {
	interval := server.config.ReconnectInterval
	server.reconnectTicker = time.NewTicker(time.Duration(interval) * time.Millisecond)

	go func() {

		// Loop until we get cancelled
	Loop:
		for {
			select {
			case <-server.reconnectTicker.C:
				// Don't get reconnect lock until we try connectors
				// Nats lock will protect the nats and stan map

				// Wait for nats to be reconnected
				for _, c := range server.config.NATS {
					if !server.CheckNATS(c.Name) {
						server.logger.Noticef("nats connection %s is down, will try retry in %d milliseconds", c.Name, interval)
						continue Loop
					}
				}

				// Make sure stan is up, if it should be
				//server.logger.Noticef("trying to reconnect to nats streaming")
				err := server.connectToSTAN() // this may be a no-op if all the connections are there but is not true once we get the lock in the connect

				if err != nil {
					server.logger.Noticef("error restarting streaming connection, will retry in %d milliseconds", interval, err.Error())
					continue Loop
				}

				server.connectorLock.Lock()
				// Do all the reconnects, we will redo the ones we have to
				for id, connector := range server.needReconnect {

					// keep checking if we should exit
					if !server.checkRunning() {
						continue Loop // go back to the loop so we can read the cancel request
					}

					server.logger.Noticef("trying to restart connector %s", connector.String())
					err := connector.Start()

					if err != nil {
						server.logger.Noticef("error restarting connector %s, will retry in %d milliseconds, %s", connector.String(), interval, err.Error())
					} else {
						delete(server.needReconnect, id)
					}
				}
				server.connectorLock.Unlock()
			case <-server.cancelReconnect:
				server.logger.Noticef("reconnect ticker cancelled")
				return
			}
		}

	}()
}
