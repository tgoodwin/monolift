package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

type serviceServer struct {
	serviceDelegate socialgraph.Service
}

// handleFollow handles HTTP requests for the Follow method.
func (s *serviceServer) handleFollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for Follow")

	var req socialgraph.FollowReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for Follow: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.Follow(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for Follow failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for Follow: %v", err)
	}
}

// handleGetFollowees handles HTTP requests for the GetFollowees method.
func (s *serviceServer) handleGetFollowees(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for GetFollowees")

	var req socialgraph.GetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for GetFollowees: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.GetFollowees(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for GetFollowees failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for GetFollowees: %v", err)
	}
}

// handleGetFollowers handles HTTP requests for the GetFollowers method.
func (s *serviceServer) handleGetFollowers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for GetFollowers")

	var req socialgraph.GetReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for GetFollowers: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.GetFollowers(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for GetFollowers failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for GetFollowers: %v", err)
	}
}

// handleGetRecommendations handles HTTP requests for the GetRecommendations method.
func (s *serviceServer) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for GetRecommendations")

	var req socialgraph.GetRecmdReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for GetRecommendations: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.GetRecommendations(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for GetRecommendations failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for GetRecommendations: %v", err)
	}
}

// handleUnfollow handles HTTP requests for the Unfollow method.
func (s *serviceServer) handleUnfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for Unfollow")

	var req socialgraph.UnfollowReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for Unfollow: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.Unfollow(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for Unfollow failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for Unfollow: %v", err)
	}
}

// Main function to set up the HTTP server
func main() {

	dbStore, err := database.NewDaprStore()

	socialGraphSvc := socialgraph.NewService(dbStore)

	if err != nil {
		logger.Fatalf("Failed to create dapr store: %v", err)
	}

	// --- Server Setup ---
	// The root dependency is the service delegate we need to wrap.
	serverInstance := &serviceServer{
		serviceDelegate: socialGraphSvc,
	}

	http.HandleFunc("/follow", serverInstance.handleFollow)

	http.HandleFunc("/getfollowees", serverInstance.handleGetFollowees)

	http.HandleFunc("/getfollowers", serverInstance.handleGetFollowers)

	http.HandleFunc("/getrecommendations", serverInstance.handleGetRecommendations)

	http.HandleFunc("/unfollow", serverInstance.handleUnfollow)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
