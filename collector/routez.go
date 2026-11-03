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

// Package collector has various collector utilities and implementations.
package collector

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func isRoutezEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "routez"
}

type routezCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer
	numRoutes  *prometheus.Desc
	serverID   *prometheus.Desc
	serverName *prometheus.Desc
	metrics    *routeMetrics
}

func newRoutezCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	serverLabels := []string{"server_id"}
	valueLabels := []string{"server_id", "value"}
	rc := &routezCollector{
		httpClient: http.DefaultClient,
		numRoutes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "num_routes"),
			"num_routes",
			serverLabels,
			nil,
		),
		serverID: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "server_id"),
			"server_id",
			valueLabels,
			nil,
		),
		serverName: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "server_name"),
			"server_name",
			valueLabels,
			nil,
		),
		metrics: newRouteMetrics(system, endpoint),
		servers: make([]*CollectedServer, len(servers)),
	}
	for i, server := range servers {
		rc.servers[i] = &CollectedServer{
			ID:  server.ID,
			URL: server.URL + "/routez",
		}
	}
	return rc
}

func (rc *routezCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.numRoutes
	ch <- rc.serverID
	ch <- rc.serverName
	rc.metrics.Describe(ch)
}

func (rc *routezCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range rc.servers {
		var resp Routez
		if err := getMetricURL(rc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(rc.numRoutes, prometheus.GaugeValue,
			float64(resp.NumRoutes), server.ID)
		ch <- prometheus.MustNewConstMetric(rc.serverID, prometheus.GaugeValue,
			1, server.ID, resp.ServerID)
		ch <- prometheus.MustNewConstMetric(rc.serverName, prometheus.GaugeValue,
			1, server.ID, resp.ServerName)
		for _, route := range resp.Routes {
			rc.metrics.Collect(server, route, ch)
		}
	}
}

type routeMetric struct {
	desc  *prometheus.Desc
	value func(*RouteInfo) float64
}

type routeMetrics struct {
	info   *prometheus.Desc
	gauges []routeMetric
}

func newRouteMetrics(system, endpoint string) *routeMetrics {
	baseLabels := []string{"server_id", "rid", "remote_id", "remote_name", "account", "ip", "port"}
	infoLabels := append(append([]string{}, baseLabels...), "is_configured", "did_solicit", "compression")
	rm := &routeMetrics{
		info: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_info"),
			"Route connection information.",
			infoLabels,
			nil,
		),
	}

	definitions := []struct {
		name  string
		help  string
		value func(*RouteInfo) float64
	}{
		{"route_pending_bytes", "Bytes pending on the route connection.", func(route *RouteInfo) float64 {
			return float64(route.Pending)
		}},
		{"route_rtt_seconds", "Round-trip time of the route connection in seconds.", func(route *RouteInfo) float64 {
			return routeRTTSeconds(route.RTT)
		}},
		{"route_in_msgs", "Messages received from the route connection.", func(route *RouteInfo) float64 {
			return float64(route.InMsgs)
		}},
		{"route_out_msgs", "Messages sent to the route connection.", func(route *RouteInfo) float64 {
			return float64(route.OutMsgs)
		}},
		{"route_in_bytes", "Bytes received from the route connection.", func(route *RouteInfo) float64 {
			return float64(route.InBytes)
		}},
		{"route_out_bytes", "Bytes sent to the route connection.", func(route *RouteInfo) float64 {
			return float64(route.OutBytes)
		}},
		{"route_subscriptions", "Subscriptions on the route connection.", func(route *RouteInfo) float64 {
			return float64(route.Subscriptions)
		}},
		{"route_uptime_seconds", "Uptime of the route connection in seconds.", func(route *RouteInfo) float64 {
			return routeDurationSeconds(route.Uptime)
		}},
		{"route_idle_seconds", "Idle time of the route connection in seconds.", func(route *RouteInfo) float64 {
			return routeDurationSeconds(route.Idle)
		}},
		{"route_start_time_seconds", "Start time of the route connection in Unix seconds.", func(route *RouteInfo) float64 {
			return routeTimeSeconds(route.Start)
		}},
		{
			"route_last_activity_seconds",
			"Last activity time of the route connection in Unix seconds.",
			func(route *RouteInfo) float64 {
				return routeTimeSeconds(route.LastActivity)
			},
		},
	}

	rm.gauges = make([]routeMetric, 0, len(definitions))
	for _, definition := range definitions {
		rm.gauges = append(rm.gauges, routeMetric{
			desc: prometheus.NewDesc(
				prometheus.BuildFQName(system, endpoint, definition.name),
				definition.help,
				baseLabels,
				nil,
			),
			value: definition.value,
		})
	}
	return rm
}

func (rm *routeMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- rm.info
	for _, gauge := range rm.gauges {
		ch <- gauge.desc
	}
}

func (rm *routeMetrics) Collect(server *CollectedServer, route *RouteInfo, ch chan<- prometheus.Metric) {
	baseLabels := []string{
		server.ID,
		strconv.FormatUint(route.RID, 10),
		route.RemoteID,
		route.RemoteName,
		route.Account,
		route.IP,
		strconv.Itoa(route.Port),
	}
	infoLabels := append(append([]string{}, baseLabels...),
		strconv.FormatBool(route.IsConfigured),
		strconv.FormatBool(route.DidSolicit),
		route.Compression)

	ch <- prometheus.MustNewConstMetric(rm.info, prometheus.GaugeValue, 1, infoLabels...)
	for _, gauge := range rm.gauges {
		ch <- prometheus.MustNewConstMetric(gauge.desc, prometheus.GaugeValue,
			gauge.value(route), baseLabels...)
	}
}

func routeRTTSeconds(value string) float64 {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0
	}
	return duration.Seconds()
}

func routeDurationSeconds(value string) float64 {
	milliseconds := parseDuration(value)
	if milliseconds < 0 {
		return 0
	}
	return milliseconds / 1000
}

func routeTimeSeconds(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.Unix())
}

// Routez is the response from the NATS server routez endpoint.
type Routez struct {
	ServerID   string       `json:"server_id,omitempty"`
	ServerName string       `json:"server_name,omitempty"`
	NumRoutes  int          `json:"num_routes,omitempty"`
	Routes     []*RouteInfo `json:"routes,omitempty"`
}

// RouteInfo contains metrics and identifying data for one route connection.
type RouteInfo struct {
	RID           uint64    `json:"rid,omitempty"`
	RemoteID      string    `json:"remote_id,omitempty"`
	RemoteName    string    `json:"remote_name,omitempty"`
	DidSolicit    bool      `json:"did_solicit,omitempty"`
	IsConfigured  bool      `json:"is_configured,omitempty"`
	IP            string    `json:"ip,omitempty"`
	Port          int       `json:"port,omitempty"`
	Start         time.Time `json:"start,omitempty"`
	LastActivity  time.Time `json:"last_activity,omitempty"`
	RTT           string    `json:"rtt,omitempty"`
	Uptime        string    `json:"uptime,omitempty"`
	Idle          string    `json:"idle,omitempty"`
	Pending       int       `json:"pending_size,omitempty"`
	InMsgs        int64     `json:"in_msgs,omitempty"`
	OutMsgs       int64     `json:"out_msgs,omitempty"`
	InBytes       int64     `json:"in_bytes,omitempty"`
	OutBytes      int64     `json:"out_bytes,omitempty"`
	Subscriptions uint32    `json:"subscriptions,omitempty"`
	Account       string    `json:"account,omitempty"`
	Compression   string    `json:"compression,omitempty"`
}
