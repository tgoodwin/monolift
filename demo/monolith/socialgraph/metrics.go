package socialgraph

import (
	"dapr-apps/socialnet/monolith/util"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics for the socialgraph service
// Adapted from daprApps_v1/socialNetwork/socialgraph/main.go
var (
	readCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "socialgraph_read_req_total",
			Help: "Number of socialgraph read requests (get_follow, get_follower) received.",
		},
	)
	recmdCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "socialgraph_recmd_req_total",
			Help: "Number of socialgraph recommendation requests received.",
		},
	)
	updateCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "socialgraph_update_req_total",
			Help: "Number of socialgraph update requests (follow, unfollow) received.",
		},
	)
	reqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "socialgraph_req_latency_ms",
		Help:    "Latency (ms) histogram of socialgraph requests (get_follow, get_follower, follow, unfollow), excluding time waiting for kvs/db.",
		Buckets: util.LatBuckets(),
	})
	recmdLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "socialgraph_recmd_latency_ms",
		Help:    "Latency (ms) histogram of socialgraph recommendation requests, excluding time waiting for kvs/db.",
		Buckets: util.LatBuckets(),
	})
	readStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "socialgraph_read_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store read operations (e.g., GetState).",
		Buckets: util.LatBuckets(),
	})
	writeStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "socialgraph_write_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store write operations (e.g., SaveState, DeleteState).",
		Buckets: util.LatBuckets(),
	})
	// Note: The original Dapr socialgraph had a single `updateStoreLatHist` for read-then-write.
	// We'll use separate read and write store histograms for more granular insight with our current DB interface.
)

// RegisterMetrics registers the Prometheus metrics for the socialgraph service.
func RegisterMetrics() {
	prometheus.MustRegister(readCtr)
	prometheus.MustRegister(recmdCtr)
	prometheus.MustRegister(updateCtr)
	prometheus.MustRegister(reqLatHist)
	prometheus.MustRegister(recmdLatHist)
	prometheus.MustRegister(readStoreLatHist)
	prometheus.MustRegister(writeStoreLatHist)
	logger.Println("SocialGraph service metrics registered.")
}
