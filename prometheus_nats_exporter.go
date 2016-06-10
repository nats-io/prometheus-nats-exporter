package main

import (
    "net/http"
    "github.com/prometheus/client_golang/prometheus"
    "flag"
    "fmt"
    "strconv"
    "log"
    "sync"
    "strings"
)

// scrape the endpoints listed.
// for each numeric metric
//  if we have a definition.
//  register the metric with help, and type set.
//
// for each map metric
//  append the map name and the metric to each
var (
  listenAddress string
  listenPort int
)

type NATSCollector struct {
  URLs []string
  Stats map[string]*prometheus.GaugeVec
  mtx sync.Mutex
}

func (nc *NATSCollector) Describe(ch chan<- *prometheus.Desc) {
  // for each stat in nc.Stats
  for _, k := range nc.Stats {
      // Describe it to the channel.
      k.Describe(ch)
  }
}

func (nc *NATSCollector) Collect(ch chan<- prometheus.Metric) {
  nc.mtx.Lock()
  defer nc.mtx.Unlock()
  // query the URL for the most recent stats.
  // get all the Metrics at once, then set the stats and collect them together.
  resps := make(map[string]map[string]interface{})
  for _, u := range nc.URLs {
    resps[u] = GetMetricURL(u)
  }

  // for each stat, see if each response contains that stat. then collect.
  for idx, k := range nc.Stats {
    for url, response := range resps {
      switch v := response[idx].(type) {
      case float64: // not sure why, but all my json numbers are coming here.
        k.WithLabelValues(url).Set(v)
      default:
        fmt.Println("value no longer a float", url, v)
      }
    }
    k.Collect(ch) // update the stat.
  }
}

func NewNATSCollector(urls []string) *NATSCollector {
  return &NATSCollector{
    URLs: urls,
    Stats: LoadMetricConfigFromResponse(urls, GetMetricURL(urls[0])),
  }
}

func main() {
    // Parse flags
  	flag.IntVar(&listenPort, "port", 7777, "Port to listen on.")
  	flag.IntVar(&listenPort, "p", 7777, "Port to listen on.")
  	flag.StringVar(&listenAddress, "addr", "", "Network host to listen on.")
  	flag.StringVar(&listenAddress, "a", "", "Network host to listen on.")
    flag.Parse()

    // indexed.Inc()
    // size.Set(5)
    http.Handle("/metrics", prometheus.Handler())


    metrics := make(map[string][]string)

    // for each URL in args
    for _, arg := range flag.Args() {
      // create a map of possible metric endpoints.
      endpoint := arg[strings.LastIndex(arg, "/")+1:] // TODO: not safe.
      // append the url to that endpoint.
      metrics[endpoint] = append(metrics[endpoint], arg)
    }

    for _, v := range metrics {
      fmt.Println("registering", v)
      nc := NewNATSCollector(v)
      prometheus.MustRegister(nc)
    }

    prometheus.EnableCollectChecks(true)

    fmt.Println("Server starting on " + listenAddress + ":" + strconv.Itoa(listenPort))
    log.Fatal(http.ListenAndServe(listenAddress + ":" + strconv.Itoa(listenPort), nil))
}

func init() {
}
