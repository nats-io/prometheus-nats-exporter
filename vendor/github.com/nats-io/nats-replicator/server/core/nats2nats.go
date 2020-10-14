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
	"time"

	"github.com/nats-io/nats-replicator/server/conf"
	nats "github.com/nats-io/nats.go"
)

// NATS2NATSConnector connects a NATS subject to a different NATS subject
type NATS2NATSConnector struct {
	ReplicatorConnector
	subscription *nats.Subscription
}

// NewNATS2NATSConnector create a new NATS to NATS connector
func NewNATS2NATSConnector(bridge *NATSReplicator, config conf.ConnectorConfig) Connector {
	connector := &NATS2NATSConnector{}
	connector.init(bridge, config, fmt.Sprintf("NATS:%s to NATS:%s", config.IncomingSubject, config.OutgoingSubject))
	return connector
}

// Start the connector
func (conn *NATS2NATSConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	config := conn.config
	incoming := config.IncomingConnection
	outgoing := config.OutgoingConnection

	if incoming == "" || outgoing == "" || config.IncomingSubject == "" || config.OutgoingSubject == "" {
		return fmt.Errorf("%s connector is improperly configured, incoming and outgoing settings are required", conn.String())
	}

	if !conn.bridge.CheckNATS(incoming) {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), incoming)
	}

	if !conn.bridge.CheckNATS(outgoing) {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), outgoing)
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	onc := conn.bridge.NATS(outgoing)
	if onc == nil {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), outgoing)
	}

	traceEnabled := conn.bridge.Logger().TraceEnabled()
	callback := func(msg *nats.Msg) {
		start := time.Now()
		l := int64(len(msg.Data))

		err := onc.Publish(config.OutgoingSubject, msg.Data)

		if err != nil {
			conn.stats.AddMessageIn(l)
			conn.bridge.Logger().Noticef("connector publish failure, %s, %s", conn.String(), err.Error())
		} else {
			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to nats", conn.String())
			}
			conn.stats.AddRequest(l, l, time.Since(start))
		}
	}

	nc := conn.bridge.NATS(incoming)

	if nc == nil {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), incoming)
	}

	var err error

	if config.IncomingQueueName == "" {
		conn.subscription, err = nc.Subscribe(config.IncomingSubject, callback)
	} else {
		conn.subscription, err = nc.QueueSubscribe(config.IncomingSubject, config.IncomingQueueName, callback)
	}

	if err != nil {
		return err
	}

	conn.stats.AddConnect()
	conn.bridge.Logger().Tracef("opened and reading %s", conn.config.IncomingSubject)
	conn.bridge.Logger().Noticef("started connection %s", conn.String())

	return nil
}

// Shutdown the connector
func (conn *NATS2NATSConnector) Shutdown() error {
	conn.Lock()
	defer conn.Unlock()
	conn.stats.AddDisconnect()

	conn.bridge.Logger().Noticef("shutting down connection %s", conn.String())

	sub := conn.subscription
	conn.subscription = nil

	if sub != nil {
		if err := sub.Unsubscribe(); err != nil {
			conn.bridge.Logger().Noticef("error unsubscribing for %s, %s", conn.String(), err.Error())
		}
	}

	return nil // ignore the disconnect error
}

// CheckConnections ensures the nats/stan connection and report an error if it is down
func (conn *NATS2NATSConnector) CheckConnections() error {
	config := conn.config
	incoming := config.IncomingConnection
	outgoing := config.OutgoingConnection
	if !conn.bridge.CheckNATS(incoming) {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), incoming)
	}

	if !conn.bridge.CheckNATS(outgoing) {
		return fmt.Errorf("%s connector requires nats connection named %s to be available", conn.String(), outgoing)
	}
	return nil
}
