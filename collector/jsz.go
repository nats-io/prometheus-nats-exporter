// Copyright 2021 The NATS Authors
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

// Package collector has various collector utilities and implementations.
package collector

import (
	"net/http"
	"strings"
	"sync"
	"time"

	nats "github.com/nats-io/nats-server/v2/server"
	"github.com/prometheus/client_golang/prometheus"
)

type jszCollector struct {
	sync.Mutex
	httpClient *http.Client
	servers    []*CollectedServer
	endpoint   string

	// JetStream server stats
	disabled   *prometheus.Desc
	streams    *prometheus.Desc
	consumers  *prometheus.Desc
	messages   *prometheus.Desc
	bytes      *prometheus.Desc
	maxMemory  *prometheus.Desc
	maxStorage *prometheus.Desc

	// Consumer stats
	consumerDeliveredConsumerSeq *prometheus.Desc
	consumerDeliveredStreamSeq   *prometheus.Desc
	consumerNumAckPending        *prometheus.Desc
	consumerNumRedelivered       *prometheus.Desc
	consumerNumWaiting           *prometheus.Desc
	consumerNumPending           *prometheus.Desc
	consumerAckFloorStreamSeq    *prometheus.Desc
	consumerAckFloorConsumerSeq  *prometheus.Desc
}

func isJszEndpoint(system string) bool {
	return system == JetStreamSystem
}

func newJszCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	serverLabels := []string{"server_id", "server_name", "cluster", "domain", "meta_leader", "is_meta_leader"}

	var streamLabels []string
	streamLabels = append(streamLabels, serverLabels...)
	streamLabels = append(streamLabels, "account")
	// Amnon: We remove the stream_name tag from our metrics, because we have a large number of streams,
	// which creates a high cardinality - overwhelming Victoria Metrics.

	//streamLabels = append(streamLabels, "stream_name")
	//streamLabels = append(streamLabels, "stream_leader")
	//streamLabels = append(streamLabels, "is_stream_leader")

	var consumerLabels []string
	consumerLabels = append(consumerLabels, streamLabels...)
	consumerLabels = append(consumerLabels, "consumer_name")
	consumerLabels = append(consumerLabels, "consumer_leader")
	consumerLabels = append(consumerLabels, "is_consumer_leader")
	consumerLabels = append(consumerLabels, "consumer_desc")

	nc := &jszCollector{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		endpoint: endpoint,
		// jetstream_disabled
		disabled: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "jetstream_disabled"),
			"JetStream disabled or not",
			serverLabels,
			nil,
		),
		// jetstream_stream_total_messages
		streams: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "total_streams"),
			"Total number of streams in JetStream",
			serverLabels,
			nil,
		),
		// jetstream_server_total_consumers
		consumers: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "total_consumers"),
			"Total number of consumers in JetStream",
			serverLabels,
			nil,
		),
		// jetstream_server_total_messages
		messages: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "total_messages"),
			"Total number of stored messages in JetStream",
			serverLabels,
			nil,
		),
		// jetstream_server_total_message_bytes
		bytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "total_message_bytes"),
			"Total number of bytes stored in JetStream",
			serverLabels,
			nil,
		),
		// jetstream_server_max_memory
		maxMemory: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "max_memory"),
			"JetStream Max Memory",
			serverLabels,
			nil,
		),
		// jetstream_server_max_storage
		maxStorage: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "max_storage"),
			"JetStream Max Storage",
			serverLabels,
			nil,
		),
		// jetstream_consumer_delivered_consumer_seq
		consumerDeliveredConsumerSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "delivered_consumer_seq"),
			"Latest sequence number of a stream consumer",
			consumerLabels,
			nil,
		),
		// jetstream_consumer_delivered_stream_seq
		consumerDeliveredStreamSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "delivered_stream_seq"),
			"Latest sequence number of a stream",
			consumerLabels,
			nil,
		),
		// jetstream_consumer_num_ack_pending
		consumerNumAckPending: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "num_ack_pending"),
			"Number of pending acks from a consumer",
			consumerLabels,
			nil,
		),
		// jetstream_consumer_num_redelivered
		consumerNumRedelivered: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "num_redelivered"),
			"Number of redelivered messages from a consumer",
			consumerLabels,
			nil,
		),
		// jetstream_consumer_num_waiting
		consumerNumWaiting: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "num_waiting"),
			"Number of inflight fetch requests from a pull consumer",
			consumerLabels,
			nil,
		),
		// jetstream_consumer_num_pending
		consumerNumPending: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "num_pending"),
			"Number of pending messages from a consumer",
			consumerLabels,
			nil,
		),
		consumerAckFloorStreamSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "ack_floor_stream_seq"),
			"Number of ack floor stream seq from a consumer",
			consumerLabels,
			nil,
		),
		consumerAckFloorConsumerSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "consumer", "ack_floor_consumer_seq"),
			"Number of ack floor consumer seq from a consumer",
			consumerLabels,
			nil,
		),
	}

	// Use the endpoint
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL,
		}
	}

	return nc
}

// Describe shares the info description from a prometheus metric.
func (nc *jszCollector) Describe(ch chan<- *prometheus.Desc) {
	// Server state
	ch <- nc.disabled
	ch <- nc.streams
	ch <- nc.consumers
	ch <- nc.messages
	ch <- nc.bytes
	ch <- nc.maxMemory
	ch <- nc.maxStorage

	// Stream state
	//ch <- nc.streamMessages
	//ch <- nc.streamBytes
	//ch <- nc.streamFirstSeq
	//ch <- nc.streamLastSeq
	//ch <- nc.streamConsumerCount

	// Consumer state
	ch <- nc.consumerDeliveredConsumerSeq
	ch <- nc.consumerDeliveredStreamSeq
	ch <- nc.consumerNumAckPending
	ch <- nc.consumerNumRedelivered
	ch <- nc.consumerNumWaiting
	ch <- nc.consumerNumPending
}

// Collect gathers the server jsz metrics.
func (nc *jszCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp nats.JSInfo
		var suffix string

		switch strings.ToLower(nc.endpoint) {
		case "account", "accounts":
			suffix = "/jsz?accounts=true"
		case "consumer", "consumers", "all":
			suffix = "/jsz?consumers=true&config=true"
		case "stream", "streams":
			suffix = "/jsz?streams=true"
		default:
			suffix = "/jsz"
		}
		if err := getMetricURL(nc.httpClient, server.URL+suffix, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		var varz nats.Varz
		if err := getMetricURL(nc.httpClient, server.URL+"/varz", &varz); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		var serverID, serverName, clusterName, jsDomain, clusterLeader string
		var isMetaLeader string

		serverID = server.ID
		serverName = varz.Name
		if resp.Meta != nil {
			clusterName = resp.Meta.Name
			clusterLeader = resp.Meta.Leader
			if resp.Meta.Leader == serverName {
				isMetaLeader = "true"
			} else {
				isMetaLeader = "false"
			}
		} else {
			isMetaLeader = "true"
		}
		jsDomain = resp.Config.Domain

		serverMetric := func(key *prometheus.Desc, value float64) prometheus.Metric {
			return prometheus.MustNewConstMetric(key, prometheus.GaugeValue, value,
				serverID, serverName, clusterName, jsDomain, clusterLeader, isMetaLeader)
		}

		var isJetStreamDisabled float64 = 0
		if resp.Disabled {
			isJetStreamDisabled = 1
		}
		ch <- serverMetric(nc.disabled, isJetStreamDisabled)
		ch <- serverMetric(nc.maxMemory, float64(resp.Config.MaxMemory))
		ch <- serverMetric(nc.maxStorage, float64(resp.Config.MaxStore))
		ch <- serverMetric(nc.streams, float64(resp.Streams))
		ch <- serverMetric(nc.consumers, float64(resp.Consumers))
		ch <- serverMetric(nc.messages, float64(resp.Messages))
		ch <- serverMetric(nc.bytes, float64(resp.Bytes))
	}
}
