package main

import (
	"flag"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"log"
	"net/http"
	"strconv"
	"strings"
	"github.com/nats-io/prometheus-nats-exporter/collector"
)

// scrape the endpoints listed.
// for each numeric metric
//  if we have a definition.
//  register the metric with help, and type set.
//
// for each map metric
//  append the map name and the metric to each
var (
	listenAddress string
	listenPort    int
)

func main() {
	// Parse flags
	flag.IntVar(&listenPort, "port", 7777, "Port to listen on.")
	flag.IntVar(&listenPort, "p", 7777, "Port to listen on.")
	flag.StringVar(&listenAddress, "addr", "", "Network host to listen on.")
	flag.StringVar(&listenAddress, "a", "", "Network host to listen on.")
	flag.Parse()

	// indexed.Inc()
	// size.Set(5)
	http.Handle("/metrics", prometheus.Handler())

	metrics := make(map[string][]string)

	// for each URL in args
	for _, arg := range flag.Args() {
		// create a map of possible metric endpoints.
		endpoint := arg[strings.LastIndex(arg, "/")+1:] // TODO: not safe.
		// append the url to that endpoint.
		metrics[endpoint] = append(metrics[endpoint], arg)
	}

	for _, v := range metrics {
		fmt.Println("registering", v)
		nc := collector.New(v)
		prometheus.MustRegister(nc)
	}

	prometheus.EnableCollectChecks(true)

	fmt.Println("Server starting on " + listenAddress + ":" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(listenAddress+":"+strconv.Itoa(listenPort), nil))
}

func init() {
}
