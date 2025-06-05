package util

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var logger = log.New(os.Stdout, "", 0)

// GetEnvVar returns the string value of an environment variable or a default value.
func GetEnvVar(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	logger.Printf("Env var %s not found, using default: %s", key, fallback)
	return fallback
}

// LatBuckets returns the buckets for latency histograms.
func LatBuckets() []float64 {
	return []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0,
		12.0, 15.0, 18.0, 20.0, 25.0, 30.0, 40.0, 50.0, 60.0,
		70.0, 80.0, 90.0, 100.0, 120.0, 150.0, 180.0, 200.0,
		250.0, 300.0, 400.0, 500.0, 600.0, 700.0, 800.0,
		900.0, 1000.0, 1500.0, 2000.0, 2500.0, 3000.0, 4000.0,
		5000.0, 10000.0, 20000.0, 30000.0, 40000.0, 50000.0, 60000.0}
}

// PostId generates a post ID.
func PostId(userId string, sendUnixMilli int64) string {
	return userId + "-" + strconv.FormatInt(sendUnixMilli, 10)
}

// ImageId generates an image ID.
func ImageId(postId string, idx int) string {
	return postId + "-img-" + strconv.Itoa(idx)
}

// CommentId generates a comment ID.
func CommentId(userId string, sendUnixMilli int64) string {
	return userId + "-comm-" + strconv.FormatInt(sendUnixMilli, 10)
}

// Helper function to observe latency and handle potential errors.
func ObserveLatency(hist *prometheus.HistogramVec, start time.Time, labels ...string) {
	(*hist).WithLabelValues(labels...).Observe(float64(time.Since(start).Milliseconds()))
}

// Helper function to observe latency and handle potential errors for non-vector histograms.
func ObserveHist(hist prometheus.Observer, val float64) {
	hist.Observe(val)
}
