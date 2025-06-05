package main

import (
	"log"
	"net/http"
	"os"

	"dapr-apps/socialnet/monolith/database" // Import the new database package
	"dapr-apps/socialnet/monolith/frontend"
	"dapr-apps/socialnet/monolith/postservice"
	"dapr-apps/socialnet/monolith/socialgraph"     // Import the socialgraph service
	"dapr-apps/socialnet/monolith/timelineservice" // Import the timelineservice
	"dapr-apps/socialnet/monolith/userservice"     // Import the userservice
	"dapr-apps/socialnet/monolith/util"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags|log.Lshortfile)

func main() {
	serviceAddress := util.GetEnvVar("ADDRESS", ":8080")   // Main service address
	promAddress := util.GetEnvVar("PROM_ADDRESS", ":8084") // Prometheus metrics address

	// Setup Prometheus metrics from the frontend package
	// The actual HTTP handler for /metrics will be started on a separate port or by the main mux
	frontend.SetupPrometheus(promAddress) // This just registers the metrics
	postservice.RegisterMetrics()         // Register postservice metrics
	socialgraph.RegisterMetrics()         // Register socialgraph service metrics
	userservice.RegisterMetrics()         // Register userservice metrics
	timelineservice.RegisterMetrics()     // Register timelineservice metrics

	dbStore := database.NewInMemoryKVStore()

	// Instantiate service modules
	socialGraphSvc := socialgraph.NewService(dbStore)
	userSvc := userservice.NewService(socialGraphSvc, dbStore)
	postSvc := postservice.NewService(dbStore)
	timelineSvc := timelineservice.NewService(dbStore, socialGraphSvc, postSvc)

	mux := http.NewServeMux()

	// Register API handlers from the frontend package
	// Pass dbStore for direct storage access like images
	frontend.RegisterHandlers(mux, postSvc, socialGraphSvc, userSvc, timelineSvc, dbStore)

	// Expose Prometheus metrics on the main server's mux
	mux.Handle("/metrics", frontend.GetPrometheusHandler())

	logger.Printf("Monolith Social Network server starting on %s", serviceAddress)
	if err := http.ListenAndServe(serviceAddress, mux); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}
}
