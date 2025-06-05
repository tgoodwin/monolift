package postservice

import (
	"dapr-apps/socialnet/monolith/util"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics for the post service
// These are adapted from daprApps_v1/socialNetwork/post/main.go
var (
	readCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postservice_read_req_total", // Renamed for clarity and consistency
			Help: "Number of post read requests received by the post service.",
		},
	)
	updateCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "postservice_update_req_total", // Renamed for clarity and consistency
			Help: "Number of post update requests (save, del, meta, comment, upvote) received by the post service.",
		},
	)
	saveReqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "postservice_save_req_latency_ms",
		Help:    "Latency (ms) histogram of post save requests processed by post service, excluding time waiting for kvs/db.",
		Buckets: util.LatBuckets(),
	})
	updateReqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "postservice_update_req_latency_ms",
		Help:    "Latency (ms) histogram of post update requests (meta, comment, upvote & del) processed by post service, excluding time waiting for kvs/db.",
		Buckets: util.LatBuckets(),
	})
	readReqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "postservice_read_req_latency_ms",
		Help:    "Latency (ms) histogram of post read requests processed by post service, excluding time waiting for kvs/db.",
		Buckets: util.LatBuckets(),
	})
	readStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "postservice_read_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store read operations (e.g., GetState).",
		Buckets: util.LatBuckets(),
	})
	writeStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "postservice_write_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store write operations (e.g., SaveState, DeleteState).",
		Buckets: util.LatBuckets(),
	})

	// updateStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
	// 	Name:    "postservice_store_update_latency_ms", // Read-then-write operations
	// 	Help:    "Latency (ms) histogram of updating post store (kvs/db) by post service.",
	// 	Buckets: util.LatBuckets(),
	// })
)

// RegisterMetrics registers the Prometheus metrics for the post service.
// This should be called from the main application setup.
func RegisterMetrics() {
	prometheus.MustRegister(readCtr)
	prometheus.MustRegister(updateCtr)
	prometheus.MustRegister(saveReqLatHist)
	prometheus.MustRegister(updateReqLatHist)
	prometheus.MustRegister(readReqLatHist)
	prometheus.MustRegister(readStoreLatHist)
	prometheus.MustRegister(writeStoreLatHist)
	// prometheus.MustRegister(updateStoreLatHist)
	logger.Println("Post service metrics registered.")
}
