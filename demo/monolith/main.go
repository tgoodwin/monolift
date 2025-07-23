package main

import (
	"log"
	"net/http"
	httppprof "net/http/pprof"
	"os"
	"strconv"
	"time"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/frontend"
	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	"github.com/tgoodwin/monolift/demo/monolith/timelineservice"
	"github.com/tgoodwin/monolift/demo/monolith/userservice"
	"github.com/tgoodwin/monolift/demo/monolith/util"
	"github.com/tgoodwin/monolift/pkg/metrics"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

func main() {
	serviceAddress := util.GetEnvVar("ADDRESS", ":8080")   // Main service address
	promAddress := util.GetEnvVar("PROM_ADDRESS", ":8084") // Prometheus metrics address

	timelineNumWorkersEnvVar := util.GetEnvVar("TIMELINE_NUM_WORKERS", "500") // Number of workers for timeline processing
	logger.Printf("Using TIMELINE_NUM_WORKERS: %s", timelineNumWorkersEnvVar)
	timelineNumWorkers, _ := strconv.Atoi(timelineNumWorkersEnvVar)

	if timelineNumWorkers <= 0 {
		timelineNumWorkers = 500 // Default to 500 if not set or invalid
	}
	timelineBufferSizeEnvVar := util.GetEnvVar("TIMELINE_BUFFER_SIZE", "1000") // Buffer size for timeline job channel
	logger.Printf("Using TIMELINE_BUFFER_SIZE: %s", timelineBufferSizeEnvVar)
	timelineBufferSize, _ := strconv.Atoi(timelineBufferSizeEnvVar)

	if timelineBufferSize <= 0 {
		timelineBufferSize = 10000 // Default to 10000 if not set or invalid
	}

	// Register all metrics for Prometheus to expose.
	database.RegisterMetrics()
	frontend.RegisterMetrics()
	postservice.RegisterMetrics()
	socialgraph.RegisterMetrics()
	userservice.RegisterMetrics()
	timelineservice.RegisterMetrics()

	dbStore, err := database.NewDaprStore()
	if err != nil {
		logger.Fatalf("Failed to create dapr store: %v", err)
	}

	// Instantiate service modules
	socialGraphSvc := socialgraph.NewService(dbStore)
	userSvc := userservice.NewService(socialGraphSvc, dbStore)
	postSvc := postservice.NewService(dbStore)
	timelineSvc := timelineservice.NewService(dbStore, socialGraphSvc, postSvc, timelineNumWorkers, timelineBufferSize)

	// Main application mux
	appMux := http.NewServeMux()

	// Register API handlers from the frontend package
	// Pass dbStore for direct storage access like images
	frontend.RegisterHandlers(appMux, postSvc, socialGraphSvc, userSvc, timelineSvc, dbStore)

	metricsMonitor, err := metrics.NewMonitor(5 * time.Second)
	if err != nil {
		logger.Fatalf("Failed to create metrics monitor: %v", err)
	}
	defer metricsMonitor.Close()
	// Start polling for metrics in the background
	go metricsMonitor.PollPrint(1 * time.Second)

	// Set up a separate mux and server for Prometheus metrics.
	promMux := http.NewServeMux()
	promMux.Handle("/metrics", frontend.GetPrometheusHandler())

	logger.Printf("Registering pprof handlers")
	promMux.HandleFunc("/debug/pprof/", httppprof.Index)
	promMux.HandleFunc("/debug/pprof/cmdline", httppprof.Cmdline)
	promMux.HandleFunc("/debug/pprof/profile", httppprof.Profile)
	promMux.HandleFunc("/debug/pprof/symbol", httppprof.Symbol)

	go func() {
		logger.Printf("Prometheus metrics server starting on %s", promAddress)
		if err := http.ListenAndServe(promAddress, promMux); err != nil {
			logger.Fatalf("Failed to start Prometheus metrics server: %v", err)
		}
	}()

	logger.Printf("Monolith Social Network server starting on %s", serviceAddress)
	if err := http.ListenAndServe(serviceAddress, appMux); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
