package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	"github.com/tgoodwin/monolift/demo/monolith/timelineservice"
	"github.com/tgoodwin/monolift/demo/monolith/types/timeline"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

type serviceServer struct {
	serviceDelegate timelineservice.Service
}

// handleReadTimeline handles HTTP requests for the ReadTimeline method.
func (s *serviceServer) handleReadTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for ReadTimeline")

	var req timeline.ReadReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for ReadTimeline: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.ReadTimeline(r.Context(), req)
	if err != nil {
		fmt.Printf("service call for ReadTimeline failed: %v\n", err)
		http.Error(w, fmt.Sprintf("service call for ReadTimeline failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for ReadTimeline: %v", err)
	}
}

// handleUpdateTimeline handles HTTP requests for the UpdateTimeline method.
func (s *serviceServer) handleUpdateTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for UpdateTimeline")

	var req timeline.UpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for UpdateTimeline: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.UpdateTimeline(r.Context(), req)
	if err != nil {
		fmt.Printf("service call for UpdateTimeline failed: %v\n", err)
		http.Error(w, fmt.Sprintf("service call for UpdateTimeline failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for UpdateTimeline: %v", err)
	}
}

// Main function to set up the HTTP server
func main() {

	dbStore, err := database.NewDaprStore()

	socialGraphSvc := socialgraph.NewService(dbStore)

	postSvc := postservice.NewService(dbStore)

	if err != nil {
		logger.Fatalf("Failed to create dapr store: %v", err)
	}

	timelineSvc := timelineservice.NewService(dbStore, socialGraphSvc, postSvc)

	// --- Server Setup ---
	// The root dependency is the service delegate we need to wrap.
	serverInstance := &serviceServer{
		serviceDelegate: timelineSvc,
	}

	http.HandleFunc("/readtimeline", serverInstance.handleReadTimeline)

	http.HandleFunc("/updatetimeline", serverInstance.handleUpdateTimeline)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
