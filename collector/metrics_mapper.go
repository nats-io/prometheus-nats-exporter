// Copyright 2016 Apcera Inc. All rights reserved.

package collector

// Initializes by reading in a json file of metric definitions.
// Will map results of a metric values to the metric definitons.
// Any numeric values undefined will be mapped as a gauge.
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
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

// TODO:  Roll these into an object
var pLock sync.Mutex
var metricDefinitions = make(map[string]map[string]PrometheusMetricConfig)

const (
	namespace = "gnatsd"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// func createPrometheusCounter(subsystem string, name string, help string) {
//   if help == nil {
//     help = name
//   }
//   metric = prometheus.NewCounter(prometheus.CounterOpts {
//     Namespace: namespace,
//     Subsystem: subsystem,
//     Name: name,
//     Help: help
//   })
//   prometheus.MustRegister(metric)
// }

// NewPrometheusGaugeVec creates a custom GaugeVec
// Based on our current integration, we're going to treat all metrics as gauges.
// We are going to call the set message on the gauge when we receive an updated
// metrics pull.
func NewPrometheusGaugeVec(pollingURL []string, subsystem string, name string, help string) (metric *prometheus.GaugeVec) {
	if help == "" {
		help = name
	}
	opts := prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      name,
		Help:      help,
	}
	metric = prometheus.NewGaugeVec(opts, []string{"polling_url"})

	Tracef("Returning metric: %s, %s, %s, %s", pollingURL, subsystem, name, help)
	return metric
}

// TODO: Add Metric configs for help text and type updates.
func loadMetricConfig(subsystem string, configFileName string) {
	cfg, err := ioutil.ReadFile(configFileName)
	check(err)
	fmt.Print(string(cfg))

	var root Root

	err = json.Unmarshal(cfg, &root)
	check(err)

	p, err := json.MarshalIndent(root, "", "  ")
	os.Stdout.Write(p)
	check(err)

	// for each metric name
	// add to metric definitions
	for k := range root.Metrics {
		Tracef(k + ": " + root.Metrics[k].Help)
	}
}

// GetMetricURL retrieves a NATS Metrics JSON.
// This can be called against any monitoring URL for NATS.
// On any this function will error, warn and return nil.
// TODO:  This is reported as a race condition in testing.
func GetMetricURL(URL string) (response map[string]interface{}, err error) {
	resp, err := http.Get(URL)
	if err != nil {
		return response, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	// now parse the body into json
	err = json.Unmarshal(body, &response)
	if err != nil {
		return response, err
	}

	return response, err
}

// LoadMetricConfigFromResponse builds the configuration
// For each NATS Metrics endpoint (/*z) get the first URL
// to determine the list of possible metrics.
// TODO: flatten embedded maps.
func LoadMetricConfigFromResponse(pollingURLs []string) (out map[string]interface{}) {
	// get the subsystem name.
	first := pollingURLs[0]
	endpoint := first[strings.LastIndex(first, "/")+1:]
	out = make(map[string]interface{})

	var response map[string]interface{}
	var err error

	// gets URLs until one responds.
	for _, v := range pollingURLs {
		response, err = GetMetricURL(v)
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
		pLock.Lock()
		_, ok := metricDefinitions[k] // TODO: Populate Metrics Definitios
		pLock.Unlock()
		if !ok {
			i := response[k]
			switch v := i.(type) {
			case float64: // not sure why, but all my json numbers are coming here.
				out[k] = NewPrometheusGaugeVec(pollingURLs, endpoint, k, "")
			case string:
				// do nothing
			default:
				// i isn't one of the types above
				Debugf("Unknown type:  %v, %v", k, v)
			}
		}
	}
	return out
}
