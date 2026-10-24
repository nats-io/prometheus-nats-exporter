// Copyright 2026 The NATS Authors
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

package collector

import (
	"net/http"
	"net/url"
	"time"

	nats "github.com/nats-io/nats-server/v2/server"
	"github.com/prometheus/client_golang/prometheus"
)

const ipqueueszEndpoint = "ipqueuesz"

type ipqueueszCollector struct {
	httpClient *http.Client
	servers    []*CollectedServer

	pending    *prometheus.Desc
	inProgress *prometheus.Desc
}

// NewIpqueueszCollector creates a collector for the server's internal queues.
// The server only reports queues that are not empty; filter, when set, keeps
// only queues whose name contains it (e.g. " messages" for stream ingest).
func NewIpqueueszCollector(prefix, filter string, servers []*CollectedServer) prometheus.Collector {
	system := getSystem(CoreSystem, prefix)
	labels := []string{"server_id", "queue"}
	nc := &ipqueueszCollector{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		pending: prometheus.NewDesc(
			prometheus.BuildFQName(system, ipqueueszEndpoint, "pending"),
			"Number of items waiting in the internal queue",
			labels,
			nil,
		),
		inProgress: prometheus.NewDesc(
			prometheus.BuildFQName(system, ipqueueszEndpoint, "in_progress"),
			"Number of items taken from the internal queue and still being processed",
			labels,
			nil,
		),
	}

	path := ipqueueszEndpoint
	if filter != "" {
		path += "?queues=" + url.QueryEscape(filter)
	}
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/" + path,
		}
	}
	return nc
}

func (nc *ipqueueszCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.pending
	ch <- nc.inProgress
}

// Collect gathers the server ipqueuesz metrics.
func (nc *ipqueueszCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var queues nats.IpqueueszStatus
		if err := getMetricURL(nc.httpClient, server.URL, &queues); err != nil {
			Debugf("error collecting server %s: %v", server.ID, err)
			continue
		}
		for name, q := range queues {
			ch <- prometheus.MustNewConstMetric(nc.pending, prometheus.GaugeValue, float64(q.Pending), server.ID, name)
			ch <- prometheus.MustNewConstMetric(nc.inProgress, prometheus.GaugeValue, float64(q.InProgress), server.ID, name)
		}
	}
}
