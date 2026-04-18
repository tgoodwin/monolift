package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/frontend"
	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	"github.com/tgoodwin/monolift/demo/monolith/timelineservice"
	"github.com/tgoodwin/monolift/demo/monolith/userservice"
	"github.com/tgoodwin/monolift/demo/monolith/util"
	"github.com/tgoodwin/monolift/pkg/metrics"
	"github.com/tgoodwin/monolift/pkg/pragma"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

func main() {
	monoliftMetricsMonitor, err := metrics.NewMonitor(1 * time.Second)
	// Main service address
	if err != nil {
		log.Fatalf("failed to create metrics monitor: %v", err)
	}
	defer monoliftMetricsMonitor.

		// Prometheus metrics address
		Close()
	serviceAddress := util.GetEnvVar("ADDRESS", ":8080")
	promAddress := util.GetEnvVar("PROM_ADDRESS", ":8084")

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
	var timelineSvc timelineservice.Service
	{
		localSvc := timelineservice.NewService(dbStore, socialGraphSvc, postSvc)
		remoteSvc :=

			// Main application mux
			NewtimelineserviceClient("http://timelineservice.default")

		// Register API handlers from the frontend package
		// Pass dbStore for direct storage access like images
		decider := pragma.NewIPSDecider("timelineservice.Service", 100)
		timelineSvc = NewtimelineserviceClientDelegate(localSvc, remoteSvc, decider)
	}

	appMux := http.NewServeMux()

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
