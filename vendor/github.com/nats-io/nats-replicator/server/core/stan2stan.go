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
	stan "github.com/nats-io/stan.go"
)

// Stan2StanConnector connects a streaming channel to another streaming channel
type Stan2StanConnector struct {
	ReplicatorConnector
	sub stan.Subscription
}

// NewStan2StanConnector create a nats to MQ connector
func NewStan2StanConnector(bridge *NATSReplicator, config conf.ConnectorConfig) Connector {
	connector := &Stan2StanConnector{}
	connector.init(bridge, config, fmt.Sprintf("Stan:%s to Stan:%s", config.IncomingChannel, config.OutgoingChannel))
	return connector
}

// Start the connector
func (conn *Stan2StanConnector) Start() error {
	conn.Lock()
	defer conn.Unlock()

	config := conn.config
	incoming := config.IncomingConnection
	outgoing := config.OutgoingConnection

	if incoming == "" || outgoing == "" || config.IncomingChannel == "" || config.OutgoingChannel == "" {
		return fmt.Errorf("%s connector is improperly configured, incoming and outgoing settings are required", conn.String())
	}

	if !conn.bridge.CheckStan(incoming) {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), incoming)
	}

	if !conn.bridge.CheckStan(outgoing) {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), outgoing)
	}

	conn.bridge.Logger().Tracef("starting connection %s", conn.String())

	options := createSubscriberOptions(config)
	traceEnabled := conn.bridge.Logger().TraceEnabled()

	osc := conn.bridge.Stan(outgoing)
	if osc == nil {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), outgoing)
	}

	callback := func(msg *stan.Msg) {
		start := time.Now()

		if traceEnabled {
			conn.bridge.Logger().Tracef("%s received message", conn.String())
		}

		_, err := osc.PublishAsync(config.OutgoingChannel, msg.Data, func(ackguid string, err error) {
			l := int64(len(msg.Data))

			if err != nil {
				conn.stats.AddMessageIn(l)
				conn.bridge.ConnectorError(conn, err)
				return
			}

			if traceEnabled {
				conn.bridge.Logger().Tracef("%s wrote message to stan", conn.String())
			}

			if err := msg.Ack(); err != nil {
				conn.stats.AddMessageIn(l)
				conn.bridge.ConnectorError(conn, err)
				return
			}

			if traceEnabled {
				conn.bridge.Logger().Tracef("%s acked message", conn.String())
			}

			conn.stats.AddRequest(l, l, time.Since(start))
		})

		// TODO(dlc) - Should we attempt to make sure message is resent before ack timeout from incoming?
		if err != nil {
			conn.stats.AddMessageIn(int64(len(msg.Data)))
			conn.bridge.ConnectorError(conn, err)
			return
		}
	}

	sc := conn.bridge.Stan(incoming)

	if sc == nil {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), incoming)
	}

	sub, err := sc.Subscribe(conn.config.IncomingChannel, callback, options...)
	if err != nil {
		return err
	}

	conn.sub = sub

	conn.stats.AddConnect()

	if config.IncomingDurableName != "" {
		conn.bridge.Logger().Tracef("opened and reading %s with durable name %s", conn.config.IncomingChannel, config.IncomingDurableName)
	} else {
		conn.bridge.Logger().Tracef("opened and reading %s", conn.config.IncomingChannel)
	}
	conn.bridge.Logger().Noticef("started connection %s", conn.String())

	return nil
}

// Shutdown the connector
func (conn *Stan2StanConnector) Shutdown() error {
	conn.Lock()
	defer conn.Unlock()
	conn.stats.AddDisconnect()

	conn.bridge.Logger().Noticef("shutting down connection %s", conn.String())

	sub := conn.sub
	conn.sub = nil

	if sub != nil {
		if err := sub.Close(); err != nil {
			conn.bridge.Logger().Noticef("error closing for %s, %s", conn.String(), err.Error())
		}
	}

	return nil // ignore the disconnect error
}

// CheckConnections ensures the nats/stan connection and report an error if it is down
func (conn *Stan2StanConnector) CheckConnections() error {
	config := conn.config
	incoming := config.IncomingConnection
	outgoing := config.OutgoingConnection
	if !conn.bridge.CheckStan(incoming) {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), incoming)
	}

	if !conn.bridge.CheckStan(outgoing) {
		return fmt.Errorf("%s connector requires stan connection named %s to be available", conn.String(), outgoing)
	}
	return nil
}
