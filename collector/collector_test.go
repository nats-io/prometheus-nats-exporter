// Copyright 2017-2023 The NATS Authors
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
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	pet "github.com/nats-io/prometheus-nats-exporter/test"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// return fqName from parsing the Desc() field of a metric.
func parseDesc(desc string) string {
	// split on quotes.
	return strings.Split(desc, "\"")[1]
}

func verifyCollector(system, url string, endpoint string, cases map[string]float64, t *testing.T) {
	// create a new collector.
	servers := make([]*CollectedServer, 1)
	servers[0] = &CollectedServer{
		ID:  "id",
		URL: url,
	}
	coll := NewCollector(system, endpoint, "", servers)

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

// To account for the metrics that share the same descriptor but differ in their variable label values,
// return a list of lists of label pairs for each of the supplied metric names.
func getLabelValues(system, url, endpoint string, metricNames []string) (map[string][]map[string]string, error) {
	labelValues := make(map[string][]map[string]string)
	namesMap := make(map[string]bool)
	for _, metricName := range metricNames {
		namesMap[metricName] = true
	}

	metrics := make(chan prometheus.Metric)
	done := make(chan bool)
	errs := make(chan error)

	// kick off the processing goroutine
	go func() {
		for {
			metric, more := <-metrics
			if more {
				metricName := parseDesc(metric.Desc().String())
				if _, ok := namesMap[metricName]; ok {
					pb := &dto.Metric{}
					if err := metric.Write(pb); err != nil {
						errs <- err
						return
					}

					labelMaps := labelValues[metricName]

					// build a map[string]string out of the []*dto.LabelPair
					labelMap := make(map[string]string)
					for _, labelPair := range pb.GetLabel() {
						labelMap[labelPair.GetName()] = labelPair.GetValue()
					}

					labelMaps = append(labelMaps, labelMap)
					labelValues[metricName] = labelMaps
				}
			} else {
				done <- true
				return
			}
		}
	}()

	// create a new collector and collect
	servers := make([]*CollectedServer, 1)
	servers[0] = &CollectedServer{
		ID:  "id",
		URL: url,
	}
	coll := NewCollector(system, endpoint, "", servers)
	coll.Collect(metrics)
	close(metrics)

	// return after the processing goroutine is done
	select {
	case err := <-errs:
		return nil, err
	case <-done:
		return labelValues, nil
	}
}

func verifyLabels(system, url, endpoint string, expectedLabels map[string]map[string]string, t *testing.T) {
	metricNames := make([]string, 0, len(expectedLabels))
	for metricName := range expectedLabels {
		metricNames = append(metricNames, metricName)
	}

	labelValues, err := getLabelValues(system, url, endpoint, metricNames)

	if err != nil {
		t.Fatalf("Failed to get label values: %v", err)
	}

	for metricName, expectedLabelSet := range expectedLabels {
		actualLabelSets, ok := labelValues[metricName]
		if !ok {
			t.Fatalf("Metric %s not found in collected metrics", metricName)
		}

		if len(actualLabelSets) == 0 {
			t.Fatalf("No label sets found for metric %s", metricName)
		}

		// Check if any of the actual label sets contains the expected labels
		found := false
		for _, actualLabelSet := range actualLabelSets {
			matches := true
			for expectedKey, expectedValue := range expectedLabelSet {
				if actualValue, ok := actualLabelSet[expectedKey]; !ok || actualValue != expectedValue {
					matches = false
					break
				}
			}
			if matches {
				found = true
				break
			}
		}

		if !found {
			t.Fatalf(
				"Expected labels %v not found in any label set for metric %s. Actual label sets: %v",
				expectedLabelSet, metricName, actualLabelSets)
		}
	}
}

func TestServerIDFromVarz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)
	result := GetServerIDFromVarz(url, 2*time.Second)
	if len(result) < 1 || result[0] != 'N' {
		t.Fatalf("Unexpected server id: %v", result)
	}
}

func TestServerNameFromVarz(t *testing.T) {
	serverName := "nats-server"
	s := pet.RunServerWithName(serverName)
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)
	result := GetServerNameFromVarz(url, 2*time.Second)
	if result != serverName {
		t.Fatalf("Unexpected server name: %v", result)
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
		"gnatsd_varz_subscriptions":     61,
	}

	verifyCollector(CoreSystem, url, "varz", cases, t)
}

func TestStartAndConfigLoadTimeVarz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	varz, err := s.Varz(nil)
	if err != nil {
		t.Fatal(err)
	}

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	nc := pet.CreateClientConnSubscribeAndPublish(t)
	defer nc.Close()

	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_varz_start":            float64(varz.Start.UnixMilli()),
		"gnatsd_varz_config_load_time": float64(varz.ConfigLoadTime.UnixMilli()),
	}

	verifyCollector(CoreSystem, url, "varz", cases, t)
}

func TestConnz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_connz_total_connections": 0,
		"gnatsd_connz_pending_bytes":     0,
		"gnatsd_varz_connections":        0,
	}

	verifyCollector(CoreSystem, url, "connz", cases, t)

	// Test with connections.

	cases = map[string]float64{
		"gnatsd_connz_total_connections": 1,
		"gnatsd_connz_pending_bytes":     0,
		"gnatsd_varz_connections":        1,
	}
	nc := pet.CreateClientConnSubscribeAndPublish(t)
	defer nc.Close()

	verifyCollector(CoreSystem, url, "connz", cases, t)
}

func TestHealthz(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)
	// see if we get the same stats as the original monitor testing code.
	// just for our monitoring_port

	cases := map[string]float64{
		"gnatsd_healthz_status":       0,
		"gnatsd_healthz_status_value": 1,
	}

	verifyCollector(CoreSystem, url, "healthz", cases, t)

	// test after server shutdown
	s.Shutdown()

	cases = map[string]float64{
		"gnatsd_healthz_status_value": 0,
	}

	verifyCollector(CoreSystem, url, "healthz", cases, t)
}

func TestNoServer(t *testing.T) {
	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)

	cases := map[string]float64{
		"gnatsd_connz_total_connections": 0,
		"gnatsd_varz_connections":        0,
	}

	verifyCollector(CoreSystem, url, "varz", cases, t)
}

func TestRegister(t *testing.T) {
	cs := &CollectedServer{
		ID:  "myid",
		URL: fmt.Sprintf("http://localhost:%d", pet.MonitorPort),
	}
	servers := make([]*CollectedServer, 0)
	servers = append(servers, cs)

	// check duplicates do not panic
	servers = append(servers, cs)

	NewCollector("test", "varz", "", servers)

	// test idenpotency.
	nc := NewCollector("test", "varz", "", servers)

	// test without a server (no error).
	if err := prometheus.Register(nc); err != nil {
		t.Fatal("Failed to register collector:", err)
	}
	if len(nc.(*NATSCollector).Stats) > 0 {
		t.Fatal("Did not expect to get collector stats.")
	}
	prometheus.Unregister(nc)

	// start a server
	s := pet.RunServer()
	defer s.Shutdown()

	// test collect with a server
	nc = NewCollector("test", "varz", "", servers)
	if err := prometheus.Register(nc); err != nil {
		t.Fatal("Failed to register collector:", err)
	}
	if len(nc.(*NATSCollector).Stats) == 0 {
		t.Fatalf("Expected to get collector stats.")
	}
	prometheus.Unregister(nc)

	// test collect with an invalid endpoint
	nc = NewCollector("test", "GARBAGE", "", servers)
	if err := prometheus.Register(nc); err != nil {
		t.Fatal("Failed to register collector:", err)
	}
	if len(nc.(*NATSCollector).Stats) > 0 {
		t.Fatal("Did not expect to get collector stats.")
	}
	prometheus.Unregister(nc)
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
	verifyCollector(CoreSystem, url, "varz", cases, t)

	cases = map[string]float64{
		"gnatsd_routez_num_routes": 0,
	}
	verifyCollector(CoreSystem, url, "routez", cases, t)

	cases = map[string]float64{
		"gnatsd_subsz_num_subscriptions": 61,
	}
	verifyCollector(CoreSystem, url, "subsz", cases, t)

	cases = map[string]float64{
		"gnatsd_connz_total_connections": 1,
	}
	verifyCollector(CoreSystem, url, "connz", cases, t)

	cases = map[string]float64{
		"gnatsd_healthz_status": 0,
	}
	verifyCollector(CoreSystem, url, "healthz", cases, t)

	cases = map[string]float64{
		"gnatsd_accountz_expired":       0,
		"gnatsd_accountz_limit_exports": 0,
	}
	verifyCollector(CoreSystem, url, "accountz", cases, t)

}

func TestLeafzMetricLabels(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	s := pet.RunLeafzStaticServer(&wg)
	defer s.Close()

	url := fmt.Sprintf("http://localhost:%d", pet.StaticPort)

	// Test first expected label set
	expectedLabels1 := map[string]map[string]string{
		"gnatsd_leafz_conn_out_msgs": {
			"name":      "leafz_server",
			"account":   "$G",
			"ip":        "127.0.0.1",
			"port":      "6223",
			"server_id": "id",
		},
	}

	// Test second expected label set
	expectedLabels2 := map[string]map[string]string{
		"gnatsd_leafz_conn_out_msgs": {
			"name":      "",
			"account":   "$G",
			"ip":        "127.0.0.2",
			"port":      "6224",
			"server_id": "id",
		},
	}

	verifyLabels(CoreSystem, url, "leafz", expectedLabels1, t)
	verifyLabels(CoreSystem, url, "leafz", expectedLabels2, t)
}

func TestAccountzMetricLabels(t *testing.T) {
	s := pet.RunServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d", pet.MonitorPort)

	// Test first expected label set
	expectedLabels1 := map[string]map[string]string{
		"gnatsd_accountz_client_connections": {
			"account_id":   "$G",
			"account_name": "$G",
			"server_id":    "id",
		},
	}

	expectedLabels2 := map[string]map[string]string{
		"gnatsd_accountz_client_connections": {
			"account_id":   "$SYS",
			"account_name": "$SYS",
			"server_id":    "id",
		},
	}

	expectedLabels3 := map[string]map[string]string{
		"gnatsd_accountz_limit_exports": {
			"account_id":   "$SYS",
			"account_name": "$SYS",
			"server_id":    "id",
		},
	}

	expectedLabels4 := map[string]map[string]string{
		"gnatsd_accountz_subscriptions": {
			"account_id":   "$SYS",
			"account_name": "$SYS",
			"server_id":    "id",
		},
	}

	verifyLabels(CoreSystem, url, "accountz", expectedLabels1, t)
	verifyLabels(CoreSystem, url, "accountz", expectedLabels2, t)
	verifyLabels(CoreSystem, url, "accountz", expectedLabels3, t)
	verifyLabels(CoreSystem, url, "accountz", expectedLabels4, t)
}

func TestJetStreamMetrics(t *testing.T) {
	clientPort := 4229
	monitorPort := 8229
	s, err := pet.RunJetStreamServerWithPorts(clientPort, monitorPort, "ABC")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(s.StoreDir())
		s.Shutdown()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/", monitorPort)
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", clientPort))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name: "foo",
	})
	if err != nil {
		t.Fatal(err)
	}

	sub, err := js.SubscribeSync("foo", nats.Durable("my-name"))
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Unsubscribe()

	js.Publish("foo", []byte("bar1"))
	js.Publish("foo", []byte("bar2"))
	js.Publish("foo", []byte("bar3"))
	time.Sleep(5 * time.Second)

	cases := map[string]float64{
		"jetstream_server_total_streams":   1,
		"jetstream_server_total_consumers": 1,
	}
	verifyCollector(JetStreamSystem, url, "jsz", cases, t)
}

func TestJetStreamSourceMetrics(t *testing.T) {
	clientPort := 4230
	monitorPort := 8230
	s, err := pet.RunJetStreamServerWithPorts(clientPort, monitorPort, "ABC")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(s.StoreDir())
		s.Shutdown()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/", monitorPort)
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", clientPort))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}

	// Create a source stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "source-stream",
		Subjects: []string{"source.*"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a stream with sources
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "target-stream",
		Subjects: []string{"target.*"},
		Sources: []*nats.StreamSource{
			{
				Name: "source-stream",
				External: &nats.ExternalStream{
					APIPrefix:     "$JS.EXT.API",
					DeliverPrefix: "$JS.EXT.DELIVER",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Publish some messages to the source stream
	js.Publish("source.test", []byte("message1"))
	js.Publish("source.test", []byte("message2"))
	time.Sleep(2 * time.Second)

	// Test WITH external map
	expectedLabels := map[string]map[string]string{
		"jetstream_stream_source_lag": {
			"source_name":    "source-stream",
			"source_api":     "$JS.EXT.API",
			"source_deliver": "$JS.EXT.DELIVER",
			"stream_name":    "target-stream",
		},
		"jetstream_stream_source_active_duration_ns": {
			"source_name":    "source-stream",
			"source_api":     "$JS.EXT.API",
			"source_deliver": "$JS.EXT.DELIVER",
			"stream_name":    "target-stream",
		},
	}

	verifyLabels(JetStreamSystem, url, "streams", expectedLabels, t)
}

func TestJetStreamSourceMetricsWithoutExternal(t *testing.T) {
	clientPort := 4231
	monitorPort := 8231
	s, err := pet.RunJetStreamServerWithPorts(clientPort, monitorPort, "ABC")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		os.RemoveAll(s.StoreDir())
		s.Shutdown()
	}()

	url := fmt.Sprintf("http://127.0.0.1:%d/", monitorPort)
	nc, err := nats.Connect(fmt.Sprintf("nats://localhost:%d", clientPort))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}

	// Create a source stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "source-stream-no-ext",
		Subjects: []string{"source.*"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a stream with sources (no external config)
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "target-stream-no-ext",
		Subjects: []string{"target.*"},
		Sources: []*nats.StreamSource{
			{
				Name: "source-stream-no-ext",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Publish some messages to the source stream
	js.Publish("source.test", []byte("message1"))
	js.Publish("source.test", []byte("message2"))
	time.Sleep(2 * time.Second)

	// Test WITHOUT external map (empty strings for source_api and source_deliver)
	expectedLabels := map[string]map[string]string{
		"jetstream_stream_source_lag": {
			"source_name":    "source-stream-no-ext",
			"source_api":     "",
			"source_deliver": "",
			"stream_name":    "target-stream-no-ext",
		},
		"jetstream_stream_source_active_duration_ns": {
			"source_name":    "source-stream-no-ext",
			"source_api":     "",
			"source_deliver": "",
			"stream_name":    "target-stream-no-ext",
		},
	}

	verifyLabels(JetStreamSystem, url, "streams", expectedLabels, t)
}

func TestMapKeys(t *testing.T) {
	m := map[string]any{
		"foo": "bar",
		"baz": "quux",
		"nested": map[string]any{
			"foo": "bar",
			"baz": "quux",
			"nested": map[string]any{
				"foo": "bar",
				"baz": "quux",
			},
		},
	}
	expected := map[string]struct{}{
		"foo":               {},
		"baz":               {},
		"nested_foo":        {},
		"nested_baz":        {},
		"nested_nested_foo": {},
		"nested_nested_baz": {},
	}
	keys := mapKeys(m, "")
	if !maps.Equal(keys, expected) {
		t.Fatalf("expected %v, got %v", expected, keys)
	}
}
