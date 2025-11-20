// Copyright 2022-2025 The NATS Authors
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
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func isAccountzEndpoint(system, endpoint string) bool {
	return system == CoreSystem && endpoint == "accountz"
}

type accountzCollector struct {
	sync.Mutex

	httpClient            *http.Client
	servers               []*CollectedServer
	accountDetailsMetrics *accountDetailsMetrics
}

func newAccountzCollector(system, endpoint string, servers []*CollectedServer) prometheus.Collector {
	nc := &accountzCollector{
		httpClient:            http.DefaultClient,
		accountDetailsMetrics: newAccountDetailsMetrics(system, endpoint),
	}

	nc.servers = make([]*CollectedServer, len(servers))
	for i, s := range servers {
		nc.servers[i] = &CollectedServer{
			ID:  s.ID,
			URL: s.URL + "/accountz",
		}
	}

	return nc
}

// Describe implements prometheus.Collector.
func (nc *accountzCollector) Describe(ch chan<- *prometheus.Desc) {
	nc.accountDetailsMetrics.Describe(ch)
}

// Describe
func (adm *accountDetailsMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- adm.connections
	ch <- adm.leafnodeConnections
	ch <- adm.subscriptions
	ch <- adm.isSystem
	ch <- adm.expired
	ch <- adm.complete
	ch <- adm.jetstreamEnabled
	// Add new Limits metrics
	ch <- adm.limitSubs
	ch <- adm.limitData
	ch <- adm.limitPayload
	ch <- adm.limitImports
	ch <- adm.limitExports
	ch <- adm.limitWildcards
	ch <- adm.limitConn
	ch <- adm.limitLeaf
}

// Collect gathers the server accountz metrics in two phases:
// 1. Get list of accounts from /accountz
// 2. Get detailed info for each account from /accountz?acc=<account_id>
func (nc *accountzCollector) Collect(ch chan<- prometheus.Metric) {
	for _, server := range nc.servers {
		// Phase 1: Get list of accounts
		var accountList Accountz
		if err := getMetricURL(nc.httpClient, server.URL, &accountList); err != nil {
			Debugf("ignoring server %s: %v", server.ID, err)
			continue
		}

		// Phase 2: Get detailed info for each account
		for _, accountID := range accountList.Accounts {
			var accountDetail AccountzDetail
			accountURL := server.URL + "?acc=" + accountID
			if err := getMetricURL(nc.httpClient, accountURL, &accountDetail); err != nil {
				Debugf("ignoring account %s on server %s: %v", accountID, server.ID, err)
				continue
			}

			nc.accountDetailsMetrics.Collect(server, &accountDetail, ch)
		}
	}
}

// accountMetrics has all of the prometheus descriptors related to
// each of the accounts of the server being scraped.
type accountDetailsMetrics struct {
	connections         *prometheus.Desc
	leafnodeConnections *prometheus.Desc
	subscriptions       *prometheus.Desc
	isSystem            *prometheus.Desc
	expired             *prometheus.Desc
	complete            *prometheus.Desc
	jetstreamEnabled    *prometheus.Desc
	// New Limits metrics
	limitSubs      *prometheus.Desc
	limitData      *prometheus.Desc
	limitPayload   *prometheus.Desc
	limitImports   *prometheus.Desc
	limitExports   *prometheus.Desc
	limitWildcards *prometheus.Desc
	limitConn      *prometheus.Desc
	limitLeaf      *prometheus.Desc
}

// newAccountMetrics initializes a new instance of accountMetrics.
func newAccountDetailsMetrics(system, endpoint string) *accountDetailsMetrics {

	// Base labels that are common to all metrics
	baseLabels := []string{"server_id", "account_id", "account_name"}

	account := &accountDetailsMetrics{
		connections: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "client_connections"),
			"client_connections",
			baseLabels,
			nil),
		leafnodeConnections: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "leafnode_connections"),
			"leafnode_connections",
			baseLabels,
			nil),
		subscriptions: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "subscriptions"),
			"subscriptions",
			baseLabels,
			nil),
		isSystem: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "is_system"),
			"is_system",
			baseLabels,
			nil),
		expired: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "expired"),
			"expired",
			baseLabels,
			nil),
		complete: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "complete"),
			"complete",
			baseLabels,
			nil),
		jetstreamEnabled: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "jetstream_enabled"),
			"jetstream_enabled",
			baseLabels,
			nil),
		// New Limits metrics
		limitSubs: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_subs"),
			"limit_subs",
			baseLabels,
			nil),
		limitData: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_data"),
			"limit_data",
			baseLabels,
			nil),
		limitPayload: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_payload"),
			"limit_payload",
			baseLabels,
			nil),
		limitImports: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_imports"),
			"limit_imports",
			baseLabels,
			nil),
		limitExports: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_exports"),
			"limit_exports",
			baseLabels,
			nil),
		limitWildcards: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_wildcards"),
			"limit_wildcards",
			baseLabels,
			nil),
		limitConn: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_conn"),
			"limit_conn",
			baseLabels,
			nil),
		limitLeaf: prometheus.NewDesc(
			prometheus.BuildFQName(system, endpoint, "limit_leaf"),
			"limit_leaf",
			baseLabels,
			nil),
	}

	return account
}

// Collect collects all the metrics about an account.
func (am *accountDetailsMetrics) Collect(server *CollectedServer, acc *AccountzDetail, ch chan<- prometheus.Metric) {

	// Base labels that are common to all metrics
	baseLabels := []string{server.ID, acc.AccountDetail.AccountName, acc.AccountDetail.NameTag}

	ch <- prometheus.MustNewConstMetric(am.connections, prometheus.GaugeValue, float64(acc.AccountDetail.ClientConnections),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.leafnodeConnections, prometheus.GaugeValue, float64(acc.AccountDetail.LeafnodeConnections),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.subscriptions, prometheus.GaugeValue, float64(acc.AccountDetail.Subscriptions),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.isSystem, prometheus.GaugeValue, boolToFloat64(acc.AccountDetail.IsSystem),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.expired, prometheus.GaugeValue, boolToFloat64(acc.AccountDetail.Expired),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.complete, prometheus.GaugeValue, boolToFloat64(acc.AccountDetail.Complete),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.jetstreamEnabled, prometheus.GaugeValue, boolToFloat64(acc.AccountDetail.JetstreamEnabled),
		baseLabels...)

	// Collect new Limits metrics
	ch <- prometheus.MustNewConstMetric(am.limitSubs, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Subs),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitData, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Data),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitPayload, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Payload),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitImports, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Imports),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitExports, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Exports),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitWildcards, prometheus.GaugeValue, boolToFloat64(acc.AccountDetail.DecodedJWT.NATS.Limits.Wildcards),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitConn, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Conn),
		baseLabels...)

	ch <- prometheus.MustNewConstMetric(am.limitLeaf, prometheus.GaugeValue, float64(acc.AccountDetail.DecodedJWT.NATS.Limits.Leaf),
		baseLabels...)
}

// Helper function to convert boolean to float64
func boolToFloat64(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

// Accountz represents the list of accounts returned by /accountz
type Accountz struct {
	Accounts []string `json:"accounts"`
}

// AccountzDetail represents the detailed account information returned by /accountz?acc=<account_id>
type AccountzDetail struct {
	ServerID      string        `json:"server_id"`
	AccountDetail AccountDetail `json:"account_detail"`
}

// AccountDetail contains the detailed account information
type AccountDetail struct {
	AccountName         string     `json:"account_name"`
	UpdateTime          time.Time  `json:"update_time"`
	IsSystem            bool       `json:"is_system"`
	Expired             bool       `json:"expired"`
	Complete            bool       `json:"complete"`
	JetstreamEnabled    bool       `json:"jetstream_enabled"`
	LeafnodeConnections int        `json:"leafnode_connections"`
	ClientConnections   int        `json:"client_connections"`
	Subscriptions       int        `json:"subscriptions"`
	NameTag             string     `json:"name_tag"`
	DecodedJWT          DecodedJWT `json:"decoded_jwt"`
}

// DecodedJWT contains the decoded JWT information
type DecodedJWT struct {
	NATS NATS `json:"nats"`
}

// NATS contains the NATS-specific configuration
type NATS struct {
	Limits Limits `json:"limits"`
}

// Limits contains the account limits
type Limits struct {
	Subs      int  `json:"subs"`
	Data      int  `json:"data"`
	Payload   int  `json:"payload"`
	Imports   int  `json:"imports"`
	Exports   int  `json:"exports"`
	Wildcards bool `json:"wildcards"`
	Conn      int  `json:"conn"`
	Leaf      int  `json:"leaf"`
}
