[![License][License-Image]][License-Url] [![Build][Build-Status-Image]][Build-Status-Url] [![Coverage][Coverage-Image]][Coverage-Url]
# The Prometheus NATS Exporter
The Prometheus NATS Exporter consists of both a package and an application that exports [NATS server](http://nats.io/documentation/server/gnatsd-intro/) metrics to [Prometheus](https://prometheus.io/) for monitoring.  The exporter aggregates metrics from the server monitoring endpoints you choose (varz, connz, subsz, routez) from a NATS server into a single Prometheus exporter endpoint.

# Build
``` bash
go build
```

# Run
Start the prometheus-nats-exporter executable, and poll the `varz` metrics endpoints of the NATS server
located on `localhost` configured with a monitor port of `5555`.
``` bash
prometheus-nats-exporter -varz "http://localhost:5555"
```

## Usage
```bash
prometheus-nats-exporter <flags> url

Flags must include at least one of: -varz, -connz, -routez, -subz

Usage of prometheus-nats-exporter:
  -D	Enable debug log level.
  -DV   Enable debug and trace log levels.
  -V	Enable trace log level.
  -a string
    	Network host to listen on. (default "0.0.0.0")
  -addr string
    	Network host to listen on. (default "0.0.0.0")
  -connz
    	Get connection metrics.    
  -http_pass string
    	Set the password for HTTP scrapes. NATS bcrypt supported.
  -http_user string
    	Enable basic auth and set user name for HTTP scrapes.		    
  -l string
    	Log file name.
  -log string
    	Log file name.
  -p int
    	Prometheus port to listen on. (default 7777)
  -port int
    	Prometheus port to listen on. (default 7777)
  -r string
    	Remote syslog address to write log statements.
  -remote_syslog string
    	Write log statements to a remote syslog.
  -ri int
    	Interval in seconds to retry NATS Server monitor URLS. (default 30)
  -routez 
        Get route metrics.        
  -s	  Write log statements to the syslog.
  -subz 
        Get subscription metrics.
  -syslog 
        Write log statements to the syslog.
  -tlscacert string
    	Client certificate CA for verification (used with HTTPS).
  -tlscert string
    	Server certificate file (Enables HTTPS).
  -tlskey string
    	Private key for server certificate (used with HTTPS).  
  -varz
        Get general metrics. 
```

###  The URL parameter
The url parameter is a standard url.  Both `http` and `https` (when TLS is configured) is supported.

e.g.
`http://denver1.foobar.com:8222`

# Monitoring

The NATS Prometheus exporter exposes metrics through an HTTP interface, and will default to:
`http://0.0.0.0:7777/metrics`.

When `--http_user` and `--http_pass` is used, you will need to set the username password
in prometheus.  See `basic_auth` in the prometheus configuration documentation.  If using 
a bcrypted password use **a very low cost** as scrapes occur frequently.

It will return output that is readable by Prometheus.  

The returned data looks like this:
```text
# HELP gnatsd_varz_in_bytes in_bytes
# TYPE gnatsd_varz_in_bytes gauge
gnatsd_varz_in_bytes{server_id="http://localhost:8222"} 0
# HELP gnatsd_varz_in_msgs in_msgs
# TYPE gnatsd_varz_in_msgs gauge
gnatsd_varz_in_msgs{server_id="http://localhost:8222"} 0
# HELP gnatsd_varz_max_connections max_connections
# TYPE gnatsd_varz_max_connections gauge
gnatsd_varz_max_connections{server_id="http://localhost:8222"} 65536
```

# The NATS Prometheus Exporter API
The NATS prometheus exporter also provides a simple and easy to use API that allows it to run embedded in your code.  

## Import the exporter package
```go
    // import the API like this
	import (
	  "github.com/nats-io/prometheus-nats-exporter/exporter"
	)
```

## API Usage
In just a few lines of code, configure and launch an instance of the exporter.
```go 
	// Get the default options, and set what you need to.  The listen address and port
	// is how prometheus can poll for collected data.
	opts := exporter.GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true
	opts.NATSServerURL = "http://localhost:8222"

	// create an exporter instance, ready to be launched.
	exp := exporter.NewExporter(opts)

	// start collecting data
	exp.Start()

	// when done, simply call Stop()
	exp.Stop()

	// For convenience, you can block until the exporter is stopped
	exp.WaitUntilDone()
```

# Monitoring Walkthrough
For further information, refer to the [walkthough](walkthrough/README.md) of monitoring NATS with Prometheus and Grafana.

[License-Url]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/badge/License-MIT-blue.svg
[Build-Status-Url]: http://travis-ci.org/nats-io/prometheus-nats-exporter
[Build-Status-Image]: https://travis-ci.org/nats-io/prometheus-nats-exporter.svg?branch=master
[Coverage-Url]: https://coveralls.io/r/nats-io/prometheus-nats-exporter?branch=master
[Coverage-image]: https://coveralls.io/repos/github/nats-io/prometheus-nats-exporter/badge.svg?branch=master
