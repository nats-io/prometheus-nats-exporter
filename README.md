[![License][License-Image]][License-Url] [![Build][Build-Status-Image]][Build-Status-Url] [![Coverage][Coverage-Image]][Coverage-Url]

# The Prometheus NATS Exporter

The Prometheus NATS Exporter consists of both a package and an application that
exports [NATS server](http://nats.io/documentation/server/gnatsd-intro) metrics
to [Prometheus](https://prometheus.io/) for monitoring.  The exporter aggregates
metrics from the server monitoring endpoints you choose (varz, connz, subz,
routez) from a NATS server into a single Prometheus exporter endpoint.

# Build
``` bash
make build
```

If you want to run tests, you can do this.

```bash
make install-tools
make test
make lint

# If you want to see the coverage locally, then run this.
# make test-cover
```

# Run
Start the prometheus-nats-exporter executable, and poll the `varz` metrics
endpoints of the NATS server located on `localhost` configured with a monitor
port of `5555`.

``` bash
prometheus-nats-exporter -varz "http://localhost:5555"
```

To run with docker, you can use the following image:

```sh
docker run natsio/prometheus-nats-exporter:latest
```

## Usage

```bash
prometheus-nats-exporter <flags> url
  -D	Enable debug log level.
  -DV
    	Enable debug and trace log levels.
  -V	Enable trace log level.
  -a string
    	Network host to listen on. (default "0.0.0.0")
  -addr string
    	Network host to listen on. (default "0.0.0.0")
  -channelz
    	Get streaming channel metrics.
  -connz
    	Get connection metrics.
  -gatewayz
    	Get gateway metrics.
  -accstatz
      Get accstatz metrics.
  -leafz
    	Get leaf metrics.
  -http_pass string
    	Set the password for HTTP scrapes. NATS bcrypt supported.
  -http_user string
    	Enable basic auth and set user name for HTTP scrapes.
  -jsz string
    	Select JetStream metrics to filter (e.g streams, accounts, consumers, all)
  -l string
    	Log file name.
  -log string
    	Log file name.
  -p int
    	Port to listen on. (default 7777)
  -path string
    	URL path from which to serve scrapes. (default "/metrics")
  -port int
    	Port to listen on. (default 7777)
  -prefix string
    	Replace the default prefix for all the metrics.
  -r string
    	Remote syslog address to write log statements.
  -remote_syslog string
    	Write log statements to a remote syslog.
  -replicatorVarz
    	Get replicator general metrics.
  -ri int
    	Interval in seconds to retry NATS Server monitor URL. (default 30)
  -routez
    	Get route metrics.
  -s	Write log statements to the syslog.
  -serverz
    	Get streaming server metrics.
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
  -use_internal_server_id
    	Enables using ServerID from /varz
  -varz
    	Get general metrics.
  -version
    	Show exporter version and exit.
```

###  The URL parameter

The url parameter is a standard url.  Both `http` and `https` (when TLS is
configured) is supported.

e.g.
`http://denver1.foobar.com:8222`

# Monitoring

The NATS Prometheus exporter exposes metrics through an HTTP interface, and will
default to:
`http://0.0.0.0:7777/metrics`.

When `--http_user` and `--http_pass` is used, you will need to set the username
password in prometheus.  See `basic_auth` in the prometheus configuration
documentation.  If using a bcrypted password use **a very low cost** as scrapes
occur frequently.

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

The NATS prometheus exporter also provides a simple and easy to use API that
allows it to run embedded in your code.

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
For additional information, refer to the [walkthrough](walkthrough/README.md) of
monitoring NATS with Prometheus and Grafana. The NATS Prometheus Exporter can be
used to monitor NATS Streaming as well. Refer to the
[walkthrough/streaming](walkthrough/streaming.md) documentation.

[License-Url]: https://www.apache.org/licenses/LICENSE-2.0
[License-Image]: https://img.shields.io/badge/License-Apache2-blue.svg
[Build-Status-Url]: https://github.com/nats-io/prometheus-nats-exporter/actions/workflows/go.yaml
[Build-Status-Image]: https://github.com/nats-io/prometheus-nats-exporter/actions/workflows/go.yaml/badge.svg
[Coverage-Url]: https://codecov.io/gh/nats-io/prometheus-nats-exporter
[Coverage-image]: https://codecov.io/gh/nats-io/prometheus-nats-exporter/branch/master/graph/badge.svg
