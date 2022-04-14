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
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func isGatewayzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "gatewayz"
}

type gatewayzCollector struct {
	sync.Mutex

	httpClient       *http.Client
	servers          []*CollectedServer
	outboundGateways *gateway
	inboundGateways  *gateway
}

func newGatewayzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &gatewayzCollector{
		httpClient:       http.DefaultClient,
		outboundGateways: newGateway(system, endpoint, "outbound_gateway"),
		inboundGateways:  newGateway(system, endpoint, "inbound_gateway"),
	}
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/gatewayz",
		}
	}
	return nc
}

func (nc *gatewayzCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.outboundGateways.Describe(ch)
	nc.inboundGateways.Describe(ch)
}

// Collect gathers the server gatewayz metrics.
func (nc *gatewayzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Gatewayz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		for obgwName, obgw := range resp.OutboundGateways {
			nc.outboundGateways.Collect(server, resp.Name, obgwName, obgw, ch)
		}
		for ibgwName, ibgws := range resp.InboundGateways {
			for _, ibgw := range ibgws {
				nc.inboundGateways.Collect(server, resp.Name, ibgwName, ibgw, ch)
			}
		}
	}
}

// gateway
type gateway struct {
	configured        *prometheus.Desc
	connStart         *prometheus.Desc
	connLastActivity  *prometheus.Desc
	connUptime        *prometheus.Desc
	connIdle          *prometheus.Desc
	connRtt           *prometheus.Desc
	connPendingBytes  *prometheus.Desc
	connInMsgs        *prometheus.Desc
	connOutMsgs       *prometheus.Desc
	connInBytes       *prometheus.Desc
	connOutBytes      *prometheus.Desc
	connSubscriptions *prometheus.Desc
}

func newGateway(system, endpoint, gwType string) *gateway {
	gw := &gateway{
		configured: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_configured"),
			"configured",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connStart: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_start_time_seconds"),
			"conn_start_time_seconds",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connLastActivity: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_last_activity_seconds"),
			"conn_last_activity_seconds",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connUptime: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_uptime_seconds"),
			"conn_uptime_seconds",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connIdle: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_idle_seconds"),
			"conn_idle_seconds",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connRtt: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_rtt"),
			"rtt",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connPendingBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_pending_bytes"),
			"pending_bytes",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connInMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_in_msgs"),
			"in_msgs",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connOutMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_out_msgs"),
			"out_msgs",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connInBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_in_bytes"),
			"in_bytes",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connOutBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_out_bytes"),
			"out_bytes",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
		connSubscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, gwType+"_conn_subscriptions"),
			"subscriptions",
			[]string{"gateway_name", "cid", "remote_gateway_name", "server_id"},
			nil),
	}

	return gw
}

// Describe
func (gw *gateway) Describe(ch chan<- *prometheus.Desc) {
	ch <- gw.configured
	ch <- gw.connStart
	ch <- gw.connLastActivity
	ch <- gw.connUptime
	ch <- gw.connIdle
	ch <- gw.connRtt
	ch <- gw.connPendingBytes
	ch <- gw.connInMsgs
	ch <- gw.connOutMsgs
	ch <- gw.connInBytes
	ch <- gw.connOutBytes
	ch <- gw.connSubscriptions
}

func (gw *gateway) Collect(server *CollectedServer, lgwName, rgwName string,
	rgw *RemoteGatewayz, ch chan<- prometheus.Metric) {

	cid := strconv.FormatUint(rgw.Connection.Cid, 10)
	idle, _ := time.ParseDuration(rgw.Connection.Idle)
	rtt, _ := time.ParseDuration(rgw.Connection.RTT)
	uptime, _ := time.ParseDuration(rgw.Connection.Uptime)

	ch <- prometheus.MustNewConstMetric(gw.configured, prometheus.GaugeValue,
		boolToFloat(rgw.IsConfigured), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connStart, prometheus.GaugeValue,
		float64(rgw.Connection.Start.Unix()), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connLastActivity, prometheus.GaugeValue,
		float64(rgw.Connection.LastActivity.Unix()), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connUptime, prometheus.GaugeValue,
		uptime.Seconds(), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connIdle, prometheus.GaugeValue,
		idle.Seconds(), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connRtt, prometheus.GaugeValue,
		rtt.Seconds(), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connPendingBytes, prometheus.GaugeValue,
		float64(rgw.Connection.Pending), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connInMsgs, prometheus.GaugeValue,
		float64(rgw.Connection.InMsgs), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connOutMsgs, prometheus.GaugeValue,
		float64(rgw.Connection.OutMsgs), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connInBytes, prometheus.GaugeValue,
		float64(rgw.Connection.InBytes), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connOutBytes, prometheus.GaugeValue,
		float64(rgw.Connection.OutBytes), lgwName, cid, rgwName, server.ID)
	ch <- prometheus.MustNewConstMetric(gw.connSubscriptions, prometheus.GaugeValue,
		float64(rgw.Connection.NumSubs), lgwName, cid, rgwName, server.ID)
}

// Gatewayz output
type Gatewayz struct {
	Name             string                       `json:"name"`
	OutboundGateways map[string]*RemoteGatewayz   `json:"outbound_gateways"`
	InboundGateways  map[string][]*RemoteGatewayz `json:"inbound_gateways"`
}

// RemoteGatewayz output
type RemoteGatewayz struct {
	IsConfigured bool     `json:"configured"`
	Connection   ConnInfo `json:"connection,omitempty"`
}

// ConnInfo output
type ConnInfo struct {
	Cid          uint64    `json:"cid"`
	Start        time.Time `json:"start"`
	LastActivity time.Time `json:"last_activity"`
	RTT          string    `json:"rtt,omitempty"`
	Uptime       string    `json:"uptime"`
	Idle         string    `json:"idle"`
	Pending      int       `json:"pending_bytes"`
	InMsgs       int64     `json:"in_msgs"`
	OutMsgs      int64     `json:"out_msgs"`
	InBytes      int64     `json:"in_bytes"`
	OutBytes     int64     `json:"out_bytes"`
	NumSubs      uint32    `json:"subscriptions"`
}
