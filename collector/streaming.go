// Copyright 2019 The NATS Authors
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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	channelszSuffix = "/streaming/channelsz?subs=1"
	serverzSuffix   = "/streaming/serverz"
)

// newStreamingCollector collects channelsz and serversz metrics of
// streaming servers.
func newStreamingCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	switch endpoint {
	case "channelsz":
		return newChannelsCollector(system, servers)
	case "serverz":
		return newServerzCollector(system, servers)
	}
	return nil
}

func isStreamingEndpoint(system, endpoint string) bool {
	return system == StreamingSystem && (endpoint == "channelsz" || endpoint == "serverz")
}

type serverzCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer
	system     string

	bytesTotal *prometheus.Desc
	bytesIn    *prometheus.Desc
	bytesOut   *prometheus.Desc
	msgsTotal  *prometheus.Desc
	msgsIn     *prometheus.Desc
	msgsOut    *prometheus.Desc
	channels   *prometheus.Desc
	subs       *prometheus.Desc
	clients    *prometheus.Desc
	active     *prometheus.Desc
	info       *prometheus.Desc
}

func newServerzCollector(system string, servers []*CollectedServer) prometheus.Collector {
	nc := &serverzCollector{
		httpClient: http.DefaultClient,
		system:     system,
		bytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "bytes_total"),
			"Total of bytes",
			[]string{"server_id"},
			nil,
		),
		bytesIn: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "bytes_in"),
			"Incoming bytes",
			[]string{"server_id"},
			nil,
		),
		bytesOut: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "bytes_out"),
			"Outgoing bytes",
			[]string{"server_id"},
			nil,
		),
		msgsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "msgs_total"),
			"Total of messages",
			[]string{"server_id"},
			nil,
		),
		msgsIn: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "msgs_in"),
			"Incoming messages",
			[]string{"server_id"},
			nil,
		),
		msgsOut: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "msgs_out"),
			"Outgoing messages",
			[]string{"server_id"},
			nil,
		),
		channels: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "channels"),
			"Total channels",
			[]string{"server_id"},
			nil,
		),
		subs: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "subscriptions"),
			"Total subscriptions",
			[]string{"server_id"},
			nil,
		),
		clients: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "clients"),
			"Total clients",
			[]string{"server_id"},
			nil,
		),
		active: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "active"),
			"Active server",
			[]string{"server_id"},
			nil,
		),
		info: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "info"),
			"Info",
			[]string{"server_id", "cluster_id", "version", "go_version", "state", "role", "start_time"},
			nil,
		),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + serverzSuffix,
		}
	}

	return nc
}

func (nc *serverzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.bytesTotal
	ch <- nc.bytesIn
	ch <- nc.bytesOut
	ch <- nc.msgsTotal
	ch <- nc.msgsIn
	ch <- nc.msgsOut
	ch <- nc.channels
	ch <- nc.subs
	ch <- nc.clients
	ch <- nc.active
	ch <- nc.info
}

// StreamingServerz represents the metrics from streaming/serverz.
type StreamingServerz struct {
	TotalBytes    int    `json:"total_bytes"`
	InBytes       int    `json:"in_bytes"`
	OutBytes      int    `json:"out_bytes"`
	TotalMsgs     int    `json:"total_msgs"`
	InMsgs        int    `json:"in_msgs"`
	OutMsgs       int    `json:"out_msgs"`
	Channels      int    `json:"channels"`
	Subscriptions int    `json:"subscriptions"`
	Clients       int    `json:"clients"`
	ClusterID     string `json:"cluster_id"`
	ServerID      string `json:"server_id"`
	Version       string `json:"version"`
	GoVersion     string `json:"go"`
	State         string `json:"state"`
	Role          string `json:"role"`
	StartTime     string `json:"start_time"`
}

// Collect gathers the streaming server serverz metrics.
func (nc *serverzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp StreamingServerz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(nc.bytesTotal, prometheus.CounterValue,
			float64(resp.TotalBytes), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.bytesIn, prometheus.CounterValue,
			float64(resp.InBytes), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.bytesOut, prometheus.CounterValue,
			float64(resp.OutBytes), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.msgsTotal, prometheus.CounterValue,
			float64(resp.TotalMsgs), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.msgsIn, prometheus.CounterValue,
			float64(resp.InMsgs), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.msgsOut, prometheus.CounterValue,
			float64(resp.OutMsgs), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.channels, prometheus.CounterValue,
			float64(resp.Channels), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.subs, prometheus.CounterValue,
			float64(resp.Subscriptions), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.clients, prometheus.CounterValue,
			float64(resp.Clients), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.active, prometheus.GaugeValue,
			boolToFloat(resp.State == "FT_ACTIVE"), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.info, prometheus.GaugeValue,
			1, server.ID, resp.ClusterID, resp.Version, resp.GoVersion, resp.State, resp.Role, resp.StartTime)
	}
}

type channelsCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer
	system     string

	chanBytesTotal   *prometheus.Desc
	chanMsgsTotal    *prometheus.Desc
	chanLastSeq      *prometheus.Desc
	subsLastSent     *prometheus.Desc
	subsPendingCount *prometheus.Desc
	subsMaxInFlight  *prometheus.Desc
}

func newChannelsCollector(system string, servers []*CollectedServer) prometheus.Collector {
	subsVariableLabels := []string{
		"server_id", "server_role", "channel", "client_id", "inbox", "queue_name",
		"is_durable", "is_offline", "durable_name",
	}
	nc := &channelsCollector{
		httpClient: http.DefaultClient,
		system:     system,
		chanBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "bytes_total"),
			"Total of bytes",
			[]string{"server_id", "server_role", "channel"},
			nil,
		),
		chanMsgsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "msgs_total"),
			"Total of messages",
			[]string{"server_id", "server_role", "channel"},
			nil,
		),
		chanLastSeq: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "last_seq"),
			"Last seq",
			[]string{"server_id", "server_role", "channel"},
			nil,
		),
		subsLastSent: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "subs_last_sent"),
			"Last message sent",
			subsVariableLabels,
			nil,
		),
		subsPendingCount: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "subs_pending_count"),
			"Pending message count",
			subsVariableLabels,
			nil,
		),
		subsMaxInFlight: prometheus.NewDesc(
			prometheus.BuildFQName(system, "chan", "subs_max_inflight"),
			"Max in flight message count",
			subsVariableLabels,
			nil,
		),
	}

	// create our own deep copy, and tweak the urls to be polled
	// for this type of endpoint
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + channelszSuffix,
		}
	}

	return nc
}

func (nc *channelsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.chanBytesTotal
	ch <- nc.chanMsgsTotal
	ch <- nc.chanLastSeq
	ch <- nc.subsLastSent
	ch <- nc.subsPendingCount
	ch <- nc.subsMaxInFlight
}

func getRoleFromChannelszURL(client *http.Client, url string) (string, error) {
	if !strings.HasSuffix(url, channelszSuffix) {
		return "", nil
	}

	var newURL = (strings.TrimSuffix(url, channelszSuffix) + serverzSuffix)
	var serverResp StreamingServerz
	if err := getMetricURL(client, newURL, &serverResp); err != nil {
		return "", err
	}
	return serverResp.Role, nil
}

func (nc *channelsCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Channelsz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		serverRole, err := getRoleFromChannelszURL(nc.httpClient, server.URL)
		if err != nil {
			Debugf("error getting server role %s: %v", server.ID, err)
		}

		for _, channel := range resp.Channels {
			ch <- prometheus.MustNewConstMetric(nc.chanBytesTotal, prometheus.GaugeValue,
				float64(channel.Bytes), server.ID, serverRole, channel.Name)
			ch <- prometheus.MustNewConstMetric(nc.chanMsgsTotal, prometheus.GaugeValue,
				float64(channel.Msgs), server.ID, serverRole, channel.Name)
			ch <- prometheus.MustNewConstMetric(nc.chanLastSeq, prometheus.GaugeValue,
				float64(channel.LastSeq), server.ID, serverRole, channel.Name)

			for _, sub := range channel.Subscriptions {

				// If this is a durable queue group subscription then split the
				// durable name from the queue name
				durableName := sub.DurableName
				queueName := sub.QueueName
				if sub.IsDurable && queueName != "" {
					subStrings := strings.Split(queueName, ":")
					durableName, queueName = subStrings[0], subStrings[1]
				}
				labelValues := []string{server.ID, serverRole, channel.Name, sub.ClientID, sub.Inbox,
					queueName, strconv.FormatBool(sub.IsDurable), strconv.FormatBool(sub.IsOffline), durableName}

				ch <- prometheus.MustNewConstMetric(nc.subsLastSent, prometheus.GaugeValue,
					float64(sub.LastSent), labelValues...)
				ch <- prometheus.MustNewConstMetric(nc.subsPendingCount, prometheus.GaugeValue,
					float64(sub.PendingCount), labelValues...)
				ch <- prometheus.MustNewConstMetric(nc.subsMaxInFlight, prometheus.GaugeValue,
					float64(sub.MaxInflight), labelValues...)
			}
		}
	}
}

// Channelsz lists the name of all NATS Streaming Channelsz
type Channelsz struct {
	ClusterID string      `json:"cluster_id"`
	Now       time.Time   `json:"now"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
	Count     int         `json:"count"`
	Total     int         `json:"total"`
	Names     []string    `json:"names,omitempty"`
	Channels  []*Channelz `json:"channels,omitempty"`
}

// Channelz describes a NATS Streaming Channel
type Channelz struct {
	Name          string           `json:"name"`
	Msgs          int              `json:"msgs"`
	Bytes         uint64           `json:"bytes"`
	FirstSeq      uint64           `json:"first_seq"`
	LastSeq       uint64           `json:"last_seq"`
	Subscriptions []*Subscriptionz `json:"subscriptions,omitempty"`
}

// Subscriptionz describes a NATS Streaming Subscription
type Subscriptionz struct {
	ClientID     string `json:"client_id"`
	Inbox        string `json:"inbox"`
	AckInbox     string `json:"ack_inbox"`
	DurableName  string `json:"durable_name,omitempty"`
	QueueName    string `json:"queue_name,omitempty"`
	IsDurable    bool   `json:"is_durable"`
	IsOffline    bool   `json:"is_offline"`
	MaxInflight  int    `json:"max_inflight"`
	AckWait      int    `json:"ack_wait"`
	LastSent     uint64 `json:"last_sent"`
	PendingCount int    `json:"pending_count"`
	IsStalled    bool   `json:"is_stalled"`
}
