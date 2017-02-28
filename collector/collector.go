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

// Root is the base of the prometheus metrics - TODO:  Used?
type Root struct {
	Metrics map[string]PrometheusMetricConfig
}

// PrometheusMetricConfig holds configuration for the metrics.
type PrometheusMetricConfig struct {
	Help       string `json:"help"`
	MetricType string `json:"type"`
}

// CollectedServer is a NATS server polled by this collector
type CollectedServer struct {
	URL string
	ID  string
}

//NATSCollector collects NATS metrics
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
func getMetricURL(httpClient *http.Client, URL string) (response map[string]interface{}, err error) {
	resp, err := httpClient.Get(URL)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// now parse the body into json
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response, err
}

// Describe the metric to the Prometheus server.
func (nc *NATSCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.Lock()
	defer nc.Unlock()

	// for each stat in nc.Stats
	for _, k := range nc.Stats {
		switch m := k.(type) {

		// is it a Gauge
		// or Counter
		// Describe it to the channel.
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
		var err error
		resps[u.ID], err = getMetricURL(nc.httpClient, u.URL)
		if err != nil {
			Debugf("ignoring server %s: %v", u.ID, err)
			delete(resps, u.ID)
		}
	}
	return resps
}

// Collect all metrics for all URLs to send to Prometheus.
func (nc *NATSCollector) Collect(ch chan<- prometheus.Metric) {
	nc.Lock()
	defer nc.Unlock()

	resps := nc.makeRequests()

	// for each stat, see if each response contains that stat. then collect.
	for idx, k := range nc.Stats {
		switch m := k.(type) {
		case *prometheus.GaugeVec:
			for id, response := range resps {
				switch v := response[idx].(type) {
				case float64: // not sure why, but all my json numbers are coming here.
					m.WithLabelValues(id).Set(v)
				default:
					Debugf("value no longer a float", id, v)
				}
			}
			m.Collect(ch) // update the stat.
		case *prometheus.CounterVec:
			for id, response := range resps {
				switch v := response[idx].(type) {
				case float64: // not sure why, but all my json numbers are coming here.
					m.WithLabelValues(id).Add(v)
				default:
					Debugf("value no longer a float", id, v)
				}
			}
			m.Collect(ch) // update the stat.
		default:
			Tracef("Unknown Metric Type %s", k)
		}
	}
}

// loadMetricConfigFromResponse builds the configuration
// For each NATS Metrics endpoint (/*z) get the first URL
// to determine the list of possible metrics.
// TODO: flatten embedded maps.
func (nc *NATSCollector) initMetricsFromServers() {
	var response map[string]interface{}
	var err error

	nc.Stats = make(map[string]interface{})

	// gets URLs until one responds.
	for _, v := range nc.servers {
		Tracef("Initializing metrics collection from: %s", v.URL)
		response, err = getMetricURL(nc.httpClient, v.URL)
		if err != nil {
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
			case float64: // not sure why, but all my json numbers are coming here.
				nc.Stats[k] = newPrometheusGaugeVec(nc.endpoint, k, "")
			case string:
				// do nothing
			default:
				// i isn't one of the types above
				Tracef("Unknown type:  %v, %v", k, v)
			}
		}
	}
}

// NewCollector creates a new NATS Collector from a list of monitoring URLs.
// Each URL should be to a specific endpoint (e.g. varz, connz, subsz, or routez)
func NewCollector(endpoint string, servers []*CollectedServer) *NATSCollector {
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
