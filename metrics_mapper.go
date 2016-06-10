package main

// Initializes by reading in a json file of metric definitions.
// Will map results of a metric values to the metric definitons.
// Any numeric values undefined will be mapped as a gauge.
import (
	"encoding/json"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

type Root struct {
	Metrics map[string]PrometheusMetricConfig
}

type PrometheusMetricConfig struct {
	Help       string `json:"help"`
	MetricType string `json:"type"`
}

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

	fmt.Println("Returning metric:", pollingURL, subsystem, name, help)
	return metric
}

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
		fmt.Println(k + ": " + root.Metrics[k].Help)
	}
}

// Retrieves a NATS Metrics JSON. This can be called against any monitoring URL for NATS.
func GetMetricURL(URL string) (response map[string]interface{}) {
	resp, err := http.Get(URL)
	check(err)

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	check(err)

	fmt.Println(string(body))

	// now parse the body into json
	err = json.Unmarshal(body, &response)
	check(err)

	return response
}

func LoadMetricConfigFromResponse(pollingURLs []string, response map[string]interface{}) (out map[string]*prometheus.GaugeVec) {
	// get the subsystem name.
	first := pollingURLs[0]

	subsystem := first[strings.LastIndex(first, "/")+1:]

	out = make(map[string]*prometheus.GaugeVec)

	// for each metric
	for k := range response {
		//  if it's not already defined in metricDefinitions
		_, ok := metricDefinitions[k]
		if !ok {
			i := response[k]
			switch v := i.(type) {
			case float64: // not sure why, but all my json numbers are coming here.
				fmt.Println("i is a float", k, v)
				out[k] = NewPrometheusGaugeVec(pollingURLs, subsystem, k, "")
			case string:
				fmt.Println("i is a string", k, v)
			default:
				// i isn't one of the types above
				fmt.Println("i don't know what i is", v)
			}
		}
	}
	return out
}
