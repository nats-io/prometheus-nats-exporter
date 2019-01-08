FROM golang:1.11

COPY . /go/src/github.com/nats-io/prometheus-nats-exporter
WORKDIR /go/src/github.com/nats-io/prometheus-nats-exporter

RUN go build

EXPOSE 7777
ENTRYPOINT ["./prometheus-nats-exporter"]
CMD ["--help"]
