package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"

	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/nats-io/prometheus-nats-exporter/exporter"
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
	var useSysLog bool
	var debugAndTrace bool
	var retryInterval int

	opts := exporter.GetDefaultExporterOptions()

	// Parse flags
	flag.IntVar(&listenPort, "port", exporter.DefaultListenPort, "Prometheus port to listen on.")
	flag.IntVar(&listenPort, "p", exporter.DefaultListenPort, "Prometheus port to listen on.")
	flag.StringVar(&listenAddress, "addr", exporter.DefaultListenAddress, "Network host to listen on.")
	flag.StringVar(&listenAddress, "a", exporter.DefaultListenAddress, "Network host to listen on.")
	flag.IntVar(&retryInterval, "ri", exporter.DefaultRetryIntervalSecs, "Interval in seconds to retry NATS Server monitor URLS.")
	flag.StringVar(&opts.LogFile, "l", "", "Log file name.")
	flag.StringVar(&opts.LogFile, "log", "", "Log file name.")
	flag.BoolVar(&useSysLog, "s", false, "Write log statements to the syslog.")
	flag.BoolVar(&useSysLog, "syslog", false, "Write log statements to the syslog.")
	flag.StringVar(&opts.RemoteSyslog, "r", "", "Remote syslog address to write log statements.")
	flag.StringVar(&opts.RemoteSyslog, "remote_syslog", "", "Write log statements to a remote syslog.")
	flag.BoolVar(&opts.Debug, "D", false, "Enable debug log level.")
	flag.BoolVar(&opts.Trace, "V", false, "Enable trace log level.")
	flag.BoolVar(&debugAndTrace, "DV", false, "Enable debug and trace log levels.")

	flag.Parse()

	opts.RetryInterval = time.Duration(retryInterval) * time.Second

	if len(flag.Args()) < 1 {
		fmt.Printf("Usage:  %s <flags> url <url url url>\n\n", os.Args[0])
		flag.Usage()
		return
	}

	if debugAndTrace {
		opts.Debug = true
		opts.Trace = true
	}

	// default is console, then handle in order of precedence:
	// remote sys log, syslog, then file.  This simplifies error handling.
	if opts.LogFile != "" {
		opts.LogType = collector.FileLogType
	}
	if useSysLog {
		opts.LogType = collector.SysLogType
	}
	if opts.RemoteSyslog != "" {
		opts.LogType = collector.RemoteSysLogType
	}

	http.Handle("/metrics", prometheus.Handler())

	// for each URL in args
	urls := make([]string, 0)
	for _, arg := range flag.Args() {
		urls = append(urls, arg)
	}

	opts.ListenAddress = listenAddress
	opts.ListenPort = listenPort
	opts.MonitorURLs = urls
	exp := exporter.NewExporter(opts)
	if err := exp.Start(); err != nil {
		collector.Fatalf("Got an error starting the exporter: %v\n", err)
	}
	//defer exp.Stop()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		exp.Stop()
		os.Exit(0)
	}()

	runtime.Goexit()
}
