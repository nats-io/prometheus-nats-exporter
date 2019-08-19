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

func isConnzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "connz"
}

type connzCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer

	numConnections *prometheus.Desc
	total          *prometheus.Desc
	offset         *prometheus.Desc
	limit          *prometheus.Desc
	pendingBytes   *prometheus.Desc
}

func newConnzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &connzCollector{
		httpClient: http.DefaultClient,
		numConnections: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "num_connections"),
			"num_connections",
			[]string{"server_id"},
			nil,
		),
		offset: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "offset"),
			"offset",
			[]string{"server_id"},
			nil,
		),
		total: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "total"),
			"total",
			[]string{"server_id"},
			nil,
		),
		limit: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit"),
			"limit",
			[]string{"server_id"},
			nil,
		),
		pendingBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "pending_bytes"),
			"pending_bytes",
			[]string{"server_id"},
			nil,
		),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/connz",
		}
	}

	return nc
}

func (nc *connzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.limit
}

// Collect gathers the streaming server serverz metrics.
func (nc *connzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Connz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		var pendingBytes = 0
		for _, conn := range resp.Connections {
			pendingBytes += conn.PendingBytes
		}

		ch <- prometheus.MustNewConstMetric(nc.numConnections, prometheus.GaugeValue, float64(resp.NumConnections), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.total, prometheus.GaugeValue, float64(resp.Total), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.offset, prometheus.GaugeValue, float64(resp.Offset), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.limit, prometheus.GaugeValue, float64(resp.Limit), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.pendingBytes, prometheus.GaugeValue, float64(pendingBytes), server.ID)
	}
}

// Connz output
type Connz struct {
	NumConnections int `json:"num_connections"`
	Total          int `json:"total"`
	Offset         int `json:"offset"`
	Limit          int `json:"limit"`
	Connections    []struct {
		PendingBytes int `json:"pending_bytes"`
	} `json:"connections"`
}
