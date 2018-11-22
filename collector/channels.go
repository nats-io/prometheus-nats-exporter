package collector

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

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

// NewChannelsCollector collects channelsz metrics
func NewChannelsCollector(servers []*CollectedServer) prometheus.Collector {
	const namespace = "nss"
	const subsystem = "chan"

	nc := &channelsCollector{
		httpClient: http.DefaultClient,
		chanBytesTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "bytes_total"),
			"Total of bytes",
			[]string{"server", "channel"},
			nil,
		),
		chanMsgsTotal: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "msgs_total"),
			"Total of messages",
			[]string{"server", "channel"},
			nil,
		),
		chanLastSeq: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "last_seq"),
			"Last seq",
			[]string{"server", "channel"},
			nil,
		),
		subsLastSent: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "subs_last_sent"),
			"Last message sent",
			[]string{"server", "channel", "client_id", "inbox"},
			nil,
		),
		subsPendingCount: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "subs_pending_count"),
			"Pending message count",
			[]string{"server", "channel", "client_id", "inbox"},
			nil,
		),
		subsMaxInFlight: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "subs_max_in_flight"),
			"Max in flight message count",
			[]string{"server", "channel", "client_id", "inbox"},
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
				Debugf("collecting subscription: %v", sub)
				ch <- prometheus.MustNewConstMetric(nc.subsLastSent, prometheus.GaugeValue, float64(sub.LastSent), server.ID, channel.Name, sub.ClientID, sub.Inbox)
				ch <- prometheus.MustNewConstMetric(nc.subsPendingCount, prometheus.GaugeValue, float64(sub.PendingCount), server.ID, channel.Name, sub.ClientID, sub.Inbox)
				ch <- prometheus.MustNewConstMetric(nc.subsMaxInFlight, prometheus.GaugeValue, float64(sub.MaxInflight), server.ID, channel.Name, sub.ClientID, sub.Inbox)
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
