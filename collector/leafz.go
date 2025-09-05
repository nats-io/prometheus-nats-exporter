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
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// isLeafzEndpoint returns wether an endpoint is a leafz or not.
func isLeafzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "leafz"
}

// leafzCollector is responsible to gather metrics on leaf nodes.
type leafzCollector struct {
	sync.Mutex

	httpClient     *http.Client
	servers        []*CollectedServer
	leafNodesTotal *prometheus.Desc
	leafMetrics    *leafMetrics
}

// newLeafzCollector creates a new instance of a leafzCollector.
func newLeafzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &leafzCollector{httpClient: http.DefaultClient}
	nc.leafNodesTotal = prometheus.NewDesc(
		prometheus.BuildFQName(system, endpoint, "conn_nodes_total"),
		"nodes_total",
		[]string{"server_id"},
		nil)
	nc.leafMetrics = newLeafMetrics(system, endpoint)
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/leafz",
		}
	}
	return nc
}

// Describe destribes the list of prometheus descriptors available
// to be scraped.
func (nc *leafzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.leafNodesTotal
	nc.leafMetrics.Describe(ch)
}

// Collect gathers the server leafz metrics.
func (nc *leafzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Leafz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		for _, lf := range resp.Leafs {
			nc.leafMetrics.Collect(server, lf, ch)
		}
		ch <- prometheus.MustNewConstMetric(nc.leafNodesTotal, prometheus.GaugeValue,
			float64(resp.LeafNodes), server.ID)
	}
}

// leafMetrics has all of the prometheus descriptors related to
// each of the leaf node connections of the server being scraped.
type leafMetrics struct {
	info                   *prometheus.Desc
	connRtt                *prometheus.Desc
	connInMsgs             *prometheus.Desc
	connOutMsgs            *prometheus.Desc
	connInBytes            *prometheus.Desc
	connOutBytes           *prometheus.Desc
	connSubscriptionsTotal *prometheus.Desc
	connSubscriptions      *prometheus.Desc
}

// newLeafMetrics initializes a new instance of leafMetrics.
func newLeafMetrics(system, endpoint string) *leafMetrics {

	// Base labels that are common to all metrics
	baseLabels := []string{"server_id", "account", "account_id", "ip", "port", "name"}

	leaf := &leafMetrics{
		info: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "info"),
			"info",
			baseLabels,
			nil),
		connRtt: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_rtt"),
			"rtt",
			baseLabels,
			nil),
		connInMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_in_msgs"),
			"in_msgs",
			baseLabels,
			nil),
		connOutMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_out_msgs"),
			"out_msgs",
			baseLabels,
			nil),
		connInBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_in_bytes"),
			"in_bytes",
			baseLabels,
			nil),
		connOutBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_out_bytes"),
			"out_bytes",
			baseLabels,
			nil),
		connSubscriptionsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_subscriptions_total"),
			"subscriptions_total",
			baseLabels,
			nil),
		connSubscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "conn_subscriptions"),
			"subscriptions",
			baseLabels,
			nil),
	}

	return leaf
}

// Describe destribes the list of prometheus descriptors available
// to be scraped.
func (lm *leafMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- lm.info
	ch <- lm.connRtt
	ch <- lm.connInMsgs
	ch <- lm.connOutMsgs
	ch <- lm.connInBytes
	ch <- lm.connOutBytes
	ch <- lm.connSubscriptionsTotal
	ch <- lm.connSubscriptions
}

// Collect collects all the metrics about the a leafnode connection.
func (lm *leafMetrics) Collect(server *CollectedServer, lf *Leaf, ch chan<- prometheus.Metric) {

	// Base labels that are common to all metrics
	baseLabels := []string{server.ID, lf.Account, lf.Account, lf.IP, fmt.Sprint(lf.Port), lf.Name}

	ch <- prometheus.MustNewConstMetric(lm.info, prometheus.GaugeValue, float64(1.0),
		baseLabels...)

	rtt, _ := time.ParseDuration(lf.RTT)
	ch <- prometheus.MustNewConstMetric(lm.connRtt, prometheus.GaugeValue, rtt.Seconds(),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(lm.connInMsgs, prometheus.GaugeValue, float64(lf.InMsgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(lm.connOutMsgs, prometheus.GaugeValue, float64(lf.OutMsgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(lm.connInBytes, prometheus.GaugeValue, float64(lf.InBytes),
		baseLabels...)
	ch <- prometheus.MustNewConstMetric(lm.connOutBytes, prometheus.GaugeValue, float64(lf.OutBytes),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(lm.connSubscriptionsTotal, prometheus.GaugeValue, float64(lf.Subscriptions),
		baseLabels...)

	for _, sub := range lf.SubscriptionsList {
		ch <- prometheus.MustNewConstMetric(lm.connSubscriptions, prometheus.GaugeValue, float64(0.0),
			server.ID, lf.Account, lf.Account, lf.IP, fmt.Sprint(lf.Port), lf.Name, sub)
	}
}

// Leafz output
type Leafz struct {
	LeafNodes int     `json:"leafnodes"`
	Leafs     []*Leaf `json:"leafs"`
}

// Leaf output
type Leaf struct {
	Name              string   `json:"name"`
	Account           string   `json:"account"`
	IP                string   `json:"ip"`
	Port              int      `json:"port"`
	RTT               string   `json:"rtt"`
	InMsgs            int      `json:"in_msgs"`
	OutMsgs           int      `json:"out_msgs"`
	InBytes           int      `json:"in_bytes"`
	OutBytes          int      `json:"out_bytes"`
	Subscriptions     int      `json:"subscriptions"`
	SubscriptionsList []string `json:"subscriptions_list"`
}
