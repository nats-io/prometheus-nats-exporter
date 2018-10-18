FROM scratch
COPY prometheus-nats-exporter /
EXPOSE 7777
ENTRYPOINT ["/prometheus-nats-exporter"]
CMD ["--help"]
