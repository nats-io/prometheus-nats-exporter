package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestMapVarz(t *testing.T) {
	t.Log("Testing Map Varz")
	loadMetricConfig("varz", "config/varz.prom.json")
	// Given a URL.
	// See if there is a config file to map.
	// Map help text, label and metric type.
	// register metric.
	varz_dat, err := ioutil.ReadFile("test/varz.sample.json")
	check(err)
	fmt.Print(string(varz_dat))

	var varz_json map[string]interface{}

	err = json.Unmarshal(varz_dat, &varz_json)
	check(err)

	p, err := json.MarshalIndent(varz_json, "", "  ")
	os.Stdout.Write(p)
	check(err)
	polls := []string{"http://localhost:5555/varz", "http://localhost:5555/connz", "http://localhost:5555/subsz"}

	// loadMetricConfigFromResponse(polls, "varz", varz_json)

	p, err = json.MarshalIndent(GetMetricURL("http://localhost:5555/varz"), "", "  ")
	os.Stdout.Write(p)
	check(err)

	var natsMetrics = make(map[string][]string)

	for _, k := range polls {
		metricName := k[strings.LastIndex(k, "/")+1:]
		natsMetrics[metricName] = append(natsMetrics[metricName], k)
	}
	for _, urls := range natsMetrics {
		LoadMetricConfigFromResponse(urls, GetMetricURL(urls[0]))
	}
}
