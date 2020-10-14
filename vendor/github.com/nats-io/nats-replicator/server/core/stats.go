/*
 * Copyright 2019 The NATS Authors
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package core

import (
	"sync"
	"time"
)

// BridgeStats wraps the current status of the bridge and all of its connectors
type BridgeStats struct {
	StartTime    int64            `json:"start_time"`
	ServerTime   int64            `json:"current_time"`
	UpTime       string           `json:"uptime"`
	RequestCount int64            `json:"request_count"`
	Connections  []ConnectorStats `json:"connectors"`
	HTTPRequests map[string]int64 `json:"http_requests"`
}

// ConnectorStats captures the statistics for a single connector
// times are in nanoseconds, use a holder to get the protection
// of a lock and to fill in the quantiles
type ConnectorStats struct {
	Name          string  `json:"name"`
	ID            string  `json:"id"`
	Connected     bool    `json:"connected"`
	Connects      int64   `json:"connects"`
	Disconnects   int64   `json:"disconnects"`
	BytesIn       int64   `json:"bytes_in"`
	BytesOut      int64   `json:"bytes_out"`
	MessagesIn    int64   `json:"msg_in"`
	MessagesOut   int64   `json:"msg_out"`
	RequestCount  int64   `json:"count"`
	MovingAverage float64 `json:"rma"`
	Quintile50    float64 `json:"q50"`
	Quintile75    float64 `json:"q75"`
	Quintile90    float64 `json:"q90"`
	Quintile95    float64 `json:"q95"`
}

// ConnectorStatsHolder provides a lock and histogram
// for a connector to updated it's stats. The holder's
// Stats() method should be used to get the current values.
type ConnectorStatsHolder struct {
	sync.Mutex
	stats     ConnectorStats
	histogram *Histogram
}

// NewConnectorStatsHolder creates an empty stats holder, and initializes the request time histogram
func NewConnectorStatsHolder(name string, id string) *ConnectorStatsHolder {
	return &ConnectorStatsHolder{
		histogram: NewHistogram(60),
		stats: ConnectorStats{
			Name: name,
			ID:   id,
		},
	}
}

// Name returns the name the holder was created with
func (stats *ConnectorStatsHolder) Name() string {
	return stats.stats.Name
}

// ID returns the ID the holder was created with
func (stats *ConnectorStatsHolder) ID() string {
	return stats.stats.ID
}

// AddMessageIn updates the messages in and bytes in fields
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddMessageIn(bytes int64) {
	stats.Lock()
	stats.stats.MessagesIn++
	stats.stats.BytesIn += bytes
	stats.Unlock()
}

// AddMessageOut updates the messages out and bytes out fields
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddMessageOut(bytes int64) {
	stats.Lock()
	stats.stats.MessagesOut++
	stats.stats.BytesOut += bytes
	stats.Unlock()
}

// AddDisconnect updates the disconnects field
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddDisconnect() {
	stats.Lock()
	stats.stats.Disconnects++
	stats.stats.Connected = false
	stats.Unlock()
}

// AddConnect updates the reconnects field
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddConnect() {
	stats.Lock()
	stats.stats.Connects++
	stats.stats.Connected = true
	stats.Unlock()
}

// AddRequestTime register a time, updating the request count, RMA and histogram
// For information on the running moving average, see https://en.wikipedia.org/wiki/Moving_average
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddRequestTime(reqTime time.Duration) {
	stats.Lock()
	reqns := float64(reqTime.Nanoseconds())
	stats.stats.RequestCount++
	stats.stats.MovingAverage = ((float64(stats.stats.RequestCount-1) * stats.stats.MovingAverage) + reqns) / float64(stats.stats.RequestCount)
	stats.histogram.Add(reqns)
	stats.Unlock()
}

// AddRequest groups addMessageIn, addMessageOut and addRequest time into a single call to reduce locking requirements.
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) AddRequest(bytesIn int64, bytesOut int64, reqTime time.Duration) {
	stats.Lock()
	stats.stats.MessagesIn++
	stats.stats.BytesIn += bytesIn
	stats.stats.MessagesOut++
	stats.stats.BytesOut += bytesOut
	reqns := float64(reqTime.Nanoseconds())
	stats.stats.RequestCount++
	stats.stats.MovingAverage = ((float64(stats.stats.RequestCount-1) * stats.stats.MovingAverage) + reqns) / float64(stats.stats.RequestCount)
	stats.histogram.Add(reqns)
	stats.Unlock()
}

// Stats updates the quantiles and returns a copy of the stats
// locks/unlocks the stats
func (stats *ConnectorStatsHolder) Stats() ConnectorStats {
	stats.Lock()
	stats.stats.Quintile50 = stats.histogram.Quantile(0.5)
	stats.stats.Quintile75 = stats.histogram.Quantile(0.75)
	stats.stats.Quintile90 = stats.histogram.Quantile(0.9)
	stats.stats.Quintile95 = stats.histogram.Quantile(0.95)
	retVal := stats.stats
	stats.Unlock()
	return retVal
}
