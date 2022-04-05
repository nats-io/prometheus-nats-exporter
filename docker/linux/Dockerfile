FROM golang:1.16 AS build
COPY . /go/src/prometheus-nats-exporter
WORKDIR /go/src/prometheus-nats-exporter
RUN make prometheus-nats-exporter.docker

FROM alpine:latest as osdeps
RUN apk add --no-cache ca-certificates

FROM scratch
COPY --from=build /go/src/prometheus-nats-exporter/prometheus-nats-exporter.docker /prometheus-nats-exporter
COPY --from=osdeps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 7777
ENTRYPOINT ["/prometheus-nats-exporter"]
CMD ["--help"]
