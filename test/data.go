// Copyright 2021-2023 The NATS Authors
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

package test

// GatewayzTestResponse is static data for tests
func GatewayzTestResponse() string {
	return `{
	"server_id": "SERVER_ID",
	"now": "2021-05-07T18:13:47.70796395Z",
	"name": "server_name",
	"host": "1.2.3.4",
	"port": 7222,
	"outbound_gateways": {
		"gwa0": {
			"configured": true,
			"connection": {
				"cid": 217,
				"ip": "8.8.8.8",
				"port": 7222,
				"start": "2021-05-07T04:09:37.199417434Z",
				"last_activity": "2021-05-07T18:13:46.638957061Z",
				"rtt": "80.711681ms",
				"uptime": "14h4m10s",
				"idle": "1s",
				"pending_bytes": 0,
				"in_msgs": 0,
				"out_msgs": 656564,
				"in_bytes": 0,
				"out_bytes": 506772014,
				"subscriptions": 10,
				"name": "nameE3",
				"tls_version": "1.3",
				"tls_cipher_suite": "TLS_AES_128_GCM_SHA256"
			}
		}
	},
	"inbound_gateways": {
		"gwa0": [
			{
				"configured": false,
				"connection": {
					"cid": 215,
					"ip": "2.3.4.5",
					"port": 34528,
					"start": "2021-05-07T04:04:12.542011131Z",
					"last_activity": "2021-05-07T04:04:12.72102885Z",
					"rtt": "84.178305ms",
					"uptime": "14h9m35s",
					"idle": "14h9m34s",
					"pending_bytes": 0,
					"in_msgs": 6,
					"out_msgs": 0,
					"in_bytes": 0,
					"out_bytes": 0,
					"subscriptions": 0,
					"name": "name5T",
					"tls_version": "1.3",
					"tls_cipher_suite": "TLS_AES_128_GCM_SHA256"
				}
			},
			{
				"configured": false,
				"connection": {
					"cid": 216,
					"ip": "2.3.4.6",
					"port": 49922,
					"start": "2021-05-07T04:09:33.765969461Z",
					"last_activity": "2021-05-07T18:13:38.277460462Z",
					"rtt": "85.411315ms",
					"uptime": "14h4m13s",
					"idle": "9s",
					"pending_bytes": 0,
					"in_msgs": 5367657,
					"out_msgs": 0,
					"in_bytes": 220785260,
					"out_bytes": 0,
					"subscriptions": 0,
					"name": "nameE3",
					"tls_version": "1.3",
					"tls_cipher_suite": "TLS_AES_128_GCM_SHA256"
				}
			},
			{
				"configured": false,
				"connection": {
					"cid": 214,
					"ip": "2.3.4.7",
					"port": 34416,
					"start": "2021-05-07T04:04:09.985755017Z",
					"last_activity": "2021-05-07T04:04:10.158700657Z",
					"rtt": "81.210233ms",
					"uptime": "14h9m37s",
					"idle": "14h9m37s",
					"pending_bytes": 0,
					"in_msgs": 6,
					"out_msgs": 0,
					"in_bytes": 0,
					"out_bytes": 0,
					"subscriptions": 0,
					"name": "nameW4",
					"tls_version": "1.3",
					"tls_cipher_suite": "TLS_AES_128_GCM_SHA256"
				}
			}
		]
	}
}
`

}

// AccstatzTestResponse is static data for tests
func AccstatzTestResponse() string {
	return `{
	"server_id": "SERVER_ID",
	"now": "2021-05-07T18:13:47.70796395Z",
	"account_statz": [
		{
	      "acc": "$G",
	      "conns": 100,
	      "leafnodes": 0,
	      "total_conns": 1000,
	      "sent": {
	        "msgs": 0,
	        "bytes": 0
	      },
	      "received": {
	        "msgs": 35922,
	        "bytes": 4574155
	      },
	      "slow_consumers": 0
	    },
		{
	      "acc": "$A",
	      "conns": 0,
	      "leafnodes": 0,
	      "total_conns": 0,
	      "sent": {
	        "msgs": 0,
	        "bytes": 0
	      },
	      "received": {
	        "msgs": 0,
	        "bytes": 0
	      },
	      "slow_consumers": 0
	    }
	]
}
`
}

func leafzTestResponse() string {
	return `{
	"server_id": "NC2FJCRMPBE5RI5OSRN7TKUCWQONCKNXHKJXCJIDVSAZ6727M7MQFVT3",
	"now": "2019-08-27T09:07:05.841132-06:00",
	"leafnodes": 1,
	"leafs": [
		{
			"account": "$G",
			"ip": "127.0.0.1",
			"port": 6223,
			"rtt": "200µs",
			"in_msgs": 0,
			"out_msgs": 10000,
			"in_bytes": 0,
			"out_bytes": 1280000,
			"subscriptions": 1,
			"subscriptions_list": [
				"foo"
			]
		},
		{
			"account": "$G",
			"ip": "127.0.0.2",
			"port": 6224,
			"rtt": "400µs",
			"in_msgs": 20,
			"out_msgs": 20000,
			"in_bytes": 30,
			"out_bytes": 2560000,
			"subscriptions": 2,
			"subscriptions_list": [
				"foo",
				"bar"
			]
		}
	]
}`
}
