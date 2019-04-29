// Copyright 2017-2018 The NATS Authors
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

package collector

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/go-nats-streaming"
	pet "github.com/nats-io/prometheus-nats-exporter/test"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// return fqName from parsing the Desc() field of a metric.
func parseDesc(desc string) string {
	// split on quotes.
	return strings.Split(desc, "\"")[1]
}

func verifyCollector(url string, endpoint string, cases map[string]float64, t *testing.T) {
	// create a new collector.
	servers := make([]*CollectedServer, 1)
	servers[0] = &CollectedServer{
		ID:  "id",
		URL: url,
	}
	coll := NewCollector(endpoint, servers)

	// now collect the metrics
	c := make(chan prometheus.Metric)
	go coll.Collect(c)
	for {
		select {
		case metric := <-c:
			pb := &dto.Metric{}
			if err := metric.Write(pb); err != nil {
				t.Fatalf("Unable to write metric: %v", err)
			}
			gauge := pb.GetGauge()
			val := gauge.GetValue()

			name := parseDesc(metric.Desc().String())
			expected, ok := cases[name]
			if ok {
				if val != expected {
					t.Fatalf("Expected %s=%v, got %v", name, expected, val)
				}
			}
		case <-time.After(10 * time.Millisecond):
			return
		}
	}
}

func getLabelPairs(url string, endpoint string, metricName string) ([]*dto.LabelPair, error) {
	// create a new collector.
	servers := make([]*CollectedServer, 1)
	servers[0] = &CollectedServer{
		ID:  "id",
		URL: url,
	}
	coll := NewCollector(endpoint, servers)

	// now collect the metrics
	c := make(chan prometheus.Metric)
	go coll.Collect(c)
	for {
		select {
		case metric := <-c:
			name := parseDesc(metric.Desc().String())
			if name == metricName {

				pb := &dto.Metric{}
				if err := metric.Write(pb); err != nil {
					return nil, err
				}
				return pb.GetLabel(), nil
			}

		case <-time.After(10 * time.Millisecond):
			return nil, errors.New("timeout")
		}
	}
}

func TestVarz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	nc := pet.CreateClientConnSubscribeAndPublish(t)
	defer nc.Close()

	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_varz_total_connections": 2,
		"gnatsd_varz_connections":       1,
		"gnatsd_varz_in_msgs":           1,
		"gnatsd_varz_out_msgs":          1,
		"gnatsd_varz_in_bytes":          5,
		"gnatsd_varz_out_bytes":         5,
		"gnatsd_varz_subscriptions":     1,
	}

	verifyCollector(url, "varz", cases, t)
}

func TestConnz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_connz_total_connections": 0,
		"gnatsd_varz_connections":        0,
	}

	verifyCollector(url, "connz", cases, t)

	// Test with connections.

	cases = map[string]float64{
		"gnatsd_connz_total_connections": 1,
		"gnatsd_varz_connections":        1,
	}
	nc := pet.CreateClientConnSubscribeAndPublish(t)
	defer nc.Close()

	verifyCollector(url, "connz", cases, t)
}

func TestNoServer(t *testing.T) {
	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)

	cases := map[string]float64{
		"gnatsd_connz_total_connections": 0,
		"gnatsd_varz_connections":        0,
	}

	verifyCollector(url, "varz", cases, t)
}

func TestRegister(t *testing.T) {
	cs := &CollectedServer{ID: "myid", URL: fmt.Sprintf("http://localhost:%d", pet.MonitorPort)}
	servers := make([]*CollectedServer, 0)
	servers = append(servers, cs)

	// check duplicates do not panic
	servers = append(servers, cs)

	NewCollector("varz", servers)

	// test idenpotency.
	nc := NewCollector("varz", servers)

	// test without a server (no error).
	if err := prometheus.Register(nc); err == nil {
		t.Fatalf("Did not get expected error.")
	}
	prometheus.Unregister(nc)

	// start a server
	s := pet.RunServer()
	defer s.Shutdown()

	// test collect with a server
	nc = NewCollector("varz", servers)
	if err := prometheus.Register(nc); err != nil {
		t.Fatalf("Got unexpected error: %v", err)
	}
	prometheus.Unregister(nc)

	// test collect with an invalid endpoint
	nc = NewCollector("GARBAGE", servers)
	if err := prometheus.Register(nc); err == nil {
		t.Fatalf("Did not get expected error.")
		defer prometheus.Unregister(nc)
	}
}

func TestAllEndpoints(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	nc := pet.CreateClientConnSubscribeAndPublish(t)
	defer nc.Close()

	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_varz_connections": 1,
	}
	verifyCollector(url, "varz", cases, t)

	cases = map[string]float64{
		"gnatsd_routez_num_routes": 0,
	}
	verifyCollector(url, "routez", cases, t)

	cases = map[string]float64{
		"gnatsd_subsz_num_subscriptions": 1,
	}
	verifyCollector(url, "subsz", cases, t)

	cases = map[string]float64{
		"gnatsd_connz_total_connections": 1,
	}
	verifyCollector(url, "connz", cases, t)
}

const (
	stanClusterName = "test-cluster"
	stanClientName  = "sample"
)

func TestStreamingVarz(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	sc, err := stan.Connect(stanClusterName, stanClientName,
		stan.NatsURL(fmt.Sprintf("nats://localhost:%d", pet.ClientPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer sc.Close()
	sub, err := sc.Subscribe("foo", func(_ *stan.Msg) {})
	if err != nil {
		t.Fatalf("Unexpected error on subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	totalMsgs := 10
	msg := []byte("hello")
	for i := 0; i < totalMsgs; i++ {
		if err := sc.Publish("foo", msg); err != nil {
			t.Fatalf("Unexpected error on publish: %v", err)
		}
	}

	cases := map[string]float64{
		"gnatsd_varz_total_connections": 4,
		"gnatsd_varz_connections":       4,
		"gnatsd_varz_in_msgs":           45,
		"gnatsd_varz_out_msgs":          44,
		"gnatsd_varz_in_bytes":          1644,
		"gnatsd_varz_out_bytes":         1599,
		"gnatsd_varz_subscriptions":     14,
	}

	verifyCollector(url, "varz", cases, t)
}

func TestStreamingMetrics(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	sc, err := stan.Connect(stanClusterName, stanClientName,
		stan.NatsURL(fmt.Sprintf("nats://localhost:%d", pet.ClientPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer sc.Close()
	sc.Subscribe("foo", func(_ *stan.Msg) {})
	if err != nil {
		t.Fatalf("Unexpected error on subscribe: %v", err)
	}

	totalMsgs := 10
	msg := []byte("hello")
	for i := 0; i < totalMsgs; i++ {
		if err := sc.Publish("foo", msg); err != nil {
			t.Fatalf("Unexpected error on publish: %v", err)
		}
	}

	cases := map[string]float64{
		"nss_chan_bytes_total":        0,
		"nss_chan_msgs_total":         0,
		"nss_chan_last_seq":           10,
		"nss_chan_subs_last_sent":     10,
		"nss_chan_subs_pending_count": 0,
		"nss_chan_subs_max_inflight":  1024,
	}

	verifyCollector(url, "channelsz", cases, t)

	cases = map[string]float64{
		"nss_server_bytes_total":   0,
		"nss_server_msgs_total":    0,
		"nss_server_channels":      0,
		"nss_server_subscriptions": 0,
		"nss_server_clients":       0,
		"nss_server_info":          1,
	}

	verifyCollector(url, "serverz", cases, t)
}

func TestStreamingServerInfoMetricLabels(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	labelPairs, err := getLabelPairs(url, "serverz", "nss_server_info")
	if err != nil {
		t.Fatalf("Unexpected error getting labels for nss_server_info metric: %v", err)
	}

	labelsFoundMap := map[string]bool{
		"cluster_id": false,
		"server_id":  false,
		"version":    false,
		"go_version": false,
		"state":      false,
		"role":       false,
		"start_time": false,
	}
	for _, labelPair := range labelPairs {
		labelsFoundMap[*labelPair.Name] = true
	}

	labelsMissingList := make([]string, 0)
	for label, found := range labelsFoundMap {
		if !found {
			labelsMissingList = append(labelsMissingList, label)
		}
	}

	if len(labelsMissingList) > 0 {
		t.Fatalf("The following labels were missing: %v", labelsMissingList)
	}
}
