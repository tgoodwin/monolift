package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	"github.com/tgoodwin/monolift/demo/monolith/types/user"
	"github.com/tgoodwin/monolift/demo/monolith/userservice"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

type serviceServer struct {
	serviceDelegate userservice.Service
}

// handleLogin handles HTTP requests for the Login method.
func (s *serviceServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for Login")

	var req user.LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for Login: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.Login(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for Login failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for Login: %v", err)
	}
}

// handleRegister handles HTTP requests for the Register method.
func (s *serviceServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for Register")

	var req user.RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for Register: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.Register(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for Register failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for Register: %v", err)
	}
}

// Main function to set up the HTTP server
func main() {

	dbStore, err := database.NewDaprStore()

	socialGraphSvc := socialgraph.NewService(dbStore)

	if err != nil {
		logger.Fatalf("Failed to create dapr store: %v", err)
	}

	userSvc := userservice.NewService(socialGraphSvc, dbStore)

	// --- Server Setup ---
	// The root dependency is the service delegate we need to wrap.
	serverInstance := &serviceServer{
		serviceDelegate: userSvc,
	}

	http.HandleFunc("/login", serverInstance.handleLogin)

	http.HandleFunc("/register", serverInstance.handleRegister)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
