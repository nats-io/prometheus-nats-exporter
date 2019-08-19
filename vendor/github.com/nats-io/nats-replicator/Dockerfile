FROM golang:1.12.4 AS builder

WORKDIR /src/nats-replicator

LABEL maintainer "Stephen Asbury <sasbury@nats.io>"

COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -v -a -tags netgo -installsuffix netgo -o /nats-replicator

FROM alpine:3.9

RUN mkdir -p /nats/bin && mkdir /nats/conf

COPY --from=builder /nats-replicator /nats/bin/nats-replicator

RUN ln -ns /nats/bin/nats-replicator /bin/nats-replicator

ENTRYPOINT ["/bin/nats-replicator"]
