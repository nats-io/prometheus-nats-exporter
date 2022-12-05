// Copyright 2017-2019 The NATS Authors
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

    httpClient      *http.Client
    servers         []*CollectedServer
    accountMetrics  *accountMetrics
}

func newAccstatzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
    nc := &accstatzCollector{
        httpClient:         http.DefaultClient,
        accountMetrics:     newAccountMetrics(system, endpoint),
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
    connections            *prometheus.Desc
    totalConnections       *prometheus.Desc
    leafNodes              *prometheus.Desc
    sentMsgs               *prometheus.Desc
    sentBytes              *prometheus.Desc
    receivedMsgs           *prometheus.Desc
    receivedBytes          *prometheus.Desc
    slowConsumers          *prometheus.Desc
}

// newAccountMetrics initializes a new instance of accountMetrics.
func newAccountMetrics(system, endpoint string) *accountMetrics {
    account := &accountMetrics{
        connections: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "current_connections"),
            "current_connections",
            []string{"server_id", "account"},
            nil),
        totalConnections: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "total_connections"),
            "total_connections",
            []string{"server_id", "account"},
            nil),
        leafNodes: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "leaf_nodes"),
            "leaf_nodes",
            []string{"server_id", "account"},
            nil),
        sentMsgs: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "sent_messages"),
            "sent_messages",
            []string{"server_id", "account"},
            nil),
        sentBytes: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "sent_bytes"),
            "sent_bytes",
            []string{"server_id", "account"},
            nil),
        receivedMsgs: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "received_messages"),
            "received_messages",
            []string{"server_id", "account"},
            nil),
        receivedBytes: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "received_bytes"),
            "received_bytes",
            []string{"server_id", "account"},
            nil),
        slowConsumers: prometheus.NewDesc(
            prometheus.BuildFQName(system, endpoint, "slow_consumers"),
            "slow_consumers",
            []string{"server_id", "account"},
            nil),
    }

    return account
}

// Describe
func (am *accountMetrics) Describe(ch chan<- *prometheus.Desc) {
    ch <- am.connections
    ch <- am.totalConnections
    ch <- am.leafNodes
    ch <- am.sentMsgs
    ch <- am.sentBytes
    ch <- am.receivedMsgs
    ch <- am.receivedBytes
    ch <- am.slowConsumers
}

// Collect collects all the metrics about an account.
func (am *accountMetrics) Collect(server *CollectedServer, acc *Account, ch chan<- prometheus.Metric) {

    ch <- prometheus.MustNewConstMetric(am.connections, prometheus.GaugeValue, float64(acc.Connections),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.totalConnections, prometheus.GaugeValue, float64(acc.TotalConnections),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.leafNodes, prometheus.GaugeValue, float64(acc.LeafNodes),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.sentMsgs, prometheus.GaugeValue, float64(acc.Sent.Messages),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.sentBytes, prometheus.GaugeValue, float64(acc.Sent.Bytes),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.receivedMsgs, prometheus.GaugeValue, float64(acc.Received.Messages),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.receivedBytes, prometheus.GaugeValue, float64(acc.Received.Bytes),
        server.ID, acc.AccountId)

    ch <- prometheus.MustNewConstMetric(am.slowConsumers, prometheus.GaugeValue, float64(acc.SlowConsumers),
        server.ID, acc.AccountId)
}

// Accstatz output
type Accstatz struct {
    Accounts     []*Account `json:"account_statz"`
}

// Leaf output
type Account struct {
    AccountId           string  `json:"acc"`
    Connections         int     `json:"conns"`
    LeafNodes           int     `json:"leafnodes"`
    TotalConnections    int     `json:"total_conns"`
    Sent                Data    `json:"sent"`
    Received            Data    `json:"received"`
    SlowConsumers       int     `json:"slow_consumers"`
}

// Data output
type Data struct {
    Messages    int     `json:"msgs"`
    Bytes       int     `json:"bytes"`
}
