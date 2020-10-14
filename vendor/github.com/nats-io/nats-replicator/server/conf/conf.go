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

package conf

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	"github.com/nats-io/nats-replicator/server/logging"
)

const (
	// NATSToNATS specifies a connector from NATS to NATS
	NATSToNATS = "NATSToNATS"
	// NATSToStan specifies a connector from NATS to NATS streaming
	NATSToStan = "NATSToStan"
	// StanToNATS specifies a connector from NATS streaming to NATS
	StanToNATS = "StanToNATS"
	// StanToStan specifies a connector from NATS streaming to NATS Streaming
	StanToStan = "StanToStan"
)

// NATSReplicatorConfig is the root structure for a bridge configuration file.
// NATS and STAN connections are specified in a map, where the key is a name used by
// the connector to reference a connection.
type NATSReplicatorConfig struct {
	ReconnectInterval int `conf:"reconnect_interval"` // milliseconds

	Logging    logging.Config
	NATS       []NATSConfig
	STAN       []NATSStreamingConfig
	Monitoring HTTPConfig
	Connect    []ConnectorConfig
}

// TLSConf holds the configuration for a TLS connection/server
type TLSConf struct {
	Key  string
	Cert string
	Root string
}

// MakeTLSConfig creates a tls.Config from a TLSConf, setting up the key pairs and certs
func (tlsConf *TLSConf) MakeTLSConfig() (*tls.Config, error) {
	if tlsConf.Cert == "" || tlsConf.Key == "" {
		return nil, nil
	}

	cert, err := tls.LoadX509KeyPair(tlsConf.Cert, tlsConf.Key)
	if err != nil {
		return nil, fmt.Errorf("error loading X509 certificate/key pair: %v", err)
	}

	cert.Leaf, err = x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate: %v", err)
	}

	config := tls.Config{
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		ClientAuth:               tls.NoClientCert,
		PreferServerCipherSuites: true,
	}

	if tlsConf.Root != "" {
		// Load CA cert
		caCert, err := ioutil.ReadFile(tlsConf.Root)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		config.RootCAs = caCertPool
	}

	return &config, nil
}

// HTTPConfig is used to specify the host/port/tls for an HTTP server
type HTTPConfig struct {
	HTTPHost  string `conf:"http_host"`
	HTTPPort  int    `conf:"http_port"`
	HTTPSPort int    `conf:"https_port"`
	TLS       TLSConf

	ReadTimeout  int `conf:"read_timeout"`  //milliseconds
	WriteTimeout int `conf:"write_timeout"` //milliseconds
}

// NATSConfig configuration for a NATS connection
type NATSConfig struct {
	Name    string
	Servers []string

	ConnectTimeout int  `conf:"connect_timeout"` //milliseconds
	ReconnectWait  int  `conf:"reconnect_wait"`  //milliseconds
	MaxReconnects  int  `conf:"max_reconnects"`
	NoRandom       bool `conf:"no_random"`
	NoEcho         bool `conf:"no_echo"`

	TLS             TLSConf
	UserCredentials string `conf:"user_credentials"`
}

// NATSStreamingConfig configuration for a STAN connection
type NATSStreamingConfig struct {
	Name      string
	ClusterID string `conf:"cluster_id"`
	ClientID  string `conf:"client_id"`

	PubAckWait         int    `conf:"pub_ack_wait"` //milliseconds
	DiscoverPrefix     string `conf:"discovery_prefix"`
	MaxPubAcksInflight int    `conf:"max_pubacks_inflight"`
	ConnectWait        int    `conf:"connect_wait"` // milliseconds

	PingInterval int `conf:"ping_interval"` // seconds
	MaxPings     int `conf:"max_pings"`

	NATSConnection string `conf:"nats_connection"` //name of the nats connection for this streaming connection
}

// DefaultConfig generates a default configuration with
// logging set to colors, time, debug and trace
func DefaultConfig() NATSReplicatorConfig {
	return NATSReplicatorConfig{
		ReconnectInterval: 5000,
		Logging: logging.Config{
			Colors: true,
			Time:   true,
			Debug:  false,
			Trace:  false,
		},
		Monitoring: HTTPConfig{
			ReadTimeout:  5000,
			WriteTimeout: 5000,
		},
	}
}

// ConnectorConfig configuration for a single connection (of any type)
// Properties are available for any type, but only the ones necessary for the
// connector type are used
type ConnectorConfig struct {
	ID   string // user specified id for a connector, will be defaulted if none is provided
	Type string // Can be any of the type constants (NATSToStan, ...)

	IncomingConnection string `conf:"incoming_connection"` // Name of the incoming connection (of either type), can be the same as outgoingConnection
	OutgoingConnection string `conf:"outgoing_connection"` // Name of the outgoing connection (of either type), can be the same as incomingConnection

	IncomingChannel         string `conf:"incoming_channel"`          // Used for stan connections
	IncomingDurableName     string `conf:"incoming_durable_name"`     // Optional, used for stan connections
	IncomingStartAtSequence int64  `conf:"incoming_startat_sequence"` // Start position for stan connection, -1 means StartWithLastReceived, 0 means DeliverAllAvailable (default)
	IncomingStartAtTime     int64  `conf:"incoming_startat_time"`     // Start time, as Unix, time takes precedence over sequence
	IncomingMaxInflight     int64  `conf:"incoming_max_in_flight"`    // maximum message in flight to this connector's subscription in Streaming
	IncomingAckWait         int64  `conf:"incoming_ack_wait"`         // max wait time in Milliseconds for the incoming subscription

	IncomingSubject   string `conf:"incoming_subject"`    // Used for nats connections
	IncomingQueueName string `conf:"incoming_queue_name"` // Optional, used for nats connections

	OutgoingChannel string `conf:"outgoing_channel"` // Used for stan connections
	OutgoingSubject string `conf:"outgoing_subject"` // Used for nats connections
}
