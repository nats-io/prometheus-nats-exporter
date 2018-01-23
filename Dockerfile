# Golang binary building stage
FROM golang:1.7.6
COPY . /go/src/github.com/nats-io/prometheus-nats-exporter
WORKDIR /go/src/github.com/nats-io/prometheus-nats-exporter
RUN CGO_ENABLED=0 go build -v -a

# Final docker image building stage
FROM scratch
COPY --from=0 /go/src/github.com/nats-io/prometheus-nats-exporter/prometheus-nats-exporter /prometheus-nats-exporter
EXPOSE 7777
ENTRYPOINT ["/prometheus-nats-exporter"]
CMD ["--help"]
