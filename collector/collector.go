// Copyright 2017-2018 The NATS Authors
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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	namespace = "gnatsd"
)

// CollectedServer is a NATS server polled by this collector
type CollectedServer struct {
	URL string
	ID  string
}

// NATSCollector collects NATS metrics
type NATSCollector struct {
	sync.Mutex
	Stats      map[string]interface{}
	httpClient *http.Client
	endpoint   string
	servers    []*CollectedServer
}

// newPrometheusGaugeVec creates a custom GaugeVec
// Based on our current integration, we're going to treat all metrics as gauges.
// We are going to call the set message on the gauge when we receive an updated
// metrics pull.
func newPrometheusGaugeVec(subsystem string, name string, help string) (metric *prometheus.GaugeVec) {
	if help == "" {
		help = name
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

// GetMetricURL retrieves a NATS Metrics JSON.
// This can be called against any monitoring URL for NATS.
// On any this function will error, warn and return nil.
func getMetricURL(httpClient *http.Client, URL string, response interface{}) error {
	resp, err := httpClient.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, &response)
}

// Describe the metric to the Prometheus server.
func (nc *NATSCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.Lock()
	defer nc.Unlock()

	// for each stat in nc.Stats
	for _, k := range nc.Stats {
		switch m := k.(type) {

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

// collectStatsFromRequests collects the statistics from a set of responses
// returned by a NATS server.
func (nc *NATSCollector) collectStatsFromRequests(key string, stat interface{}, resps map[string]map[string]interface{}, ch chan<- prometheus.Metric) {
	switch m := stat.(type) {
	case *prometheus.GaugeVec:
		for id, response := range resps {
			switch v := response[key].(type) {
			case float64: // not sure why, but all my json numbers are coming here.
				m.WithLabelValues(id).Set(v)
			default:
				Debugf("value no longer a float", id, v)
			}
		}
		m.Collect(ch) // update the stat.
	case *prometheus.CounterVec:
		for id, response := range resps {
			switch v := response[key].(type) {
			case float64: // not sure why, but all my json numbers are coming here.
				m.WithLabelValues(id).Add(v)
			default:
				Debugf("value no longer a float", id, v)
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

// loadMetricConfigFromResponse builds the configuration
// For each NATS Metrics endpoint (/*z) get the first URL
// to determine the list of possible metrics.
// TODO: flatten embedded maps.
func (nc *NATSCollector) initMetricsFromServers() {
	var response map[string]interface{}

	nc.Stats = make(map[string]interface{})

	// gets URLs until one responds.
	for _, v := range nc.servers {
		Tracef("Initializing metrics collection from: %s", v.URL)
		if err := getMetricURL(nc.httpClient, v.URL, &response); err != nil {
			// if a server is not running, silently ignore it.
			if strings.Contains(err.Error(), "connection refused") {
				Debugf("Unable to connect to the NATS server: %v", err)
			} else {
				// TODO:  Do not retry for other errors?
				Errorf("Error loading metric config from response: %s", err)
			}
		} else {
			break
		}
	}

	// for each metric
	for k := range response {
		//  if it's not already defined in metricDefinitions
		_, ok := nc.Stats[k]
		if !ok {
			i := response[k]
			switch v := i.(type) {
			case float64: // all json numbers are handled here.
				nc.Stats[k] = newPrometheusGaugeVec(nc.endpoint, k, "")
			case string:
				// do nothing
			default:
				// not one of the types currently handled
				Tracef("Unknown type:  %v, %v", k, v)
			}
		}
	}
}

// NewCollector creates a new NATS Collector from a list of monitoring URLs.
// Each URL should be to a specific endpoint (e.g. varz, connz, subsz, or routez)
func NewCollector(endpoint string, servers []*CollectedServer) prometheus.Collector {
	if isStreamingEndpoint(endpoint) {
		return newStreamingCollector(endpoint, servers)
	}
	
	// TODO:  Potentially add TLS config in the transport.
	tr := &http.Transport{}
	hc := &http.Client{Transport: tr}
	nc := &NATSCollector{
		httpClient: hc,
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

	nc.initMetricsFromServers()

	return nc
}
