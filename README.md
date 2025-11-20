[![License][License-Image]][License-Url] ![Build][Build-Status-Image] [![Coverage][Coverage-Image]][Coverage-Url]

# The Prometheus NATS Exporter

The Prometheus NATS Exporter consists of both a package and an application that
exports [NATS server](https://docs.nats.io/nats-concepts/overview) metrics
to [Prometheus](https://prometheus.io/) for monitoring.  
The exporter aggregates metrics from the NATS server server [monitoring endpoints](https://docs.nats.io/running-a-nats-service/nats_admin/monitoring#monitoring-endpoints) you choose (varz, connz, subz,
routez, healthz...) into a single Prometheus exporter endpoint.

For an alternative approach that uses a System account instead of HTTP monitoring endpoints, see also: [NATS Surveyor](https://github.com/nats-io/nats-surveyor)

# Build

``` bash
make build
```

If you want to run tests, you can do this.

```bash
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
# ./prometheus-nats-exporter
Usage:  ./prometheus-nats-exporter <flags> url
  -D    Enable debug log level.
  -DV
        Enable debug and trace log levels.
  -V    Enable trace log level.
  -a string
        Network host to listen on. (default "0.0.0.0")
  -accountz
        Get accountz metrics and limits.      
  -accstatz
        Get accstatz metrics.
  -addr string
        Network host to listen on. (default "0.0.0.0")
  -connz
        Get connection metrics.
  -connz_detailed connz
        Get detailed connection metrics for each client. Enables flag connz implicitly.
  -gatewayz
        Get gateway metrics.
  -healthz
        Get health metrics.
  -healthz_js_enabled_only
        Get health metrics with js-enabled-only=true.
  -healthz_js_server_only
        Get health metrics with js-server-only=true.
  -http_pass string
        Set the password for HTTP scrapes. NATS bcrypt supported.
  -http_user string
        Enable basic auth and set user name for HTTP scrapes.
  -jsz string
        Select JetStream metrics to filter (e.g streams, accounts, consumers)
  -l string
        Log file name.
  -leafz
        Get leaf metrics.
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
  -ri int
        Interval in seconds to retry NATS Server monitor URL. (default 30)
  -routez
        Get route metrics.
  -s    Write log statements to the syslog.
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
  -use_internal_server_name
        Enables using ServerName from /varz
  -varz
        Get general metrics.
  -version
        Show exporter version and exit.

```

### The URL parameter

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
monitoring NATS with Prometheus and Grafana.

[License-Url]: https://www.apache.org/licenses/LICENSE-2.0
[License-Image]: https://img.shields.io/badge/License-Apache2-blue.svg
[Build-Status-Image]: https://img.shields.io/github/actions/workflow/status/nats-io/prometheus-nats-exporter/coverage.yaml?branch=main
[Coverage-Url]: https://coveralls.io/github/nats-io/prometheus-nats-exporter?branch=main
[Coverage-Image]: https://coveralls.io/repos/github/nats-io/prometheus-nats-exporter/badge.svg?branch=main
