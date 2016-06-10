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
