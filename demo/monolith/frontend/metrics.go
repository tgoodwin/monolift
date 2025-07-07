package frontend

import (
	"log"
	"net/http"
	"os"

	"github.com/tgoodwin/monolift/demo/monolith/util" // Adjusted import path

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = log.New(os.Stdout, "monolith-frontend: ", log.LstdFlags|log.Lshortfile)

// prometheus metric
var (
	// A single counter vector for all frontend requests, partitioned by type.
	// This is the idiomatic way to handle this kind of metric.
	frontendReqsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "frontend_requests_total",
			Help: "Total number of frontend requests, partitioned by type.",
		},
		[]string{"request_type"},
	)

	// Latency Histograms (simplified for now, will be expanded as services are ported)
	e2eReqLatHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "frontend_e2e_latency_ms",
			Help:    "End-to-end latency (ms) histogram of frontend requests by type.",
			Buckets: util.LatBuckets(),
		},
		[]string{"request_type"},
	)
	readImageStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "frontend_read_image_store_latency_ms",
		Help:    "Latency (ms) histogram of reading image store by frontend.",
		Buckets: util.LatBuckets(),
	})
	writeImageStoreLatHist = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "frontend_write_image_store_latency_ms",
		Help:    "Latency (ms) histogram of writing image store by frontend.",
		Buckets: util.LatBuckets(),
	})
	// Placeholder for other specific histograms if needed
	// For example, if we want to keep the very specific ones from the original:
	// saveReqLatHist = prometheus.NewHistogram(...)
)

// RegisterMetrics registers all the metrics defined in this package.
func RegisterMetrics() {
	prometheus.MustRegister(frontendReqsTotal)
	prometheus.MustRegister(e2eReqLatHist)
	prometheus.MustRegister(readImageStoreLatHist)
	prometheus.MustRegister(writeImageStoreLatHist)

	// Example of how you might register more specific histograms if you keep them
	// prometheus.MustRegister(saveReqLatHist)
	// prometheus.MustRegister(imgReqLatHist)
	// prometheus.MustRegister(readTlReqLatHist)
	// prometheus.MustRegister(readStoreLatHist)
	// prometheus.MustRegister(updateStoreLatHist)

	logger.Printf("Prometheus metrics registered.")
}

// GetPrometheusHandler returns the promhttp.Handler() to be used by the main server
func GetPrometheusHandler() http.Handler {
	return promhttp.Handler()
}
