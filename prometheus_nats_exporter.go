package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"

	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/nats-io/prometheus-nats-exporter/exporter"
)

func getNameAndURL(urlArg string) (string, string) {
	name := urlArg
	url := urlArg
	if strings.Contains(urlArg, ",") {
		idx := strings.LastIndex(urlArg, ",")
		name = urlArg[:idx]
		url = urlArg[idx+1:]
	}
	return name, url
}

// updateOptions sets up the options based on the provided flags.
func updateOptions(debugAndTrace, useSysLog bool, opts *exporter.NATSExporterOptions) {
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

	if !opts.GetConnz && !opts.GetVarz && !opts.GetSubz && !opts.GetRoutez {
		// Mo logger setup yet...
		fmt.Printf("No metrics specified.  Defaulting to varz.\n")
		opts.GetVarz = true
	}
}

func main() {
	var useSysLog bool
	var debugAndTrace bool
	var retryInterval int

	opts := exporter.GetDefaultExporterOptions()

	// Parse flags
	flag.IntVar(&opts.ListenPort, "port", exporter.DefaultListenPort, "Prometheus port to listen on.")
	flag.IntVar(&opts.ListenPort, "p", exporter.DefaultListenPort, "Prometheus port to listen on.")
	flag.StringVar(&opts.ListenAddress, "addr", exporter.DefaultListenAddress, "Network host to listen on.")
	flag.StringVar(&opts.ListenAddress, "a", exporter.DefaultListenAddress, "Network host to listen on.")
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
	flag.BoolVar(&opts.GetConnz, "connz", false, "Get connection metrics.")
	flag.BoolVar(&opts.GetRoutez, "routez", false, "Get route metrics.")
	flag.BoolVar(&opts.GetSubz, "subz", false, "Get subscription metrics.")
	flag.BoolVar(&opts.GetVarz, "varz", false, "Get general metrics.")

	flag.Parse()

	opts.RetryInterval = time.Duration(retryInterval) * time.Second

	if len(flag.Args()) < 1 {
		fmt.Printf("Usage:  %s <flags> url <url url url>\n\n", os.Args[0])
		flag.Usage()
		return
	}

	updateOptions(debugAndTrace, useSysLog, opts)

	exp := exporter.NewExporter(opts)

	// for each URL in args
	for _, arg := range flag.Args() {
		name, url := getNameAndURL(arg)
		if err := exp.AddServer(name, url); err != nil {
			collector.Fatalf("Unable to setup server in exporter: %s, %s: %v", name, url, err)
		}
	}

	if err := exp.Start(); err != nil {
		collector.Fatalf("Got an error starting the exporter: %v\n", err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		exp.Stop()
		os.Exit(0)
	}()

	runtime.Goexit()
}
