package frontend

import (
	"log"
	"net/http"
	"os"

	"dapr-apps/socialnet/monolith/util" // Adjusted import path

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var logger = log.New(os.Stdout, "monolith-frontend: ", log.LstdFlags|log.Lshortfile)

// prometheus metric
var (
	// flow counters
	saveReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_save_req",
			Help: "Number of frontend save requests received.",
		},
	)
	delReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_del_req",
			Help: "Number of frontend delete requests received.",
		},
	)
	commentReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_comment_req",
			Help: "Number of frontend comment requests received.",
		},
	)
	upvoteReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_upvote_req",
			Help: "Number of frontend upvote requests received.",
		},
	)
	imageReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_image_req",
			Help: "Number of frontend image read requests received.",
		},
	)
	tlReqCtr = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "frontend_tl_req",
			Help: "Number of frontend timeline read requests received.",
		},
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

func SetupPrometheus(promAddress string) {
	prometheus.MustRegister(saveReqCtr)
	prometheus.MustRegister(delReqCtr)
	prometheus.MustRegister(commentReqCtr)
	prometheus.MustRegister(upvoteReqCtr)
	prometheus.MustRegister(imageReqCtr)
	prometheus.MustRegister(tlReqCtr)
	prometheus.MustRegister(e2eReqLatHist)
	prometheus.MustRegister(readImageStoreLatHist)
	prometheus.MustRegister(writeImageStoreLatHist)

	// Example of how you might register more specific histograms if you keep them
	// prometheus.MustRegister(saveReqLatHist)
	// prometheus.MustRegister(imgReqLatHist)
	// prometheus.MustRegister(readTlReqLatHist)
	// prometheus.MustRegister(readStoreLatHist)
	// prometheus.MustRegister(updateStoreLatHist)

	// The Handler function provides a default handler to expose metrics
	// via an HTTP server. "/metrics" is the usual endpoint for that.
	// This will be handled by the main HTTP server in monolith/main.go
	// http.Handle("/metrics", promhttp.Handler())
	// logger.Fatal(http.ListenAndServe(promAddress, nil))
	// Instead, we just register. The main server will expose promhttp.Handler().

	logger.Printf("Prometheus metrics registered. Metrics will be available at /metrics on the main server.")
}

// GetPrometheusHandler returns the promhttp.Handler() to be used by the main server
func GetPrometheusHandler() http.Handler {
	return promhttp.Handler()
}
