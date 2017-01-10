package main

import (
	"flag"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
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
	lOpts := &collector.LoggerOptions{}
	var useSysLog bool
	var debugAndTrace bool
	// Parse flags
	flag.IntVar(&listenPort, "port", 7777, "Prometheus port to listen on.")
	flag.IntVar(&listenPort, "p", 7777, "Prometheus port to listen on.")
	flag.StringVar(&listenAddress, "addr", "0.0.0.0", "Network host to listen on.")
	flag.StringVar(&listenAddress, "a", "", "Network host to listen on.")
	flag.StringVar(&lOpts.LogFile, "l", "", "")
	flag.StringVar(&lOpts.LogFile, "log", "", "")
	flag.BoolVar(&useSysLog, "s", false, "")
	flag.BoolVar(&useSysLog, "syslog", false, "")
	flag.StringVar(&lOpts.RemoteSyslog, "r", "", "")
	flag.StringVar(&lOpts.RemoteSyslog, "remote_syslog", "", "")
	flag.BoolVar(&lOpts.Debug, "D", false, "")
	flag.BoolVar(&lOpts.Debug, "debug", false, "")
	flag.BoolVar(&lOpts.Trace, "V", false, "")
	flag.BoolVar(&lOpts.Trace, "trace", false, "")
	flag.BoolVar(&debugAndTrace, "DV", false, "")

	flag.Parse()

	if debugAndTrace {
		lOpts.Debug = true
		lOpts.Trace = true
	}
	// default is console, then handle in order of precenence:
	// remote sys log, syslog, then file.  This simplifies error handling.
	// TODO: fix this
	if lOpts.LogFile != "" {
		lOpts.LogType = collector.FileLogType
	}
	if useSysLog {
		lOpts.LogType = collector.SysLogType
	}
	if lOpts.RemoteSyslog != "" {
		lOpts.LogType = collector.RemoteSysLogType
	}
	collector.ConfigureLogger(lOpts)

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
		collector.Tracef("Registering server: ", v)
		nc := collector.New(v)
		prometheus.MustRegister(nc)
	}

	collector.Noticef("NATS Prometheus exporter starting on " + listenAddress + ":" + strconv.Itoa(listenPort))
	log.Fatal(http.ListenAndServe(listenAddress+":"+strconv.Itoa(listenPort), nil))
}

func init() {
}
