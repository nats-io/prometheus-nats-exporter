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
