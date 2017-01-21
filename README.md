# prometheus-nats-exporter
A [Prometheus](https://prometheus.io/) exporter for NATS metrics. Maps all numeric NATS server metrics to Prometheus for monitoring. 

Aggregates any number of metrics endpoints (varz, connz, subsz, routez) across any number of monitored NATS servers into a
single Prometheus Exporter endpoint to simplify configuration of Prometheus.

# Prerequisites
``` bash
go get github.com/prometheus/client_golang/prometheus
```

# Build
``` bash
go build
```

# Run
Start prometheus-nats-exporter pointed to the varz metrics endpoints of NATS servers 
with monitoring ports: localhost:5555 and localhost:5656
``` bash
prometheus-nats-exporter "http://localhost:5555/varz" "http://localhost:5656/varz"
```

##usage
```
prometheus-nats-exporter <flags> url <url url url>

Usage of ./prometheus-nats-exporter:
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

# Using the API
The NATS prometheus exporter can be run within another go application.  Use the exporter API as follows:
```go
	// Get the default options, and set what you need to.  The listen address and Port
	// is how prometheus can poll for collected data.
	opts = GetDefaultExporterOptions()
	opts.ListenAddress = "localhost"
	opts.ListenPort = 8888
    opts.GetVarz = true
    opts.AddServer("myserver", "http://localhost:8222")

	// create an exporter instance, ready to be launched.
	exp := NewExporter(opts)

	// start collecting data
	exp.Start()

	// when done, simply call Stop()
	exp.Stop()

	// For convenience, you can block until the exporter is stopped, using
	exp.WaitUntilDone()
```