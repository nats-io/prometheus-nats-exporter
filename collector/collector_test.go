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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	pet "github.com/nats-io/prometheus-nats-exporter/test"
	stan "github.com/nats-io/stan.go"
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
	coll := NewCollector(system, endpoint, "", servers, time.Microsecond)

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

func verifyStreamingCollector(url string, endpoint string, cases map[string]float64, t *testing.T) {
	// create a new collector.
	servers := make([]*CollectedServer, 1)
	servers[0] = &CollectedServer{
		ID:  "id",
		URL: url,
	}
	coll := NewCollector(StreamingSystem, endpoint, "", servers, time.Microsecond)

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
	coll := NewCollector(system, endpoint, "", servers, time.Microsecond)
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
	serverName := "My Awesome Server Name"
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
		"gnatsd_varz_subscriptions":     44,
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

	NewCollector("test", "varz", "", servers, time.Microsecond)

	// test idenpotency.
	nc := NewCollector("test", "varz", "", servers, time.Microsecond)

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
	nc = NewCollector("test", "varz", "", servers, time.Microsecond)
	if err := prometheus.Register(nc); err != nil {
		t.Fatal("Failed to register collector:", err)
	}
	if len(nc.(*NATSCollector).Stats) == 0 {
		t.Fatalf("Expected to get collector stats.")
	}
	prometheus.Unregister(nc)

	// test collect with an invalid endpoint
	nc = NewCollector("test", "GARBAGE", "", servers, time.Microsecond)
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
		"gnatsd_subsz_num_subscriptions": 44,
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
		"gnatsd_varz_total_connections": 5,
		"gnatsd_varz_connections":       5,
		"gnatsd_varz_in_msgs":           45,
		"gnatsd_varz_out_msgs":          44,
		"gnatsd_varz_in_bytes":          1594,
		"gnatsd_varz_out_bytes":         1549,
		"gnatsd_varz_subscriptions":     57,
	}

	verifyCollector(CoreSystem, url, "varz", cases, t)
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

	_, err = sc.Subscribe("foo", func(_ *stan.Msg) {})
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
		"test_chan_bytes_total":        240,
		"test_chan_msgs_total":         10,
		"test_chan_last_seq":           10,
		"test_chan_subs_last_sent":     10,
		"test_chan_subs_pending_count": 0,
		"test_chan_subs_max_inflight":  1024,
	}

	verifyCollector(StreamingSystem, url, "channelsz", cases, t)

	cases = map[string]float64{
		"test_server_bytes_total":   0,
		"test_server_bytes_in":      0,
		"test_server_bytes_out":     0,
		"test_server_msgs_total":    0,
		"test_chan_msgs_in":         0,
		"test_chan_msgs_out":        0,
		"test_server_channels":      0,
		"test_server_subscriptions": 0,
		"test_server_clients":       0,
		"test_server_info":          1,
		"test_server_active":        0,
	}

	verifyCollector(StreamingSystem, url, "serverz", cases, t)
}

func TestStreamingMetricsCustomPrefix(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	sc, err := stan.Connect(stanClusterName, stanClientName,
		stan.NatsURL(fmt.Sprintf("nats://localhost:%d", pet.ClientPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer sc.Close()

	_, err = sc.Subscribe("foo", func(_ *stan.Msg) {})
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
		"nss_chan_bytes_total":        240,
		"nss_chan_msgs_total":         10,
		"nss_chan_last_seq":           10,
		"nss_chan_subs_last_sent":     10,
		"nss_chan_subs_pending_count": 0,
		"nss_chan_subs_max_inflight":  1024,
	}

	verifyStreamingCollector(url, "channelsz", cases, t)

	cases = map[string]float64{
		"nss_server_bytes_total":   0,
		"nss_server_bytes_in":      0,
		"nss_server_bytes_out":     0,
		"nss_server_msgs_total":    0,
		"nss_chan_msgs_in":         0,
		"nss_chan_msgs_out":        0,
		"nss_server_channels":      0,
		"nss_server_subscriptions": 0,
		"nss_server_clients":       0,
		"nss_server_info":          1,
		"nss_server_active":        0,
	}

	verifyStreamingCollector(url, "serverz", cases, t)
}

func TestStreamingServerInfoMetricLabels(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	serverInfoMetric := "nss_server_info"
	labelValues, err := getLabelValues(StreamingSystem, url, "serverz", []string{serverInfoMetric})
	if err != nil {
		t.Fatalf("Unexpected error getting labels for nss_server_info metric: %v", err)
	}

	labelMaps, found := labelValues[serverInfoMetric]
	if !found || len(labelMaps) != 1 {
		t.Fatalf("No info found for metric: %v", serverInfoMetric)
	}
	labelMap := labelMaps[0]

	expectedLabelNames := []string{"cluster_id", "server_id", "version", "go_version", "state", "role", "start_time"}
	expectedLabelsNotFound := make([]string, 0)
	for _, labelName := range expectedLabelNames {
		if _, found := labelMap[labelName]; !found {
			expectedLabelsNotFound = append(expectedLabelsNotFound, labelName)
		}
	}

	if len(expectedLabelsNotFound) > 0 {
		t.Fatalf("The following expected labels were missing: %v", expectedLabelsNotFound)
	}
}

func TestStreamingSubscriptionsMetricLabels(t *testing.T) {
	s := pet.RunStreamingServer()
	defer s.Shutdown()

	queueName := "some-queue-name"
	durableSubscriptionName := "some-durable-name"
	durableGroupSubscriptionName := "some-group-durable-name"

	sc, err := stan.Connect(stanClusterName, stanClientName,
		stan.NatsURL(fmt.Sprintf("nats://localhost:%d", pet.ClientPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err = sc.Close()
		if err != nil {
			t.Fatal(err)
		}
	}()

	subscriptions := make([]stan.Subscription, 0)

	subscription, err := sc.Subscribe("foo", func(_ *stan.Msg) {})
	if err != nil {
		t.Fatalf("Unexpected error on subscribe: %v", err)
	} else {
		subscriptions = append(subscriptions, subscription)
	}
	subscription, err = sc.QueueSubscribe("bar", queueName, func(_ *stan.Msg) {},
		stan.DurableName(durableGroupSubscriptionName))
	if err != nil {
		t.Fatalf("Unexpected error on subscribe: %v", err)
	} else {
		subscriptions = append(subscriptions, subscription)
	}
	subscription, err = sc.Subscribe("baz", func(_ *stan.Msg) {},
		stan.DurableName(durableSubscriptionName))
	if err != nil {
		t.Fatalf("Unexpected error on subscribe: %v", err)
	} else {
		subscriptions = append(subscriptions, subscription)
	}
	defer func() {
		for _, subscription := range subscriptions {
			err = subscription.Unsubscribe()
			if err != nil {
				t.Fatal(err)
			}
		}
	}()

	url := fmt.Sprintf("http://localhost:%d/", pet.MonitorPort)

	streamingSunscriptionMetrics := []string{"nss_chan_subs_last_sent",
		"nss_chan_subs_pending_count", "nss_chan_subs_max_inflight"}
	labelValues, err := getLabelValues(StreamingSystem, url, "channelsz", streamingSunscriptionMetrics)
	if err != nil {
		t.Fatalf("Unexpected error getting labels for nss_server_info metric: %v", err)
	}

	for _, streamingSunscriptionMetric := range streamingSunscriptionMetrics {
		labelMaps, found := labelValues[streamingSunscriptionMetric]
		if !found || len(labelMaps) != len(subscriptions) {
			t.Fatalf("No sufficient info found for metric: %v", streamingSunscriptionMetric)
		}

		foundQueuedDurableLabels, foundDurableLabels := false, false
		expectedLabelNames := []string{"server_id", "server_role", "channel", "client_id", "inbox",
			"queue_name", "is_durable", "is_offline", "durable_name"}
		for subscriptionIndex := range subscriptions {
			expectedLabelsNotFound := make([]string, 0)
			for _, labelName := range expectedLabelNames {
				if _, found := labelMaps[subscriptionIndex][labelName]; !found {
					expectedLabelsNotFound = append(expectedLabelsNotFound, labelName)
				}
			}

			if len(expectedLabelsNotFound) > 0 {
				t.Fatalf("Streaming subscription metric %v for channel %v was missing the following expected labels %v",
					streamingSunscriptionMetric, labelMaps[subscriptionIndex]["channel"], expectedLabelsNotFound)
			}

			if labelMaps[subscriptionIndex]["queue_name"] == queueName &&
				labelMaps[subscriptionIndex]["durable_name"] == durableGroupSubscriptionName &&
				labelMaps[subscriptionIndex]["is_durable"] == "true" {
				foundQueuedDurableLabels = true
			}

			if labelMaps[subscriptionIndex]["durable_name"] == durableSubscriptionName &&
				labelMaps[subscriptionIndex]["is_durable"] == "true" {
				foundDurableLabels = true
			}
		}
		if !foundQueuedDurableLabels {
			t.Fatalf("Streaming subscription metric %v is missing expected label values "+
				"for a queued durable subscription", streamingSunscriptionMetric)
		}
		if !foundDurableLabels {
			t.Fatalf("Streaming subscription metric %v is missing expected label values "+
				"for a durable subscription", streamingSunscriptionMetric)
		}
	}
}

func TestJetStreamMetrics(t *testing.T) {
	clientPort := 4229
	monitorPort := 8229
	s := pet.RunJetStreamServerWithPorts(clientPort, monitorPort, "ABC")
	defer s.Shutdown()

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

func TestReplicatorMetrics(t *testing.T) {
	s1 := pet.RunServerWithPorts(pet.ClientPort, pet.MonitorPort)
	defer s1.Shutdown()

	s2 := pet.RunServerWithPorts(pet.ClientPort+1, pet.MonitorPort+1)
	defer s2.Shutdown()

	// Just test with NATS for this, getting protobuf errors with multiple
	// streaming servers in the same process.
	r, err := pet.RunTestReplicator(9922, pet.ClientPort, pet.ClientPort+1)
	if err != nil {
		t.Fatalf("couldn't start replicator, %s", err)
	}
	defer r.Stop()

	cases := map[string]float64{
		"replicator_connector_bytes_in":       0,
		"replicator_connector_bytes_out":      0,
		"replicator_connector_connected":      1,
		"replicator_connector_connects":       1,
		"replicator_connector_disconnects":    0,
		"replicator_connector_messages_in":    0,
		"replicator_connector_messages_out":   0,
		"replicator_connector_moving_average": 0,
		"replicator_connector_quintile_50":    -1,
		"replicator_connector_quintile_75":    -1,
		"replicator_connector_quintile_90":    -1,
		"replicator_connector_quintile_95":    -1,
		"replicator_connector_request_count":  0,
	}

	url := "http://127.0.0.1:9922"
	verifyCollector(ReplicatorSystem, url, "varz", cases, t)
}
