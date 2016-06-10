# prometheus-nats-exporter
A Prometheus exporter for NATS metrics

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
