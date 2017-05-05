[![License][License-Image]][License-Url] [![Build][Build-Status-Image]][Build-Status-Url] [![Coverage][Coverage-Image]][Coverage-Url]
# prometheus-nats-exporter
A [Prometheus](https://prometheus.io/) exporter for NATS metrics. Maps all numeric NATS server metrics to Prometheus for monitoring. 

The NATS prometheus exporter aggregates the metrics endpoints you choose (varz, connz, subsz, routez) across any number of monitored NATS servers into a
single Prometheus Exporter endpoint.

# Build
``` bash
go build
```

# Run
Start prometheus-nats-exporter pointed to the varz metrics endpoints of NATS servers 
with monitoring ports: localhost:5555 and localhost:5656
``` bash
prometheus-nats-exporter -varz "http://localhost:5555" "http://localhost:5656"
```

## Usage
```
prometheus-nats-exporter <flags> url <url url url>

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
  -syslog Write log statements to the syslog.
  -varz
        Get general metrics. 
```

# The NATS Prometheus Exporter API
The NATS prometheus exporter also provides a simple, easy to use, API that allows it to run embedded.  

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
	// Get the default options, and set what you need to.  The listen address and Port
	// is how prometheus can poll for collected data.
	opts := exporter.GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
	opts.GetVarz = true

	// create an exporter instance, ready to be launched.
	exp := exporter.NewExporter(opts)

	// Add a NATS server providing a tag and the monitoring url
	exp.AddServer("myserver", "http://localhost:8222")

	// start collecting data
	exp.Start()

	// when done, simply call Stop()
	exp.Stop()

	// For convenience, you can block until the exporter is stopped
	exp.WaitUntilDone()
```

# Monitoring Walkthrough
For further information, refer to the [walkthough](walkthrough/README.md) of monitoring NATS with 
Prometheus and Grafana.

[License-Url]: http://opensource.org/licenses/MIT
[License-Image]: https://img.shields.io/badge/License-MIT-blue.svg
[Build-Status-Url]: http://travis-ci.com/nats-io/prometheus-nats-exporter
[Build-Status-Image]: https://travis-ci.com/nats-io/prometheus-nats-exporter.svg?token=bQqsBkZfycgqwrXTwekn&branch=master
[Coverage-Url]: https://coveralls.io/r/nats-io/prometheus-nats-exporter?branch=master
[Coverage-image]: https://coveralls.io/repos/github/nats-io/prometheus-nats-exporter/badge.svg?branch=master
