// Copyright 2023 The NATS Authors
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

const (
	healthzEndpoint              = "healthz"
	healthzJsEnabledOnlyEndpoint = "healthz_js_enabled_only"
	healthzJsServerOnlyEndpoint  = "healthz_js_server_only"
)

func isHealthzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && (endpoint == healthzEndpoint ||
		endpoint == healthzJsEnabledOnlyEndpoint ||
		endpoint == healthzJsServerOnlyEndpoint)
}

type healthzCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer

	status *prometheus.Desc
}

func newHealthzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &healthzCollector{
		httpClient: http.DefaultClient,
		status: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "status"),
			"status",
			[]string{"server_id", "value"},
			nil,
		),
	}

	healthzURLPathAndQuerryArgs := healthzEndpoint
	switch endpoint {
	case healthzJsEnabledOnlyEndpoint:
		healthzURLPathAndQuerryArgs = healthzEndpoint + "?js-enabled-only=true"
	case healthzJsServerOnlyEndpoint:
		healthzURLPathAndQuerryArgs = healthzEndpoint + "?js-server-only=true"
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/" + healthzURLPathAndQuerryArgs,
		}
	}

	return nc
}

func (nc *healthzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.status
}

// Collect gathers the server healthz metrics.
func (nc *healthzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var health Healthz
		if err := getMetricURL(nc.httpClient, server.URL, &health); err != nil {
			Debugf("error collecting server %s: %v", server.ID, err)
			health.Error = err.Error()
		}

		var (
			status float64 = 1
			value          = health.Error
		)

		if health.Status == "ok" {
			status = 0
			value = health.Status
		}

		ch <- prometheus.MustNewConstMetric(nc.status, prometheus.GaugeValue, status, server.ID, value)
	}
}

// Healthz output
type Healthz struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
