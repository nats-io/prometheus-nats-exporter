
# A Walkthough of Monitoring NATS with Prometheus and Grafana

This describes the basic setup and walks through a 
NATS/Prometheus/Grafana monitoring setup.

## Prerequisites
For this walkthrough, you'll need to install the NATS Prometheus Exporter, [NATS Server](https://github.com/nats-io/gnatsd), [Prometheus](https://prometheus.io/), and [Grafana](https://grafana.com/)

### NATS Server
```bash
go get github.com/nats-io/gnatsd
```

### Prometheus
```sh
brew install prometheus
```

### Grafana
```sh
brew install grafana
```

## Launching the processes

Change directory to the walkthrough directory
```sh
cd walkthrough
```

It is helpful to setup three console windows, one for each process.

### 1) Start the NATS server 
Be sure to include the a monitoring port:
```sh
gnatsd -m 8222
```

Next, launch the NATS prometheus exporter.  Here we specified an optional tag
of `test-nats-server`, and the monitor port we specified when launching the 
NATS server.  If a NATS server is not running, the exporter will retry to connect
to the NATS server indefinitely.
```sh
prometheus-nats-exporter -varz test-nats-server,http://localhost:8222
```

You should see something like this:
```text
[21690] 2017/05/02 10:10:37.461404 [INF] Prometheus exporter listening at 0.0.0.0:7777
^CColins-MacBook-Pro:prometheus-nats-exporter colinsullivan$ ./prometheus-nats-eorter -varz test-nats-server,http://localhost:8222
[21709] 2017/05/02 10:11:26.919467 [INF] Prometheus exporter listening at 0.0.0.0:7777
```

### 2) Launch Prometheus
```sh
prometheus
```

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

For this walkthrough, we'll use the default URL, http://localhost:9090.

### 3) Start Grafana

Start your Grafana Server (__your settings may differ__).
```bash
grafana-server --config=/usr/local/etc/grafana/grafana.ini --homepath /usr/local/share/grafana cfg:default.paths.logs=/usr/local/var/log/grafana cfg:default.paths.data=/usr/local/var/lib/grafana cfg:default.paths.plugins=/usr/local/var/lib/grafana/plugins
```

*Note, your configuration and startup may vary*

Look for a log line like this to find the Grafana URL:
```text
INFO[05-02|10:36:04] Server Listening logger=server address=0.0.0.0:3000 protocol=http subUrl=
```

Connect to Grafana, using the correct host and port.

Add a Prometheus data source with the following parameters:
* Name:  NATS-Prometheus
* Type:  Prometheus
* Url:  `http://localhost:9090` (default, unless changed above)

Leave the rest as defaults.

![Data Source Image](images/GrafanaDatasource.jpg?raw=true "Grafana NATS Data Source")

Next import the NATS dashboard, `grafana-nats-dash.json` from into grafana, and associate the 
datasource previously added.  You should start seeing data graph as follows (note that
selecting a view of the last five minutes will be more exciting).

Experiment running benchmarks and applications, to see a dashboard like the one below.

![Dashboard Image](images/GrafanaDashboard.jpg?raw=true "Grafana NATS Dashboard")




