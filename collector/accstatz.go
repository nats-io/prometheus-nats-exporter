// Copyright 2022-2023 The NATS Authors
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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

func isAccstatzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "accstatz"
}

type accstatzCollector struct {
	sync.Mutex

	httpClient     *http.Client
	servers        []*CollectedServer
	accountMetrics *accountMetrics
}

func newAccstatzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &accstatzCollector{
		httpClient:     http.DefaultClient,
		accountMetrics: newAccountMetrics(system, endpoint),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/accstatz?unused=1",
		}
	}

	return nc
}

func (nc *accstatzCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.accountMetrics.Describe(ch)
}

// Collect gathers the server accstatz metrics.
func (nc *accstatzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Accstatz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		for _, acc := range resp.Accounts {
			nc.accountMetrics.Collect(server, acc, ch)
		}
	}
}

// accountMetrics has all of the prometheus descriptors related to
// each of the accounts of the server being scraped.
type accountMetrics struct {
	connections      *prometheus.Desc
	totalConnections *prometheus.Desc
	numSubs          *prometheus.Desc
	leafNodes        *prometheus.Desc
	sentMsgs         *prometheus.Desc
	sentBytes        *prometheus.Desc
	receivedMsgs     *prometheus.Desc
	receivedBytes    *prometheus.Desc
	slowConsumers    *prometheus.Desc
}

// newAccountMetrics initializes a new instance of accountMetrics.
func newAccountMetrics(system, endpoint string) *accountMetrics {

	// Base labels that are common to all metrics
	baseLabels := []string{"server_id", "account", "account_id", "account_name"}

	account := &accountMetrics{
		connections: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "current_connections"),
			"current_connections",
			baseLabels,
			nil),
		totalConnections: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "total_connections"),
			"total_connections",
			baseLabels,
			nil),
		numSubs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "subscriptions"),
			"subscriptions",
			baseLabels,
			nil),
		leafNodes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "leaf_nodes"),
			"leaf_nodes",
			baseLabels,
			nil),
		sentMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "sent_messages"),
			"sent_messages",
			baseLabels,
			nil),
		sentBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "sent_bytes"),
			"sent_bytes",
			baseLabels,
			nil),
		receivedMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "received_messages"),
			"received_messages",
			baseLabels,
			nil),
		receivedBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "received_bytes"),
			"received_bytes",
			baseLabels,
			nil),
		slowConsumers: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "slow_consumers"),
			"slow_consumers",
			baseLabels,
			nil),
	}

	return account
}

// Describe
func (am *accountMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- am.connections
	ch <- am.totalConnections
	ch <- am.numSubs
	ch <- am.leafNodes
	ch <- am.sentMsgs
	ch <- am.sentBytes
	ch <- am.receivedMsgs
	ch <- am.receivedBytes
	ch <- am.slowConsumers
}

// Collect collects all the metrics about an account.
func (am *accountMetrics) Collect(server *CollectedServer, acc *Account, ch chan<- prometheus.Metric) {

	// Base labels that are common to all metrics
	baseLabels := []string{server.ID, acc.Account, acc.Account, acc.Name}

	ch <- prometheus.MustNewConstMetric(am.connections, prometheus.GaugeValue, float64(acc.Conns),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.totalConnections, prometheus.GaugeValue, float64(acc.TotalConns),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.numSubs, prometheus.GaugeValue, float64(acc.NumSubs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.leafNodes, prometheus.GaugeValue, float64(acc.LeafNodes),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.sentMsgs, prometheus.GaugeValue, float64(acc.Sent.Msgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.sentBytes, prometheus.GaugeValue, float64(acc.Sent.Bytes),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.receivedMsgs, prometheus.GaugeValue, float64(acc.Received.Msgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.receivedBytes, prometheus.GaugeValue, float64(acc.Received.Bytes),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.slowConsumers, prometheus.GaugeValue, float64(acc.SlowConsumers),
		baseLabels...)
}

// Accstatz output
type Accstatz struct {
	Accounts []*Account `json:"account_statz"`
}

// Account stats output
type Account struct {
	Account       string    `json:"acc"`
	Name          string    `json:"name"`
	Conns         int       `json:"conns"`
	LeafNodes     int       `json:"leafnodes"`
	TotalConns    int       `json:"total_conns"`
	NumSubs       uint32    `json:"num_subscriptions"`
	Sent          DataStats `json:"sent"`
	Received      DataStats `json:"received"`
	SlowConsumers int64     `json:"slow_consumers"`
}

// DataStats output
type DataStats struct {
	Msgs  int64 `json:"msgs"`
	Bytes int64 `json:"bytes"`
}
