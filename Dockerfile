# https://hub.docker.com/_/golang/
FROM golang:1.8-alpine

WORKDIR /go/src/app

COPY prometheus_nats_exporter.go .

RUN apk add --no-cache git \
  && go-wrapper download \
  && go-wrapper download github.com/nats-io/go-nats \
  && go-wrapper download github.com/nats-io/gnatsd \
  && go-wrapper download github.com/mattn/goveralls \
  && go-wrapper download github.com/wadey/gocovmerge \
  && go-wrapper download github.com/alecthomas/gometalinter

RUN go-wrapper install

# Expose Prometheus port to listen on
EXPOSE 7777

ENTRYPOINT ["go-wrapper", "run"]
