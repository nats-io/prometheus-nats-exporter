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

	// Stream stats
	streamMessages      *prometheus.Desc
	streamBytes         *prometheus.Desc
	streamFirstSeq      *prometheus.Desc
	streamLastSeq       *prometheus.Desc
	streamConsumerCount *prometheus.Desc
	streamSubjectCount  *prometheus.Desc
	streamLimitBytes    *prometheus.Desc
	streamLimitMessages *prometheus.Desc

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
	streamLabels = append(streamLabels, "account_id")
	streamLabels = append(streamLabels, "stream_name")
	streamLabels = append(streamLabels, "stream_leader")
	streamLabels = append(streamLabels, "is_stream_leader")
	streamLabels = append(streamLabels, "stream_raft_group")

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
		// jetstream_server_total_streams
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
		// jetstream_stream_total_messages
		streamMessages: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "total_messages"),
			"Total number of messages from a stream",
			streamLabels,
			nil,
		),
		// jetstream_stream_limit_messages
		streamLimitMessages: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "limit_messages"),
			"The maximum number of messages allowed in a JetStream stream as per its configuration. A value of -1 indicates no limit.",
			streamLabels,
			nil,
		),
		// jetstream_stream_total_bytes
		streamBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "total_bytes"),
			"Total stored bytes from a stream",
			streamLabels,
			nil,
		),
		// jetstream_stream_limit_bytes
		streamLimitBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "limit_bytes"),
			"The maximum configured storage limit (in bytes) for a JetStream stream. A value of -1 indicates no limit.",
			streamLabels,
			nil,
		),
		// jetstream_stream_first_seq
		streamFirstSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "first_seq"),
			"First sequence from a stream",
			streamLabels,
			nil,
		),
		// jetstream_stream_last_seq
		streamLastSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "last_seq"),
			"Last sequence from a stream",
			streamLabels,
			nil,
		),
		// jetstream_stream_consumer_count
		streamConsumerCount: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "consumer_count"),
			"Total number of consumers from a stream",
			streamLabels,
			nil,
		),
		// jetstream_stream_subjects
		streamSubjectCount: prometheus.NewDesc(
			prometheus.BuildFQName(system, "stream", "subject_count"),
			"Total number of subjects in a stream",
			streamLabels,
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
	ch <- nc.streamMessages
	ch <- nc.streamBytes
	ch <- nc.streamFirstSeq
	ch <- nc.streamLastSeq
	ch <- nc.streamConsumerCount
	ch <- nc.streamSubjectCount
	ch <- nc.streamLimitBytes
	ch <- nc.streamLimitMessages

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
			suffix = "/jsz?consumers=true&config=true&raft=true"
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
		var streamName, streamLeader, streamRaftGroup string
		var consumerName, consumerDesc, consumerLeader string
		var isMetaLeader, isStreamLeader, isConsumerLeader string
		var accountName string
		var accountID string

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

		var isJetStreamDisabled float64
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

		for _, account := range resp.AccountDetails {
			accountName = account.Name
			accountID = account.Id
			for _, stream := range account.Streams {
				streamName = stream.Name

				if stream.Cluster != nil {
					streamLeader = stream.Cluster.Leader
					if streamLeader == serverName {
						isStreamLeader = "true"
					} else {
						isStreamLeader = "false"
					}
				} else {
					isStreamLeader = "true"
				}
				streamRaftGroup = stream.RaftGroup

				streamMetric := func(key *prometheus.Desc, value float64) prometheus.Metric {
					return prometheus.MustNewConstMetric(key, prometheus.GaugeValue, value,
						// Server Labels
						serverID, serverName, clusterName, jsDomain, clusterLeader, isMetaLeader,
						// Stream Labels
						accountName, accountID, streamName, streamLeader, isStreamLeader, streamRaftGroup)
				}
				ch <- streamMetric(nc.streamMessages, float64(stream.State.Msgs))
				ch <- streamMetric(nc.streamBytes, float64(stream.State.Bytes))
				ch <- streamMetric(nc.streamFirstSeq, float64(stream.State.FirstSeq))
				ch <- streamMetric(nc.streamLastSeq, float64(stream.State.LastSeq))
				ch <- streamMetric(nc.streamConsumerCount, float64(stream.State.Consumers))
				ch <- streamMetric(nc.streamSubjectCount, float64(stream.State.NumSubjects))

				if stream.Config != nil {
					ch <- streamMetric(nc.streamLimitBytes, float64(stream.Config.MaxBytes))
					ch <- streamMetric(nc.streamLimitMessages, float64(stream.Config.MaxMsgs))
				}

				// Now with the consumers.
				for _, consumer := range stream.Consumer {
					consumerName = consumer.Name
					if consumer.Config != nil {
						consumerDesc = consumer.Config.Description
					}
					if consumer.Cluster != nil {
						consumerLeader = consumer.Cluster.Leader
						if consumerLeader == serverName {
							isConsumerLeader = "true"
						} else {
							isConsumerLeader = "false"
						}
					} else {
						isConsumerLeader = "true"
					}
					consumerMetric := func(key *prometheus.Desc, value float64) prometheus.Metric {
						return prometheus.MustNewConstMetric(key, prometheus.GaugeValue, value,
							// Server Labels
							serverID, serverName, clusterName, jsDomain, clusterLeader, isMetaLeader,
							// Stream Labels
							accountName, accountID, streamName, streamLeader, isStreamLeader, streamRaftGroup,
							// Consumer Labels
							consumerName, consumerLeader, isConsumerLeader, consumerDesc,
						)
					}
					ch <- consumerMetric(nc.consumerDeliveredConsumerSeq, float64(consumer.Delivered.Consumer))
					ch <- consumerMetric(nc.consumerDeliveredStreamSeq, float64(consumer.Delivered.Stream))
					ch <- consumerMetric(nc.consumerNumAckPending, float64(consumer.NumAckPending))
					ch <- consumerMetric(nc.consumerNumRedelivered, float64(consumer.NumRedelivered))
					ch <- consumerMetric(nc.consumerNumWaiting, float64(consumer.NumWaiting))
					ch <- consumerMetric(nc.consumerNumPending, float64(consumer.NumPending))
					ch <- consumerMetric(nc.consumerAckFloorStreamSeq, float64(consumer.AckFloor.Stream))
					ch <- consumerMetric(nc.consumerAckFloorConsumerSeq, float64(consumer.AckFloor.Consumer))
				}
			}
		}
	}
}
