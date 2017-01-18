package collector

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

//NATSCollector collects NATS metrics
type NATSCollector struct {
	sync.Mutex
	URLs  []string
	Stats map[string]interface{}
}

// Describe the metric to the Prometheus server.
func (nc *NATSCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.Lock()
	defer nc.Unlock()

	// for each stat in nc.Stats
	for _, k := range nc.Stats {
		switch m := k.(type) {

		// is it a Gauge
		// or Counter
		// Describe it to the channel.
		case *prometheus.GaugeVec:
			m.Describe(ch)
		case *prometheus.CounterVec:
			m.Describe(ch)
		default:
			Debugf("Describe: Unknown Metric Type %s:", k)
		}
	}
}

// Collect all metrics for all URLs to send to Prometheus.
// TODO: Refactor!
func (nc *NATSCollector) Collect(ch chan<- prometheus.Metric) {
	nc.Lock()
	defer nc.Unlock()

	// query the URL for the most recent stats.
	// get all the Metrics at once, then set the stats and collect them together.
	resps := make(map[string]map[string]interface{})
	for _, u := range nc.URLs {
		var err error
		resps[u], err = GetMetricURL(u)
		if err != nil {
			Tracef("ignoring %s", u)
			delete(resps, u)
		}
	}

	// for each stat, see if each response contains that stat. then collect.
	for idx, k := range nc.Stats {
		switch m := k.(type) {
		case *prometheus.GaugeVec:
			for url, response := range resps {
				switch v := response[idx].(type) {
				case float64: // not sure why, but all my json numbers are coming here.
					m.WithLabelValues(url).Set(v)
				default:
					Debugf("value no longer a float", url, v)
				}
			}
			m.Collect(ch) // update the stat.
		case *prometheus.CounterVec:
			for url, response := range resps {
				switch v := response[idx].(type) {
				case float64: // not sure why, but all my json numbers are coming here.
					m.WithLabelValues(url).Add(v)
				default:
					Debugf("value no longer a float", url, v)
				}
			}
			m.Collect(ch) // update the stat.
		default:
			Tracef("Unknown Metric Type %s", k)
		}
	}
}

// NewCollector creates a new NATS Collector from a list of monitoring URLs.
// Each URL should be to a specific endpoint (e.g. /varz, /connz, subsz, or routez)
func NewCollector(urls []string) *NATSCollector {
	return &NATSCollector{
		URLs:  urls,
		Stats: LoadMetricConfigFromResponse(urls),
	}
}
