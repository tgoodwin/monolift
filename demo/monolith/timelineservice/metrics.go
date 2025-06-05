package timelineservice

import (
	"dapr-apps/socialnet/monolith/util"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics for the timeline service
var (
	readTimelineReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "timelineservice_read_req_total",
			Help: "Number of timeline read requests received.",
		},
	)
	updateTimelineReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "timelineservice_update_req_total",
			Help: "Number of timeline update requests received (e.g., from new posts or deletions).",
		},
	)
	reqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "timelineservice_req_latency_ms",
		Help:    "Latency (ms) histogram of timeline service requests (read, update), excluding time waiting for kvs/db or other services.",
		Buckets: util.LatBuckets(),
	})
	readStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "timelineservice_read_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store read operations (e.g., GetState).",
		Buckets: util.LatBuckets(),
	})
	writeStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "timelineservice_write_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store write operations (e.g., SaveState, DeleteState).",
		Buckets: util.LatBuckets(),
	})
)

// RegisterMetrics registers the Prometheus metrics for the timeline service.
func RegisterMetrics() {
	prometheus.MustRegister(readTimelineReqCtr)
	prometheus.MustRegister(updateTimelineReqCtr)
	prometheus.MustRegister(reqLatHist)
	prometheus.MustRegister(readStoreLatHist)
	prometheus.MustRegister(writeStoreLatHist)
	logger.Println("Timeline service metrics registered.")
}
