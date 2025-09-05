// Copyright 2017-2023 The NATS Authors
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

// prometheus-nats-exporter
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/prometheus-nats-exporter/collector"
	"github.com/nats-io/prometheus-nats-exporter/exporter"
)

var (
	version = "0.0.0"
)

// parseServerIDAndURL parses the url argument the optional id for the server ID.
func parseServerIDAndURL(urlArg string) (string, string, error) {
	var id string
	var monURL string

	// if there is an optional tag, parse it out and check the url
	if strings.Contains(urlArg, ",") {
		idx := strings.LastIndex(urlArg, ",")
		id = urlArg[:idx]
		monURL = urlArg[idx+1:]
		if _, err := url.ParseRequestURI(monURL); err != nil {
			return "", "", err
		}
	} else {
		// The URL is the basis for a default id with credentials stripped out.
		u, err := url.ParseRequestURI(urlArg)
		if err != nil {
			return "", "", err
		}
		id = fmt.Sprintf("%s://%s", u.Scheme, u.Host)
		monURL = urlArg
	}
	return id, monURL, nil
}

// updateOptions sets up additional options based on the provided flags.
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

	metricsSpecified := opts.GetConnz || opts.GetVarz || opts.GetSubz || opts.GetHealthz ||
		opts.GetHealthzJsEnabledOnly || opts.GetHealthzJsServerOnly ||
		opts.GetRoutez || opts.GetGatewayz || opts.GetAccstatz || opts.GetAccountz || opts.GetLeafz ||
		opts.GetJszFilter == ""
	if !metricsSpecified {
		// No logger setup yet, so use fmt
		fmt.Printf("No metrics specified.  Defaulting to varz.\n")
		opts.GetVarz = true
	}
}

func main() {
	var useSysLog bool
	var debugAndTrace bool
	var retryInterval int
	var printVersion bool

	// Setup the interrupt handler to gracefully exit.
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	opts := exporter.GetDefaultExporterOptions()

	// Parse flags
	flag.BoolVar(&printVersion, "version", false, "Show exporter version and exit.")
	flag.IntVar(&opts.ListenPort, "port", exporter.DefaultListenPort, "Port to listen on.")
	flag.IntVar(&opts.ListenPort, "p", exporter.DefaultListenPort, "Port to listen on.")
	flag.StringVar(&opts.ListenAddress, "addr", exporter.DefaultListenAddress, "Network host to listen on.")
	flag.StringVar(&opts.ListenAddress, "a", exporter.DefaultListenAddress, "Network host to listen on.")
	flag.StringVar(&opts.ScrapePath, "path", exporter.DefaultScrapePath, "URL path from which to serve scrapes.")
	flag.IntVar(&retryInterval, "ri", exporter.DefaultRetryIntervalSecs,
		"Interval in seconds to retry NATS Server monitor URL.")
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
	flag.BoolVar(&opts.GetConnzDetailed, "connz_detailed", false,
		"Get detailed connection metrics for each client. Enables flag `connz` implicitly.")
	flag.BoolVar(&opts.GetHealthz, "healthz", false, "Get health metrics.")
	flag.BoolVar(&opts.GetHealthzJsEnabledOnly, "healthz_js_enabled_only", false,
		"Get health metrics with js-enabled-only=true.")
	flag.BoolVar(&opts.GetHealthzJsServerOnly, "healthz_js_server_only", false,
		"Get health metrics with js-server-only=true.")
	flag.BoolVar(&opts.GetGatewayz, "gatewayz", false, "Get gateway metrics.")
	flag.BoolVar(&opts.GetAccstatz, "accstatz", false, "Get accstatz metrics.")
	flag.BoolVar(&opts.GetAccountz, "accountz", false, "Get account details metrics.")
	flag.BoolVar(&opts.GetLeafz, "leafz", false, "Get leaf metrics.")
	flag.BoolVar(&opts.GetRoutez, "routez", false, "Get route metrics.")
	flag.BoolVar(&opts.GetSubz, "subz", false, "Get subscription metrics.")
	flag.BoolVar(&opts.GetVarz, "varz", false, "Get general metrics.")
	flag.StringVar(&opts.GetJszFilter, "jsz", "", "Select JetStream metrics to filter (e.g streams, accounts, consumers)")
	flag.StringVar(&opts.CertFile, "tlscert", "", "Server certificate file (Enables HTTPS).")
	flag.StringVar(&opts.KeyFile, "tlskey", "", "Private key for server certificate (used with HTTPS).")
	flag.StringVar(&opts.CaFile, "tlscacert", "", "Client certificate CA for verification (used with HTTPS).")
	flag.StringVar(&opts.HTTPUser, "http_user", "", "Enable basic auth and set user name for HTTP scrapes.")
	flag.StringVar(&opts.HTTPPassword, "http_pass", "", "Set the password for HTTP scrapes. NATS bcrypt supported.")
	flag.StringVar(&opts.Prefix, "prefix", "", "Replace the default prefix for all the metrics.")
	flag.BoolVar(&opts.UseInternalServerID, "use_internal_server_id", false, "Enables using ServerID from /varz")
	flag.BoolVar(&opts.UseServerName, "use_internal_server_name", false, "Enables using ServerName from /varz")
	flag.Parse()

	opts.RetryInterval = time.Duration(retryInterval) * time.Second

	if printVersion {
		fmt.Println("prometheus-nats-exporter version", version)
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) < 1 {
		fmt.Printf("Usage:  %s <flags> url\n\n", os.Args[0])
		flag.Usage()
		return
	} else if len(args) > 1 {
		fmt.Println(
			`WARNING:  While permitted by this exporter, monitoring more than one server
violates Prometheus guidelines and best practices.  Each Prometheus NATS
exporter should monitor exactly one NATS server, preferably sitting right
beside it on the same machine.  Aggregate multiple servers only when
necessary.`)
	}

	updateOptions(debugAndTrace, useSysLog, opts)

	// Create an instance of the NATS exporter.
	exp := exporter.NewExporter(opts)

	switch {
	case len(args) == 1 && opts.UseInternalServerID:
		// Pick the server id from the /varz endpoint info.
		url := flag.Args()[0]
		id := collector.GetServerIDFromVarz(url, opts.RetryInterval)
		if err := exp.AddServer(id, url); err != nil {
			collector.Fatalf("Unable to setup server in exporter: %s, %s: %v", id, url, err)
		}

	case len(args) == 1 && opts.UseServerName:
		// Pick the server name from the /varz endpoint info.
		url := flag.Args()[0]
		id := collector.GetServerNameFromVarz(url, opts.RetryInterval)
		if err := exp.AddServer(id, url); err != nil {
			collector.Fatalf("Unable to setup server in exporter: %s, %s: %v", id, url, err)
		}

	default:
		// For each URL specified, add the NATS server with the optional ID.
		for _, arg := range args {
			// This should make the http request to get the server id
			// that is returned from /varz.
			id, url, err := parseServerIDAndURL(arg)
			if err != nil {
				collector.Fatalf("Unable to parse URL %q: %v", arg, err)
			}
			if err := exp.AddServer(id, url); err != nil {
				collector.Fatalf("Unable to setup server in exporter: %s, %s: %v",
					id, url, err)
			}
		}
	}

	// Start the exporter.
	if err := exp.Start(); err != nil {
		collector.Fatalf("error starting the exporter: %v\n", err)
	}

	<-c
	exp.Stop()
}
