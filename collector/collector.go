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
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// System Name Variables
var (
	// use gnatsd for backward compatibility. Changing would require users to
	// change their dashboards or other applications that rely on the
	// prometheus metric names.
	CoreSystem       = "gnatsd"
	StreamingSystem  = "nss"
	ReplicatorSystem = "replicator"
	JetStreamSystem  = "jetstream"
)

// CollectedServer is a NATS server polled by this collector
type CollectedServer struct {
	URL string
	ID  string
}

type metric struct {
	path   []string
	metric interface{}
}

// NATSCollector collects NATS metrics
type NATSCollector struct {
	sync.Mutex
	Stats      map[string]metric
	httpClient *http.Client
	endpoint   string
	system     string
	servers    []*CollectedServer
}

// newPrometheusGaugeVec creates a custom GaugeVec
// Based on our current integration, we're going to treat all metrics as gauges.
// We are going to call the set message on the gauge when we receive an updated
// metrics pull.
func newPrometheusGaugeVec(system, subsystem, name, help, prefix string) (metric *prometheus.GaugeVec) {
	if help == "" {
		help = name
	}
	namespace := system
	if prefix != "" {
		namespace = prefix
	}
	opts := prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}
	metric = prometheus.NewGaugeVec(opts, []string{"server_id"})

	Tracef("Created metric: %s, %s, %s, %s", namespace, subsystem, name, help)
	return metric
}

// newLabelGauge creates a dummy gauge whose value should always be 1. This
// gauge is useful to report static information like a version.
func newLabelGauge(system, subsystem, name, help, prefix, label string) *prometheus.GaugeVec {
	if help == "" {
		help = name
	}
	namespace := system
	if prefix != "" {
		namespace = prefix
	}
	opts := prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}
	metric := prometheus.NewGaugeVec(opts, []string{"server_id", label})

	Tracef("Created metric: %s, %s, %s, %s", namespace, subsystem, name, help)
	return metric
}

// GetMetricURL retrieves a NATS Metrics JSON.
// This can be called against any monitoring URL for NATS.
// On any this function will error, warn and return nil.
func getMetricURL(httpClient *http.Client, url string, response interface{}) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	Tracef("Retrieved metric result:\n%s\n", string(body))
	return json.Unmarshal(body, &response)
}

// GetServerIDFromVarz gets the server ID from the server.
func GetServerIDFromVarz(endpoint string, retryInterval time.Duration) string {
	return getServerKeyFromVarz(endpoint, retryInterval, "server_id")
}

// GetServerNameFromVarz gets the server name from the server.
func GetServerNameFromVarz(endpoint string, retryInterval time.Duration) string {
	return getServerKeyFromVarz(endpoint, retryInterval, "server_name")
}

func getServerKeyFromVarz(endpoint string, retryInterval time.Duration, key string) string {
	// Retry periodically until available, in case it never starts
	// then a liveness check against the NATS Server itself should
	// detect that an restart the server, in terms of the exporter
	// we just wait for it to eventually be available.
	getServerVarzValue := func() (string, error) {
		resp, err := http.DefaultClient.Get(endpoint + "/varz")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		var response map[string]interface{}
		err = json.Unmarshal(body, &response)
		if err != nil {
			return "", err
		}
		serverVarzValue, ok := response[key]
		if !ok {
			Fatalf("Could not find %s in /varz", key)
		}
		varzValue, ok := serverVarzValue.(string)
		if !ok {
			Fatalf("Invalid %s type in /varz: %+v", key, serverVarzValue)
		}

		return varzValue, nil
	}

	var varzValue string
	var err error
	varzValue, err = getServerVarzValue()
	if err == nil {
		return varzValue
	}

	for range time.NewTicker(retryInterval).C {
		varzValue, err = getServerVarzValue()
		if err != nil {
			Errorf("Could not find %s: %s", key, err)
			continue
		}
		break
	}
	return varzValue
}

// Describe the metric to the Prometheus server.
func (nc *NATSCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.Lock()
	defer nc.Unlock()

	// for each stat in nc.Stats
	for _, k := range nc.Stats {
		switch m := k.metric.(type) {

		// Describe the stat to the channel
		case *prometheus.GaugeVec:
			m.Describe(ch)
		case *prometheus.CounterVec:
			m.Describe(ch)
		default:
			Tracef("Describe: Unknown metric type: %v", k)
		}
	}
}

// makeRequests makes HTTP request to the NATS server(s) monitor URLs and returns
// a map of responses.
func (nc *NATSCollector) makeRequests() map[string]map[string]interface{} {
	// query the URL for the most recent stats.
	// get all the Metrics at once, then set the stats and collect them together.
	resps := make(map[string]map[string]interface{})
	for _, u := range nc.servers {
		var response = map[string]interface{}{}
		if err := getMetricURL(nc.httpClient, u.URL, &response); err != nil {
			Debugf("ignoring server %s: %v", u.ID, err)
			delete(resps, u.ID)
		}
		resps[u.ID] = response
	}
	return resps
}

func lookupValue(response map[string]interface{}, path []string) interface{} {
	for _, tk := range path {
		switch r := response[tk].(type) {
		case map[string]interface{}:
			response = r
		default:
			return r
		}
	}
	return nil
}

// collectStatsFromRequests collects the statistics from a set of responses
// returned by a NATS server.
func (nc *NATSCollector) collectStatsFromRequests(
	key string, stat metric, resps map[string]map[string]interface{}, ch chan<- prometheus.Metric) {
	switch m := stat.metric.(type) {
	case *prometheus.GaugeVec:
		for id, response := range resps {
			switch v := lookupValue(response, stat.path).(type) {
			case float64: // json only has floats
				m.WithLabelValues(id).Set(v)
			case string:
				m.Reset()
				m.With(prometheus.Labels{"server_id": id, "value": v}).Set(1)
			default:
				Debugf("value %s no longer a float", key, id, v)
			}
		}
		m.Collect(ch) // update the stat.
	case *prometheus.CounterVec:
		for id, response := range resps {
			switch v := lookupValue(response, stat.path).(type) {
			case float64: // json only has floats
				m.WithLabelValues(id).Add(v)
			default:
				Debugf("value %s no longer a float", key, id, v)
			}
		}
		m.Collect(ch) // update the stat.
	default:
		Tracef("Unknown Metric Type %s", key)
	}
}

// Collect all metrics for all URLs to send to Prometheus.
func (nc *NATSCollector) Collect(ch chan<- prometheus.Metric) {
	nc.Lock()
	defer nc.Unlock()

	resps := nc.makeRequests()
	if len(resps) > 0 {
		for key, stat := range nc.Stats {
			nc.collectStatsFromRequests(key, stat, resps, ch)
		}
	}
}

// initMetricsFromServers builds the configuration
// For each NATS Metrics endpoint (/*z) get the first URL
// to determine the list of possible metrics.
func (nc *NATSCollector) initMetricsFromServers(namespace string) {
	var response map[string]interface{}

	nc.Stats = make(map[string]metric)

	// gets URLs until one responds.
	for _, v := range nc.servers {
		Tracef("Initializing metrics collection from: %s", v.URL)
		if err := getMetricURL(nc.httpClient, v.URL, &response); err != nil {
			// if a server is not running, silently ignore it.

			isConnectErr := strings.Contains(err.Error(), "connection refused") ||
				strings.Contains(err.Error(), "cannot assign requested address")
			if isConnectErr {
				Debugf("Unable to connect to the NATS server: %v", err)
			} else {
				// TODO:  Do not retry for other errors?
				Errorf("Error loading metric config from response: %s", err)
			}
		} else {
			break
		}
	}

	nc.objectToMetrics(response, namespace)
}

// returns a sanitized fully qualified name and path
func fqName(name string, prefix ...string) (string, []string) {
	l := len(prefix) + 1
	path := make([]string, 0, l)
	if l > 1 {
		path = append(path, prefix...)
	}
	path = append(path, name)
	fqn := strings.Trim(strings.ReplaceAll(strings.Join(path, "_"), "/", "_"), "_")
	for strings.Contains(fqn, "__") {
		fqn = strings.ReplaceAll(fqn, "__", "_")
	}
	return fqn, path
}

func (nc *NATSCollector) objectToMetrics(response map[string]interface{}, namespace string, prefix ...string) {
	skipFQN := map[string]struct{}{
		"leaf":                    {},
		"trusted_operators_claim": {},
		"cluster_tls_timeout":     {},
		"cluster_cluster_port":    {},
		"cluster_auth_timeout":    {},
		"gateway_port":            {},
		"gateway_auth_timeout":    {},
		"gateway_tls_timeout":     {},
		"gateway_connect_retries": {},
	}
	labelKeys := map[string]struct{}{
		"server_id":   {},
		"server_name": {},
		"version":     {},
		"domain":      {},
		"leader":      {},
		"name":        {},
	}
	for k := range response {
		fqn, path := fqName(k, prefix...)
		if _, ok := skipFQN[fqn]; ok {
			continue
		}
		// if it's not already defined in metricDefinitions
		if _, ok := nc.Stats[fqn]; ok {
			continue
		}
		i := response[k]
		switch v := i.(type) {
		case float64: // all json numbers are handled here.
			nc.Stats[fqn] = metric{
				path:   path,
				metric: newPrometheusGaugeVec(nc.system, nc.endpoint, fqn, "", namespace),
			}
		case string:
			if _, ok := labelKeys[k]; !ok {
				break
			}
			nc.Stats[fqn] = metric{
				path:   path,
				metric: newLabelGauge(nc.system, nc.endpoint, fqn, "", namespace, "value"),
			}
		case map[string]interface{}:
			// recurse and flatten
			nc.objectToMetrics(v, namespace, path...)
		default:
			// not one of the types currently handled
			Tracef("Unknown type:  %v %v, %v", fqn, k, v)
		}
	}
}

func newNatsCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	// TODO:  Potentially add TLS config in the transport.
	tr := &http.Transport{}
	hc := &http.Client{Transport: tr}
	nc := &NATSCollector{
		httpClient: hc,
		system:     system,
		endpoint:   endpoint,
	}

	// create our own deep copy, and tweak the urls to be polled
	// for this type of endpoint
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/" + endpoint,
		}
	}

	nc.initMetricsFromServers(system)

	return nc
}

func getSystem(system, prefix string) string {
	if prefix == "" {
		return system
	}
	return prefix
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// NewCollector creates a new NATS Collector from a list of monitoring URLs.
// Each URL should be to a specific endpoint (e.g. varz, connz, healthz, subsz, or routez)
func NewCollector(system, endpoint, prefix string, servers []*CollectedServer) prometheus.Collector {
	if isStreamingEndpoint(system, endpoint) {
		return newStreamingCollector(getSystem(system, prefix), endpoint, servers)
	}
	if isHealthzEndpoint(system, endpoint) {
		return newHealthzCollector(getSystem(system, prefix), endpoint, servers)
	}
	if isConnzEndpoint(system, endpoint) {
		return newConnzCollector(getSystem(system, prefix), endpoint, servers)
	}
	if isGatewayzEndpoint(system, endpoint) {
		return newGatewayzCollector(getSystem(system, prefix), endpoint, servers)
	}
	if isLeafzEndpoint(system, endpoint) {
		return newLeafzCollector(getSystem(system, prefix), endpoint, servers)
	}
	if isReplicatorEndpoint(system, endpoint) {
		return newReplicatorCollector(getSystem(system, prefix), servers)
	}
	if isJszEndpoint(system) {
		return newJszCollector(getSystem(system, prefix), endpoint, servers)
	}
	return newNatsCollector(getSystem(system, prefix), endpoint, servers)
}
