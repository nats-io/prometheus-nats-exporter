
# A Walkthrough of Monitoring NATS with Prometheus and Grafana

This walkthrough covers the basic setup and configuration for monitoring a
NATS server using the NATS Prometheus exporter, Prometheus, and Grafana.

## Prerequisites
For this walkthrough, you'll need to install the NATS Prometheus Exporter, [NATS Server](https://github.com/nats-io/nats-server), [Prometheus](https://prometheus.io/), and [Grafana](https://grafana.com/)

### NATS Server
```bash
go get github.com/nats-io/nats-server
```

### Prometheus

You'll need Prometheus installed; refer to the Prometheus [download](https://prometheus.io/download/) and [installation](https://prometheus.io/docs/introduction/install/) instructions.

### Grafana
For this walkthough you'll also need to [download](https://grafana.com/grafana/download) and [install](http://docs.grafana.org/#installing-grafana) Grafana.

## Launching the processes

Ensure you are working in the walkthrough directory:
```sh
cd walkthrough
```

It is helpful to start four console windows, one for each component.

### 1) Start the NATS server and NATS Prometheus Exporter
Be sure to include the a monitoring port:
```sh
nats-server -m 8222
```

Next, launch the NATS prometheus exporter.  Here we configure the same monitor port we specified when launching the NATS server.  If a NATS server is not running, the exporter will retry to connect to the NATS server indefinitely.
```sh
prometheus-nats-exporter -varz http://localhost:8222
```

You should see something like this:
```text
[21690] 2017/05/02 10:10:37.461404 [INF] Prometheus exporter listening at 0.0.0.0:7777
```

### 2) Start Prometheus
```sh
prometheus
```

Prometheus will use the configuration file located in this directory.

You'll see something like this:
```text
INFO[0000] Starting prometheus (version=1.3.1, branch=, revision=)  source=main.go:75
INFO[0000] Build context (go=go1.7.3, user=brew@yosemitevm.local, date=20161105-11:15:15)  source=main.go:76
INFO[0000] Loading configuration file prometheus.yml     source=main.go:247
INFO[0000] Loading series map and head chunks...         source=storage.go:354
INFO[0000] 0 series loaded.                              source=storage.go:359
INFO[0000] Listening on :9090                            source=web.go:240
WARN[0000] No AlertManagers configured, not dispatching any alerts  source=notifier.go:176
INFO[0000] Starting target manager...                    source=targetmanager.go:76
```

For this walkthrough we use the default URL, `http://localhost:9090`.

### 3) Start Grafana

Start your Grafana server (__your settings may differ__).
```bash
grafana-server --config=/usr/local/etc/grafana/grafana.ini --homepath /usr/local/share/grafana cfg:default.paths.logs=/usr/local/var/log/grafana cfg:default.paths.data=/usr/local/var/lib/grafana cfg:default.paths.plugins=/usr/local/var/lib/grafana/plugins
```

*Note, your configuration and startup may vary*

Look for a log line like this to find the Grafana URL:
```text
INFO[05-02|10:36:04] Server Listening logger=server address=0.0.0.0:3000 protocol=http subUrl=
```

Connect to Grafana, using the correct host and port.  The default Grafana user is `admin` and the default password is `admin`.

Add a Prometheus data source with the following parameters:
* Name:  NATS-Prometheus
* Type:  Prometheus
* Url:  `http://localhost:9090` (default, unless changed above)

Leave the rest as defaults.

![Data Source Image](images/GrafanaDatasource.jpg?raw=true "Grafana NATS Data Source")

Next import the NATS dashboard, `grafana-nats-dash.json` into Grafana, and associate the 
Prometheus datasource you just created.  You should start seeing data graph as follows (note that
selecting a view of the last five minutes will be more exciting).

Experiment running benchmarks and applications and you'll see a dashboard like the one below!

![Dashboard Image](images/GrafanaDashboard.jpg?raw=true "Grafana NATS Dashboard")

If you're using JetStream, you can import the JetStream dashboard (`grafana-jetstream-dash.json`) too:

![Dashboard Image](images/GrafanaJetStreamDashboard.png?raw=true "Grafana JetStream Dashboard")



