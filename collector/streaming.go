package collector

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// newStreamingCollector collects channelsz and serversz metrics of
// streaming servers.
func newStreamingCollector(endpoint string, servers []*CollectedServer) prometheus.Collector {
	switch endpoint {
	case "channelsz":
		return newChannelsCollector(servers)
	case "serverz":
		return newServerzCollector(servers)
	}
	return nil
}

func isStreamingEndpoint(endpoint string) bool {
	return endpoint == "channelsz" || endpoint == "serverz"
}

type serverzCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer

	bytesTotal *prometheus.Desc
	msgsTotal  *prometheus.Desc
	channels   *prometheus.Desc
	subs       *prometheus.Desc
	clients    *prometheus.Desc
}

func newServerzCollector(servers []*CollectedServer) prometheus.Collector {
	nc := &serverzCollector{
		httpClient: http.DefaultClient,
		bytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "server", "bytes_total"),
			"Total of bytes",
			[]string{"server"},
			nil,
		),
		msgsTotal: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "server", "msgs_total"),
			"Total of messages",
			[]string{"server"},
			nil,
		),
		channels: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "server", "channels"),
			"Total channels",
			[]string{"server"},
			nil,
		),
		subs: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "server", "subscriptions"),
			"Total subscriptions",
			[]string{"server"},
			nil,
		),
		clients: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "server", "clients"),
			"Total clients",
			[]string{"server"},
			nil,
		),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/streaming/serverz",
		}
	}

	return nc
}

func (nc *serverzCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.bytesTotal
	ch <- nc.msgsTotal
	ch <- nc.channels
	ch <- nc.subs
	ch <- nc.clients
}

// StreamingServerz represents the metrics from streaming/serverz.
type StreamingServerz struct {
	TotalBytes    int `json:"total_bytes"`
	TotalMsgs     int `json:"total_msgs"`
	Channels      int `json:"channels"`
	Subscriptions int `json:"subscriptions"`
	Clients       int `json:"clients"`
}

// Collect gathers the streaming server serverz metrics.
func (nc *serverzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp StreamingServerz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		ch <- prometheus.MustNewConstMetric(nc.bytesTotal, prometheus.CounterValue, float64(resp.TotalBytes), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.msgsTotal, prometheus.CounterValue, float64(resp.TotalMsgs), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.channels, prometheus.CounterValue, float64(resp.Channels), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.subs, prometheus.CounterValue, float64(resp.Subscriptions), server.ID)
		ch <- prometheus.MustNewConstMetric(nc.clients, prometheus.CounterValue, float64(resp.Clients), server.ID)
	}
}

type channelsCollector struct {
	sync.Mutex

	httpClient *http.Client
	servers    []*CollectedServer

	chanBytesTotal   *prometheus.Desc
	chanMsgsTotal    *prometheus.Desc
	chanLastSeq      *prometheus.Desc
	subsLastSent     *prometheus.Desc
	subsPendingCount *prometheus.Desc
	subsMaxInFlight  *prometheus.Desc
}

func newChannelsCollector(servers []*CollectedServer) prometheus.Collector {
	nc := &channelsCollector{
		httpClient: http.DefaultClient,
		chanBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "bytes_total"),
			"Total of bytes",
			[]string{"server", "channel"},
			nil,
		),
		chanMsgsTotal: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "msgs_total"),
			"Total of messages",
			[]string{"server", "channel"},
			nil,
		),
		chanLastSeq: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "last_seq"),
			"Last seq",
			[]string{"server", "channel"},
			nil,
		),
		subsLastSent: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "subs_last_sent"),
			"Last message sent",
			[]string{"server", "channel", "client_id"},
			nil,
		),
		subsPendingCount: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "subs_pending_count"),
			"Pending message count",
			[]string{"server", "channel", "client_id"},
			nil,
		),
		subsMaxInFlight: prometheus.NewDesc(
			prometheus.BuildFQName("nss", "chan", "subs_max_inflight"),
			"Max in flight message count",
			[]string{"server", "channel", "client_id"},
			nil,
		),
	}

	// create our own deep copy, and tweak the urls to be polled
	// for this type of endpoint
	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/streaming/channelsz?subs=1",
		}
	}

	return nc
}

func (nc *channelsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- nc.chanBytesTotal
	ch <- nc.chanMsgsTotal
	ch <- nc.chanLastSeq
	ch <- nc.subsLastSent
	ch <- nc.subsPendingCount
	ch <- nc.subsMaxInFlight
}

func (nc *channelsCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Channelsz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		for _, channel := range resp.Channels {
			ch <- prometheus.MustNewConstMetric(nc.chanBytesTotal, prometheus.CounterValue, float64(channel.Bytes), server.ID, channel.Name)
			ch <- prometheus.MustNewConstMetric(nc.chanMsgsTotal, prometheus.CounterValue, float64(channel.Msgs), server.ID, channel.Name)
			ch <- prometheus.MustNewConstMetric(nc.chanLastSeq, prometheus.GaugeValue, float64(channel.LastSeq), server.ID, channel.Name)

			for _, sub := range channel.Subscriptions {
				ch <- prometheus.MustNewConstMetric(nc.subsLastSent, prometheus.GaugeValue, float64(sub.LastSent), server.ID, channel.Name, sub.ClientID)
				ch <- prometheus.MustNewConstMetric(nc.subsPendingCount, prometheus.GaugeValue, float64(sub.PendingCount), server.ID, channel.Name, sub.ClientID)
				ch <- prometheus.MustNewConstMetric(nc.subsMaxInFlight, prometheus.GaugeValue, float64(sub.MaxInflight), server.ID, channel.Name, sub.ClientID)
			}
		}
	}
}

// Channelsz lists the name of all NATS Streaming Channelsz
type Channelsz struct {
	ClusterID string      `json:"cluster_id"`
	ServerID  string      `json:"server_id"`
	Now       time.Time   `json:"now"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
	Count     int         `json:"count"`
	Total     int         `json:"total"`
	Names     []string    `json:"names,omitempty"`
	Channels  []*Channelz `json:"channels,omitempty"`
}

// Channelz describes a NATS Streaming Channel
type Channelz struct {
	Name          string           `json:"name"`
	Msgs          int              `json:"msgs"`
	Bytes         uint64           `json:"bytes"`
	FirstSeq      uint64           `json:"first_seq"`
	LastSeq       uint64           `json:"last_seq"`
	Subscriptions []*Subscriptionz `json:"subscriptions,omitempty"`
}

// Subscriptionz describes a NATS Streaming Subscription
type Subscriptionz struct {
	ClientID     string `json:"client_id"`
	Inbox        string `json:"inbox"`
	AckInbox     string `json:"ack_inbox"`
	DurableName  string `json:"durable_name,omitempty"`
	QueueName    string `json:"queue_name,omitempty"`
	IsDurable    bool   `json:"is_durable"`
	IsOffline    bool   `json:"is_offline"`
	MaxInflight  int    `json:"max_inflight"`
	AckWait      int    `json:"ack_wait"`
	LastSent     uint64 `json:"last_sent"`
	PendingCount int    `json:"pending_count"`
	IsStalled    bool   `json:"is_stalled"`
}
