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
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type replicatorCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer

	// Replicator metrics
	startTime    *prometheus.Desc
	currentTime  *prometheus.Desc
	requestCount *prometheus.Desc
	info         *prometheus.Desc

	// Individual connector metrics
	connected     *prometheus.Desc
	connects      *prometheus.Desc
	disconnects   *prometheus.Desc
	bytesIn       *prometheus.Desc
	bytesOut      *prometheus.Desc
	messagesIn    *prometheus.Desc
	messagesOut   *prometheus.Desc
	count         *prometheus.Desc
	movingAverage *prometheus.Desc
	quintile50    *prometheus.Desc
	quintile75    *prometheus.Desc
	quintile90    *prometheus.Desc
	quintile95    *prometheus.Desc
}

type replicatorConnector struct {
	Name          string  `json:"name"`
	ID            string  `json:"id"`
	Connected     bool    `json:"connected"`
	Connects      int64   `json:"connects"`
	Disconnects   int64   `json:"disconnects"`
	BytesIn       int64   `json:"bytes_in"`
	BytesOut      int64   `json:"bytes_out"`
	MessagesIn    int64   `json:"msg_in"`
	MessagesOut   int64   `json:"msg_out"`
	RequestCount  int64   `json:"count"`
	MovingAverage float64 `json:"rma"`
	Quintile50    float64 `json:"q50"`
	Quintile75    float64 `json:"q75"`
	Quintile90    float64 `json:"q90"`
	Quintile95    float64 `json:"q95"`
}

type replicatorVarz struct {
	StartTime    int                   `json:"start_time"`
	CurrentTime  int                   `json:"current_time"`
	Uptime       string                `json:"uptime_time"`
	RequestCount int                   `json:"request_count"`
	Connectors   []replicatorConnector `json:"connectors"`
	HTTPRequests map[string]int64      `json:"http_requests"`
}

func isReplicatorEndpoint(system, endpoint string) bool {
	return system == ReplicatorSystem && endpoint == "varz"
}

func newReplicatorCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &replicatorCollector{
		httpClient: http.DefaultClient,
		startTime: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "start_time"),
			"Start Time",
			[]string{"server_id"},
			nil,
		),
		currentTime: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "current_time"),
			"Current Time",
			[]string{"server_id"},
			nil,
		),
		requestCount: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "request_count"),
			"Request Count",
			[]string{"server_id"},
			nil,
		),
		info: prometheus.NewDesc(
			prometheus.BuildFQName(system, "server", "info"),
			"Info",
			[]string{"server_id", "uptime"},
			nil,
		),
		connected: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "connected"),
			"Connected",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		connects: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "connects"),
			"Connects",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		disconnects: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "disconnects"),
			"Disonnects",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		bytesIn: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "bytes_in"),
			"Bytes In",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		bytesOut: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "bytes_out"),
			"Bytes Out",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		messagesIn: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "messages_in"),
			"Messages In",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		messagesOut: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "messages_out"),
			"Messages Out",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		count: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "request_count"),
			"Connector Request Count",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		movingAverage: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "moving_average"),
			"Connector Moving Average",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		quintile50: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "quintile_50"),
			"Connector 50th Quintile",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		quintile75: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "quintile_75"),
			"Connector 75th Quintile",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		quintile90: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "quintile_90"),
			"Connector 90th Quintile",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
		quintile95: prometheus.NewDesc(
			prometheus.BuildFQName(system, "connector", "quintile_95"),
			"Connector 95th Quintile",
			[]string{"server_id", "connector_id", "name"},
			nil,
		),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/varz",
		}
	}

	return nc
}

func (nc *replicatorCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.startTime
	ch <- nc.currentTime
	ch <- nc.requestCount
	ch <- nc.info
	ch <- nc.connected
	ch <- nc.connects
	ch <- nc.disconnects
	ch <- nc.bytesIn
	ch <- nc.bytesOut
	ch <- nc.messagesIn
	ch <- nc.messagesOut
	ch <- nc.count
	ch <- nc.movingAverage
	ch <- nc.quintile50
	ch <- nc.quintile75
	ch <- nc.quintile90
	ch <- nc.quintile95
}

// Collect gathers the streaming server serverz metrics.
func (nc *replicatorCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp replicatorVarz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v\n", server.ID, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(nc.requestCount, prometheus.CounterValue, float64(resp.RequestCount), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.startTime, prometheus.CounterValue, float64(resp.StartTime), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.currentTime, prometheus.CounterValue, float64(resp.CurrentTime), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.info, prometheus.GaugeValue, 1, server.ID, resp.Uptime)
		for _, c := range resp.Connectors {

			labelValues := []string{server.ID, c.ID, c.Name}

			connected := float64(0)
			if c.Connected == true {
				connected = float64(1)
			}
			ch <- prometheus.MustNewConstMetric(nc.connected, prometheus.GaugeValue, connected, labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.connects, prometheus.GaugeValue, float64(c.Connects), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.disconnects, prometheus.GaugeValue, float64(c.Disconnects), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.bytesIn, prometheus.GaugeValue, float64(c.BytesIn), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.bytesOut, prometheus.GaugeValue, float64(c.BytesOut), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.messagesIn, prometheus.GaugeValue, float64(c.MessagesIn), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.messagesOut, prometheus.GaugeValue, float64(c.MessagesOut), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.count, prometheus.GaugeValue, float64(c.RequestCount), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.movingAverage, prometheus.GaugeValue, float64(c.MovingAverage), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.quintile50, prometheus.GaugeValue, float64(c.Quintile50), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.quintile75, prometheus.GaugeValue, float64(c.Quintile75), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.quintile90, prometheus.GaugeValue, float64(c.Quintile90), labelValues...)
			ch <- prometheus.MustNewConstMetric(nc.quintile95, prometheus.GaugeValue, float64(c.Quintile95), labelValues...)
		}
	}
}
