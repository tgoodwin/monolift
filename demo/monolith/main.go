package main

import (
	"context"
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
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

func main() {
	serviceAddress := util.GetEnvVar("ADDRESS", ":8080")              // Main service address
	promAddress := util.GetEnvVar("PROM_ADDRESS", ":8084")            // Prometheus metrics address
	redisAddress := util.GetEnvVar("REDIS_ADDRESS", "localhost:6379") // Redis address for future use
	redisPassword := util.GetEnvVar("REDIS_PASSWORD", "")             // Redis password, if any

	// Register all metrics for Prometheus to expose.
	frontend.RegisterMetrics()
	postservice.RegisterMetrics()
	socialgraph.RegisterMetrics()
	userservice.RegisterMetrics()
	timelineservice.RegisterMetrics()

	// dbStore := database.NewInMemoryKVStore()
	dbStore, err := database.NewRedisStore(context.Background(), redisAddress, redisPassword, 0)
	if err != nil {
		logger.Fatalf("Failed to connect to Redis at %s: %v", redisAddress, err)
	}

	// Instantiate service modules
	socialGraphSvc := socialgraph.NewService(dbStore)
	userSvc := userservice.NewService(socialGraphSvc, dbStore)
	postSvc := postservice.NewService(dbStore)
	timelineSvc := timelineservice.NewService(dbStore, socialGraphSvc, postSvc)

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
