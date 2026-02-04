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
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var uptimeRegex = regexp.MustCompile(`(?:(\d+)y)?(?:(\d+)d)?(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?`)

// parseNATSUptime parses NATS uptime format like "7d13h49m13s" or "1y2d3h4m5s"
// into a time.Duration. Returns error if the format is invalid.
func parseNATSUptime(uptime string) (time.Duration, error) {
	if uptime == "" {
		return 0, fmt.Errorf("empty uptime string")
	}

	matches := uptimeRegex.FindStringSubmatch(uptime)
	if matches == nil || matches[0] != uptime {
		return 0, fmt.Errorf("invalid uptime format: %s", uptime)
	}

	var total time.Duration

	// Parse years (matches[1])
	if matches[1] != "" {
		years, _ := strconv.ParseInt(matches[1], 10, 64)
		total += time.Duration(years) * 365 * 24 * time.Hour
	}

	// Parse days (matches[2])
	if matches[2] != "" {
		days, _ := strconv.ParseInt(matches[2], 10, 64)
		total += time.Duration(days) * 24 * time.Hour
	}

	// Parse hours (matches[3])
	if matches[3] != "" {
		hours, _ := strconv.ParseInt(matches[3], 10, 64)
		total += time.Duration(hours) * time.Hour
	}

	// Parse minutes (matches[4])
	if matches[4] != "" {
		minutes, _ := strconv.ParseInt(matches[4], 10, 64)
		total += time.Duration(minutes) * time.Minute
	}

	// Parse seconds (matches[5])
	if matches[5] != "" {
		seconds, _ := strconv.ParseInt(matches[5], 10, 64)
		total += time.Duration(seconds) * time.Second
	}

	if total == 0 {
		return 0, fmt.Errorf("invalid uptime format: %s", uptime)
	}

	return total, nil
}

func isRoutezEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "routez"
}

type routezCollector struct {
	sync.Mutex

	httpClient   *http.Client
	servers      []*CollectedServer
	serverID     *prometheus.Desc
	serverName   *prometheus.Desc
	numRoutes    *prometheus.Desc
	routeMetrics *routeMetrics
}

func newRoutezCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	rc := &routezCollector{
		httpClient:   http.DefaultClient,
		routeMetrics: newRouteMetrics(system, endpoint),
	}
	rc.serverID = prometheus.NewDesc(
		prometheus.BuildFQName(system, endpoint, "server_id"),
		"server_id",
		[]string{"server_id", "value"},
		nil)
	rc.serverName = prometheus.NewDesc(
		prometheus.BuildFQName(system, endpoint, "server_name"),
		"server_name",
		[]string{"server_id", "value"},
		nil)
	rc.numRoutes = prometheus.NewDesc(
		prometheus.BuildFQName(system, endpoint, "num_routes"),
		"num_routes",
		[]string{"server_id"},
		nil)
	rc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		rc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/routez",
		}
	}
	return rc
}

func (rc *routezCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- rc.serverID
	ch <- rc.serverName
	ch <- rc.numRoutes
	rc.routeMetrics.Describe(ch)
}

func (rc *routezCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range rc.servers {
		var resp Routez
		if err := getMetricURL(rc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(rc.serverID, prometheus.GaugeValue,
			1, server.ID, resp.ServerID)
		ch <- prometheus.MustNewConstMetric(rc.serverName, prometheus.GaugeValue,
			1, server.ID, resp.ServerName)
		ch <- prometheus.MustNewConstMetric(rc.numRoutes, prometheus.GaugeValue,
			float64(resp.NumRoutes), server.ID)

		for _, route := range resp.Routes {
			rc.routeMetrics.Collect(server, &route, ch)
		}
	}
}

type routeMetrics struct {
	info          *prometheus.Desc
	rtt           *prometheus.Desc
	subscriptions *prometheus.Desc
	uptime        *prometheus.Desc
	inMsgs        *prometheus.Desc
	outMsgs       *prometheus.Desc
	inBytes       *prometheus.Desc
	outBytes      *prometheus.Desc
}

func newRouteMetrics(system, endpoint string) *routeMetrics {
	// Base labels for all metrics (route identifier)
	baseLabels := []string{"server_id", "rid"}
	// Extended labels for info metric
	infoLabels := []string{"server_id", "rid", "remote_id", "remote_name", "account", "ip", "compression"}

	return &routeMetrics{
		info: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_info"),
			"route_info",
			infoLabels,
			nil),
		rtt: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_rtt_seconds"),
			"rtt_seconds",
			baseLabels,
			nil),
		subscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_subscriptions"),
			"subscriptions",
			baseLabels,
			nil),
		uptime: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_uptime_seconds"),
			"uptime_seconds",
			baseLabels,
			nil),
		inMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_in_msgs"),
			"in_msgs",
			baseLabels,
			nil),
		outMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_out_msgs"),
			"out_msgs",
			baseLabels,
			nil),
		inBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_in_bytes"),
			"in_bytes",
			baseLabels,
			nil),
		outBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "route_out_bytes"),
			"out_bytes",
			baseLabels,
			nil),
	}
}

func (rm *routeMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- rm.info
	ch <- rm.rtt
	ch <- rm.subscriptions
	ch <- rm.uptime
	ch <- rm.inMsgs
	ch <- rm.outMsgs
	ch <- rm.inBytes
	ch <- rm.outBytes
}

func (rm *routeMetrics) Collect(server *CollectedServer, route *RouteInfo, ch chan<- prometheus.Metric) {
	// Base labels for route identification
	baseLabels := []string{server.ID, fmt.Sprint(route.Rid)}
	// Extended labels for info metric
	infoLabels := []string{server.ID, fmt.Sprint(route.Rid), route.RemoteID, route.RemoteName, route.Account, route.IP, route.Compression}

	ch <- prometheus.MustNewConstMetric(rm.info, prometheus.GaugeValue, float64(1.0),
		infoLabels...)

	// Only export RTT if available
	if route.RTT != "" {
		rtt, err := time.ParseDuration(route.RTT)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(rm.rtt, prometheus.GaugeValue, rtt.Seconds(),
				baseLabels...)
		}
	}

	ch <- prometheus.MustNewConstMetric(rm.subscriptions, prometheus.GaugeValue, float64(route.Subscriptions),
		baseLabels...)

	// Only export uptime if available
	if route.Uptime != "" {
		// Try standard Go duration format first
		uptime, err := time.ParseDuration(route.Uptime)
		if err != nil {
			// Try NATS-specific format (e.g., "7d13h49m13s")
			uptime, err = parseNATSUptime(route.Uptime)
		}
		if err == nil {
			ch <- prometheus.MustNewConstMetric(rm.uptime, prometheus.GaugeValue, uptime.Seconds(),
				baseLabels...)
		}
	}

	ch <- prometheus.MustNewConstMetric(rm.inMsgs, prometheus.GaugeValue, float64(route.InMsgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(rm.outMsgs, prometheus.GaugeValue, float64(route.OutMsgs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(rm.inBytes, prometheus.GaugeValue, float64(route.InBytes),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(rm.outBytes, prometheus.GaugeValue, float64(route.OutBytes),
		baseLabels...)
}

type Routez struct {
	ServerID   string      `json:"server_id"`
	ServerName string      `json:"server_name"`
	NumRoutes  int         `json:"num_routes"`
	Routes     []RouteInfo `json:"routes"`
}

type RouteInfo struct {
	Rid           uint64 `json:"rid"`
	RemoteID      string `json:"remote_id"`
	RemoteName    string `json:"remote_name"`
	Account       string `json:"account,omitempty"`
	IP            string `json:"ip,omitempty"`
	Compression   string `json:"compression,omitempty"`
	RTT           string `json:"rtt,omitempty"`
	Uptime        string `json:"uptime,omitempty"`
	InMsgs        int64  `json:"in_msgs"`
	OutMsgs       int64  `json:"out_msgs"`
	InBytes       int64  `json:"in_bytes"`
	OutBytes      int64  `json:"out_bytes"`
	Subscriptions uint32 `json:"subscriptions"`
}
