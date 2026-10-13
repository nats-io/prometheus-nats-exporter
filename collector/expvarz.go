// Copyright 2026 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package collector has various collector utilities and implementations.
package collector

import (
	"net/http"
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
)

const expvarzEndpoint = "expvarz"

func isExpvarzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == expvarzEndpoint
}

// expvarzMetric maps one runtime.MemStats field to a Prometheus metric.
type expvarzMetric struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
	value     func(*runtime.MemStats) float64
}

// expvarzCollector exports the NATS server's own Go runtime memory and GC
// statistics, read from the expvar handler the server mounts at /debug/vars.
type expvarzCollector struct {
	httpClient *http.Client
	servers    []*CollectedServer
	metrics    []expvarzMetric
}

func newExpvarzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	m := func(name, help string, vt prometheus.ValueType, value func(*runtime.MemStats) float64) expvarzMetric {
		return expvarzMetric{
			desc:      prometheus.NewDesc(prometheus.BuildFQName(system, endpoint, name), help, []string{"server_id"}, nil),
			valueType: vt,
			value:     value,
		}
	}
	nc := &expvarzCollector{
		httpClient: http.DefaultClient,
		metrics: []expvarzMetric{
			m("memstats_gc_cycles_total", "Number of completed GC cycles.",
				prometheus.CounterValue, func(ms *runtime.MemStats) float64 { return float64(ms.NumGC) }),
			m("memstats_forced_gc_cycles_total", "Number of GC cycles forced by the application calling runtime.GC.",
				prometheus.CounterValue, func(ms *runtime.MemStats) float64 { return float64(ms.NumForcedGC) }),
			m("memstats_gc_pause_seconds_total", "Cumulative time spent in GC stop-the-world pauses.",
				prometheus.CounterValue, func(ms *runtime.MemStats) float64 { return float64(ms.PauseTotalNs) / 1e9 }),
			m("memstats_alloc_bytes_total", "Cumulative bytes allocated for heap objects.",
				prometheus.CounterValue, func(ms *runtime.MemStats) float64 { return float64(ms.TotalAlloc) }),
			m("memstats_heap_alloc_bytes", "Bytes of allocated heap objects.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.HeapAlloc) }),
			m("memstats_heap_objects", "Number of allocated heap objects.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.HeapObjects) }),
			m("memstats_heap_inuse_bytes", "Bytes in in-use heap spans.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.HeapInuse) }),
			m("memstats_next_gc_bytes", "Target heap size of the next GC cycle.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.NextGC) }),
			m("memstats_sys_bytes", "Total bytes of memory obtained from the OS.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.Sys) }),
			m("memstats_last_gc_time_seconds", "Time the last GC cycle finished, in seconds since the epoch.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return float64(ms.LastGC) / 1e9 }),
			m("memstats_gc_cpu_fraction",
				"Fraction of available CPU time used by the GC since the server started. "+
					"A lifetime average that moves slowly; use rate() of the GC counters for current behavior.",
				prometheus.GaugeValue, func(ms *runtime.MemStats) float64 { return ms.GCCPUFraction }),
		},
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/debug/vars",
		}
	}

	return nc
}

func (nc *expvarzCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range nc.metrics {
		ch <- m.desc
	}
}

// Collect gathers the server's Go runtime metrics.
func (nc *expvarzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		var resp Expvarz
		if err := getMetricURL(nc.httpClient, server.URL, &resp); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}
		if resp.MemStats == nil {
			Debugf("ignoring server %s: no memstats in %s", server.ID, server.URL)
			continue
		}
		for _, m := range nc.metrics {
			ch <- prometheus.MustNewConstMetric(m.desc, m.valueType, m.value(resp.MemStats), server.ID)
		}
	}
}

// Expvarz is the part of the /debug/vars output this collector reads.
// The server publishes memstats with the standard expvar package, which
// serializes runtime.MemStats with its Go field names.
type Expvarz struct {
	MemStats *runtime.MemStats `json:"memstats"`
}
