package userservice

import (
	"dapr-apps/socialnet/monolith/util"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics for the user service
var (
	registerReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "userservice_register_req_total",
			Help: "Number of user registration requests received.",
		},
	)
	loginReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "userservice_login_req_total",
			Help: "Number of user login requests received.",
		},
	)
	reqLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "userservice_req_latency_ms",
		Help:    "Latency (ms) histogram of user service requests (register, login), excluding time waiting for kvs/db or other services.",
		Buckets: util.LatBuckets(),
	})
	readStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "userservice_read_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store read operations (e.g., GetState).",
		Buckets: util.LatBuckets(),
	})
	writeStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "userservice_write_store_latency_ms",
		Help:    "Latency (ms) histogram of underlying store write operations (e.g., SaveState, DeleteState).",
		Buckets: util.LatBuckets(),
	})
)

// RegisterMetrics registers the Prometheus metrics for the user service.
func RegisterMetrics() {
	prometheus.MustRegister(registerReqCtr)
	prometheus.MustRegister(loginReqCtr)
	prometheus.MustRegister(reqLatHist)
	prometheus.MustRegister(readStoreLatHist)
	prometheus.MustRegister(writeStoreLatHist)
	logger.Println("User service metrics registered.")
}
