FROM alpine:latest as osdeps

RUN apk add --no-cache ca-certificates

ARG BINARY=prometheus-nats-exporter

FROM scratch

COPY ${BINARY} /${BINARY}

COPY --from=osdeps /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 7777

ENTRYPOINT ["/prometheus-nats-exporter"]
CMD ["--help"]
