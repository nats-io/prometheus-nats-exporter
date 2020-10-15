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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats-replicator/server/conf"
	"github.com/nats-io/nuid"
	stan "github.com/nats-io/stan.go"
)

// Connector is the abstraction for all of the bridge connector types
type Connector interface {
	Start() error
	Shutdown() error

	CheckConnections() error

	String() string
	ID() string

	Stats() ConnectorStats
}

// CreateConnector builds a connector from the supplied configuration
func CreateConnector(config conf.ConnectorConfig, bridge *NATSReplicator) (Connector, error) {
	switch strings.ToLower(config.Type) {
	case strings.ToLower(conf.NATSToNATS):
		return NewNATS2NATSConnector(bridge, config), nil
	case strings.ToLower(conf.StanToNATS):
		return NewStan2NATSConnector(bridge, config), nil
	case strings.ToLower(conf.NATSToStan):
		return NewNATS2StanConnector(bridge, config), nil
	case strings.ToLower(conf.StanToStan):
		return NewStan2StanConnector(bridge, config), nil
	default:
		return nil, fmt.Errorf("unknown connector type %q in configuration", config.Type)
	}
}

// ReplicatorConnector is the base type used for connectors so that they can share code
// The config, bridge and stats are all fixed at creation, so no lock is required on the
// connector at this level. The stats do keep a lock to protect their data.
// The connector has a lock for use by composing types to protect themselves during start/shutdown.
type ReplicatorConnector struct {
	sync.Mutex

	config conf.ConnectorConfig
	bridge *NATSReplicator
	stats  *ConnectorStatsHolder
}

// Start is a no-op, designed for overriding
func (conn *ReplicatorConnector) Start() error {
	return nil
}

// Shutdown is a no-op, designed for overriding
func (conn *ReplicatorConnector) Shutdown() error {
	return nil
}

// CheckConnections is a no-op, designed for overriding
// This is called when nats or stan goes down
// the connector should return an error if it has to be shut down
func (conn *ReplicatorConnector) CheckConnections() error {
	return nil
}

// String returns the name passed into init
func (conn *ReplicatorConnector) String() string {
	return conn.stats.Name()
}

// ID returns the id from the stats
func (conn *ReplicatorConnector) ID() string {
	return conn.stats.ID()
}

// Stats returns a copy of the current stats for this connector
func (conn *ReplicatorConnector) Stats() ConnectorStats {
	return conn.stats.Stats()
}

// Init sets up common fields for all connectors
func (conn *ReplicatorConnector) init(bridge *NATSReplicator, config conf.ConnectorConfig, name string) {
	conn.config = config
	conn.bridge = bridge

	id := conn.config.ID
	if id == "" {
		id = nuid.Next()
	}
	conn.stats = NewConnectorStatsHolder(name, id)
}

func createSubscriberOptions(config conf.ConnectorConfig) []stan.SubscriptionOption {

	options := []stan.SubscriptionOption{}

	if config.IncomingDurableName != "" {
		options = append(options, stan.DurableName(config.IncomingDurableName))
	}

	if config.IncomingStartAtTime != 0 {
		t := time.Unix(config.IncomingStartAtTime, 0)
		options = append(options, stan.StartAtTime(t))
	} else if config.IncomingStartAtSequence == -1 {
		options = append(options, stan.StartWithLastReceived())
	} else if config.IncomingStartAtSequence > 0 {
		options = append(options, stan.StartAtSequence(uint64(config.IncomingStartAtSequence)))
	} else {
		options = append(options, stan.DeliverAllAvailable())
	}

	if config.IncomingMaxInflight != 0 {
		options = append(options, stan.MaxInflight(int(config.IncomingMaxInflight)))
	}

	if config.IncomingAckWait != 0 {
		options = append(options, stan.AckWait(time.Duration(config.IncomingAckWait)*time.Millisecond))
	}

	options = append(options, stan.SetManualAckMode())

	return options
}
