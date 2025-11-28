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
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	connzEndpoint         = "connz"
	connzDetailedEndpoint = "connz_detailed"
)

func isConnzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && (endpoint == connzEndpoint || endpoint == connzDetailedEndpoint)
}

type connzCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer
	detailed   bool

	numConnections     *prometheus.Desc
	total              *prometheus.Desc
	offset             *prometheus.Desc
	limit              *prometheus.Desc
	totalPendingBytes  *prometheus.Desc
	totalSubscriptions *prometheus.Desc
	totalInBytes       *prometheus.Desc
	totalOutBytes      *prometheus.Desc
	totalInMsgs        *prometheus.Desc
	totalOutMsgs       *prometheus.Desc
	connzCollectorDetailed
}

type connzCollectorDetailed struct {
	pendingBytes  *prometheus.Desc
	subscriptions *prometheus.Desc
	inBytes       *prometheus.Desc
	outBytes      *prometheus.Desc
	inMsgs        *prometheus.Desc
	outMsgs       *prometheus.Desc
	start         *prometheus.Desc
	lastActivity  *prometheus.Desc
	rtt           *prometheus.Desc
	uptime        *prometheus.Desc
	idle          *prometheus.Desc
}

func createConnzCollector(system string) *connzCollector {
	summaryLabels := []string{"server_id"}
	return &connzCollector{
		httpClient: http.DefaultClient,
		numConnections: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "num_connections"),
			"num_connections",
			summaryLabels,
			nil,
		),
		offset: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "offset"),
			"offset",
			summaryLabels,
			nil,
		),
		total: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "total"),
			"total",
			summaryLabels,
			nil,
		),
		limit: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "limit"),
			"limit",
			summaryLabels,
			nil,
		),
		totalPendingBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "pending_bytes"),
			"pending_bytes",
			summaryLabels,
			nil,
		),
		totalSubscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "subscriptions"),
			"subscriptions",
			summaryLabels,
			nil,
		),
		totalInBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "in_bytes"),
			"in_bytes",
			summaryLabels,
			nil,
		),
		totalOutBytes: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "out_bytes"),
			"out_bytes",
			summaryLabels,
			nil,
		),
		totalInMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "in_msgs"),
			"in_msgs",
			summaryLabels,
			nil,
		),
		totalOutMsgs: prometheus.NewDesc(
			prometheus.BuildFQName(system, connzEndpoint, "out_msgs"),
			"out_msgs",
			summaryLabels,
			nil,
		),
	}
}

func createConnzDetailedCollector(system string) *connzCollector {
	connzCollector := createConnzCollector(system)
	detailLabels := []string{"server_id", "cid", "kind", "type", "ip", "port", "name", "name_tag",
		"account", "account_id", "lang", "version", "tls_version", "tls_cipher_suite"}
	connzCollector.pendingBytes = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "pending_bytes"),
		"pending_bytes",
		detailLabels,
		nil,
	)
	connzCollector.subscriptions = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "subscriptions"),
		"subscriptions",
		detailLabels,
		nil,
	)
	connzCollector.inBytes = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "in_bytes"),
		"in_bytes",
		detailLabels,
		nil,
	)
	connzCollector.outBytes = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "out_bytes"),
		"out_bytes",
		detailLabels,
		nil,
	)
	connzCollector.inMsgs = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "in_msgs"),
		"in_msgs",
		detailLabels,
		nil,
	)
	connzCollector.outMsgs = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "out_msgs"),
		"out_msgs",
		detailLabels,
		nil,
	)
	connzCollector.start = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "start"),
		"epoch time at which the connection was started",
		detailLabels,
		nil,
	)
	connzCollector.lastActivity = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "last_activity"),
		"epoch time at which the last activity was registred",
		detailLabels,
		nil,
	)
	connzCollector.rtt = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "rtt"),
		"response time latency in microseconds",
		detailLabels,
		nil,
	)
	connzCollector.uptime = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "uptime"),
		"uptime duration in milliseconds",
		detailLabels,
		nil,
	)
	connzCollector.idle = prometheus.NewDesc(
		prometheus.BuildFQName(system, connzEndpoint, "idle"),
		"idle time duration in milliseconds",
		detailLabels,
		nil,
	)
	return connzCollector
}

func newConnzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	var nc *connzCollector
	if endpoint == connzDetailedEndpoint {
		nc = createConnzDetailedCollector(system)
		nc.detailed = true
	} else {
		nc = createConnzCollector(system)
	}
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/" + connzEndpoint,
		}

		if nc.detailed {
			nc.servers[i].URL += "?auth=true"
		}
	}
	return nc
}

func (nc *connzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.limit
}

// Collect gathers the server connz metrics.
func (nc *connzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Connz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		var pendingBytes, subscriptions, inBytes, outBytes, inMsgs, outMsgs float64
		for _, conn := range resp.Connections {
			pendingBytes += conn.PendingBytes
			subscriptions += conn.Subscriptions
			inBytes += conn.InBytes
			outBytes += conn.OutBytes
			inMsgs += conn.InMsgs
			outMsgs += conn.OutMsgs
			if nc.detailed {
				detailLabelValues := []string{server.ID, conn.Cid, conn.Kind, conn.Type, conn.IP, conn.Port,
					conn.Name, conn.NameTag, conn.Account, conn.Account, conn.Lang, conn.Version, conn.TLSVersion, conn.TLSCipherSuite}
				ch <- prometheus.MustNewConstMetric(nc.pendingBytes, prometheus.GaugeValue, conn.PendingBytes, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.subscriptions, prometheus.GaugeValue, conn.Subscriptions,
					detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.inBytes, prometheus.CounterValue, conn.InBytes, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.outBytes, prometheus.CounterValue, conn.OutBytes, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.inMsgs, prometheus.CounterValue, conn.InMsgs, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.outMsgs, prometheus.CounterValue, conn.OutMsgs, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.start, prometheus.UntypedValue, conn.Start, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.lastActivity, prometheus.UntypedValue, conn.LastActivity,
					detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.rtt, prometheus.GaugeValue, conn.Rtt, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.uptime, prometheus.UntypedValue, conn.Uptime, detailLabelValues...)
				ch <- prometheus.MustNewConstMetric(nc.idle, prometheus.GaugeValue, conn.Idle, detailLabelValues...)
			}
		}

		ch <- prometheus.MustNewConstMetric(nc.numConnections, prometheus.GaugeValue, resp.NumConnections, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.total, prometheus.GaugeValue, resp.Total, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.offset, prometheus.GaugeValue, resp.Offset, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.limit, prometheus.GaugeValue, resp.Limit, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalPendingBytes, prometheus.GaugeValue, pendingBytes, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalSubscriptions, prometheus.GaugeValue, subscriptions, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalInBytes, prometheus.CounterValue, inBytes, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalOutBytes, prometheus.CounterValue, outBytes, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalInMsgs, prometheus.CounterValue, inMsgs, server.ID)
		ch <- prometheus.MustNewConstMetric(nc.totalOutMsgs, prometheus.CounterValue, outMsgs, server.ID)
	}
}

// Connz output
type Connz struct {
	NumConnections float64           `json:"num_connections"`
	Total          float64           `json:"total"`
	Offset         float64           `json:"offset"`
	Limit          float64           `json:"limit"`
	Connections    []ConnzConnection `json:"connections"`
}

// ConnzConnection represents the connections details
type ConnzConnection struct {
	Cid            string  `json:"cid"`
	Kind           string  `json:"kind"`
	Type           string  `json:"type"`
	IP             string  `json:"ip"`
	Port           string  `json:"port"`
	Start          float64 `json:"start"`
	LastActivity   float64 `json:"last_activity"`
	Rtt            float64 `json:"rtt"`
	Uptime         float64 `json:"uptime"`
	Idle           float64 `json:"idle"`
	PendingBytes   float64 `json:"pending_bytes"`
	InMsgs         float64 `json:"in_msgs"`
	OutMsgs        float64 `json:"out_msgs"`
	InBytes        float64 `json:"in_bytes"`
	OutBytes       float64 `json:"out_bytes"`
	Subscriptions  float64 `json:"subscriptions"`
	Name           string  `json:"name"`
	NameTag        string  `json:"name_tag"`
	Account        string  `json:"account"`
	Lang           string  `json:"lang"`
	Version        string  `json:"version"`
	TLSVersion     string  `json:"tls_version"`
	TLSCipherSuite string  `json:"tls_cipher_suite"`
}

// UnmarshalJSON converts JSON string to struct. This is required as we want to
// parse time or duration fields as `time.Duration` and then to milliseconds
func (c *ConnzConnection) UnmarshalJSON(data []byte) error {
	var connection map[string]interface{}
	if err := json.Unmarshal(data, &connection); err != nil {
		return err
	}
	if val, exists := connection["cid"]; exists {
		c.Cid = fmt.Sprintf("%v", val)
	}
	if val, exists := connection["kind"]; exists {
		c.Kind = val.(string)
	}
	if val, exists := connection["type"]; exists {
		c.Type = val.(string)
	}
	if val, exists := connection["ip"]; exists {
		c.IP = val.(string)
	}
	if val, exists := connection["port"]; exists {
		c.Port = fmt.Sprintf("%v", val)
	}
	if val, exists := connection["start"]; exists {
		c.Start = parseDateString(val.(string))
	}
	if val, exists := connection["last_activity"]; exists {
		c.LastActivity = parseDateString(val.(string))
	}
	if val, exists := connection["rtt"]; exists {
		// rtt should be in seconds at most!
		if parsedVal, err := time.ParseDuration(val.(string)); err == nil {
			c.Rtt = float64(parsedVal.Microseconds())
		} else {
			Errorf("string %s could not be parsed as duration for rtt: %s", val.(string), err)
			c.Rtt = -1
		}
	}
	if val, exists := connection["uptime"]; exists {
		c.Uptime = parseDuration(val.(string))
	}
	if val, exists := connection["idle"]; exists {
		c.Idle = parseDuration(val.(string))
	}
	if val, exists := connection["pending_bytes"]; exists {
		c.PendingBytes = val.(float64)
	}
	if val, exists := connection["in_msgs"]; exists {
		c.InMsgs = val.(float64)
	}
	if val, exists := connection["out_msgs"]; exists {
		c.OutMsgs = val.(float64)
	}
	if val, exists := connection["in_bytes"]; exists {
		c.InBytes = val.(float64)
	}
	if val, exists := connection["out_bytes"]; exists {
		c.OutBytes = val.(float64)
	}
	if val, exists := connection["subscriptions"]; exists {
		c.Subscriptions = val.(float64)
	}
	if val, exists := connection["name"]; exists {
		c.Name = val.(string)
	}
	if val, exists := connection["name_tag"]; exists {
		c.NameTag = val.(string)
	}
	if val, exists := connection["account"]; exists {
		c.Account = val.(string)
	}
	if val, exists := connection["lang"]; exists {
		c.Lang = val.(string)
	}
	if val, exists := connection["version"]; exists {
		c.Version = val.(string)
	}
	if val, exists := connection["tls_version"]; exists {
		c.TLSVersion = val.(string)
	}
	if val, exists := connection["tls_cipher_suite"]; exists {
		c.TLSCipherSuite = val.(string)
	}
	return nil
}

// parse a date-time string as epoch milliseconds
func parseDateString(data string) float64 {
	theTime, err := time.Parse(time.RFC3339Nano, data)
	if err != nil {
		Errorf("could not parse value %s as a date-time object using the layout %s", data, time.RFC3339Nano)
		return -1
	}
	return float64(theTime.UnixMilli())
}

// parse the duration as epoch milliseconds
// for some reason NATS server deviated away from the allowed options
// for duration. Please see https://github.com/nats-io/nats-server/blob/main/server/monitor.go#L1309
// or (if the lines changed) check the function `server.myUptime(d time.Duration) string `
// duration can possibly have `y`, `d`, `h`, `m`, `s`
// for years 365 days is factored in NATS server
func parseDuration(data string) float64 {
	accruedHours, i := extractHoursFromYearsAndDays(data)
	if accruedHours == -1 {
		return -1
	}
	durationWithoutYearsAndDays := data[i:]
	splitByHours := strings.Split(durationWithoutYearsAndDays, "h")
	durationToParse := ""
	switch len(splitByHours) {
	case 1:
		durationToParse = fmt.Sprintf("%dh%s", accruedHours, splitByHours[0])
	case 2:
		if hours, err := strconv.Atoi(splitByHours[0]); err == nil {
			accruedHours += hours
			durationToParse = fmt.Sprintf("%dh%s", accruedHours, splitByHours[1])
		} else {
			Errorf("string %s could not be parsed as duration: %s", data, err)
			return -1
		}
	default:
		Errorf("string %s could not be parsed as duration", data)
		return -1
	}
	parsedValue, err := time.ParseDuration(durationToParse)
	if err == nil {
		return float64(parsedValue.Milliseconds())
	}
	Errorf("string %s could not be parsed as duration: %s", data, err)
	return -1
}

// extract years and days as hours from the provided string
// to be able to parse the string as duration
func extractHoursFromYearsAndDays(data string) (int, int) {
	accruedHours := 0
	valueSoFar := ""
	for i, v := range data {
		switch v {
		case 'y':
			if value, err := strconv.Atoi(valueSoFar); err == nil {
				accruedHours += value * 365 * 24
				valueSoFar = ""
			} else {
				Errorf("string %s could not be parsed as duration: %s", data, err)
				return -1, -1
			}
		case 'd':
			value, err := strconv.Atoi(valueSoFar)
			if err == nil {
				accruedHours += value * 24
				return accruedHours, i + 1
			}
			Errorf("string %s could not be parsed as duration: %s", data, err)
			return -1, -1
		case 'h', 'm', 's':
			return accruedHours, 0
		default:
			valueSoFar += string(v)
		}
	}
	return accruedHours, 0
}
