# Prometheus Export support for NATS Streaming

Since 0.2.0 release of the NATS Prometheus Exporter, it is possible to
have the exporter poll metrics from the NATS Streaming Server
monitoring port:

```sh
$ docker run synadia/prometheus-nats-exporter:0.2.0 -h
...
  -channelz
    	Get streaming channel metrics.
  -serverz
    	Get streaming server metrics.
...
```

Once enabled, the exporter will make available the following metrics:

```sh
# Per Channel metrics
nss_chan_bytes_total
nss_chan_last_seq
nss_chan_msgs_total
nss_chan_subs_last_sent
nss_chan_subs_max_inflight
nss_chan_subs_pending_count

# Server Totals
nss_server_bytes_total
nss_server_channels
nss_server_clients
nss_server_msgs_total
nss_server_subscriptions
```

And example dashboard can be found [here](grafana-nss-dash.json):

<img width="2155" alt="Example" src="https://user-images.githubusercontent.com/26195/54957961-78215700-4f11-11e9-8b0b-ae013d0b066d.png">

## Monitoring msgs/sec from a channel 

```
sum(rate(nss_chan_msgs_total{channel="foo"}[5m])) by (channel) / 3
```

With this query you an find the rate of messages being delivered on
the channel `foo`. Note that in this case we are using `3` since that
is the size of the cluster.

<img width="1579" alt="msgs-per-sec" src="https://user-images.githubusercontent.com/26195/54960588-80ca5b00-4f1a-11e9-92d5-de59c81b6c63.png">

## Monitoring pending count from channel

```
sum(rate(nss_chan_msgs_total{channel="foo"}[5m])) by (channel) / 3
sum(nss_chan_subs_pending_count{channel="foo"}) by (channel) / 3
```

You could combine queries from `nss_chan_msgs_total` and
`nss_chan_subs_pending_count` to compare the rate of messages with the
pending count to detect whether processing is getting behind:

<img width="1468" alt="combination" src="https://user-images.githubusercontent.com/26195/54960992-4235a000-4f1c-11e9-8e55-47515a5d944d.png">
