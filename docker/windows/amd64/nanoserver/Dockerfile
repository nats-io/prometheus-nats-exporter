# Golang binary building stage
FROM golang:1.9.2

# download the source
WORKDIR /go/src/github.com/nats-io/prometheus-nats-exporter
RUN git clone https://github.com/nats-io/prometheus-nats-exporter.git .

# build 
RUN go build -v -a -tags netgo -installsuffix netgo

# Final docker image building stage
FROM microsoft/nanoserver
COPY --from=0 /go/src/github.com/nats-io/prometheus-nats-exporter/prometheus-nats-exporter.exe /prometheus-nats-exporter.exe
EXPOSE 7777
ENTRYPOINT ["/prometheus-nats-exporter.exe"]
CMD ["--help"]
