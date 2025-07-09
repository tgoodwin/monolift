package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/types/post"
)

var logger = log.New(os.Stdout, "monolith-main: ", log.LstdFlags)

type serviceServer struct {
	serviceDelegate postservice.Service
}

// handleAddComment handles HTTP requests for the AddComment method.
func (s *serviceServer) handleAddComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for AddComment")

	var req post.CommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for AddComment: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.AddComment(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for AddComment failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for AddComment: %v", err)
	}
}

// handleDeletePost handles HTTP requests for the DeletePost method.
func (s *serviceServer) handleDeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for DeletePost")

	var req post.DelPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for DeletePost: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.DeletePost(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for DeletePost failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for DeletePost: %v", err)
	}
}

// handleReadPosts handles HTTP requests for the ReadPosts method.
func (s *serviceServer) handleReadPosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for ReadPosts")

	var req post.ReadPostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for ReadPosts: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.ReadPosts(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for ReadPosts failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for ReadPosts: %v", err)
	}
}

// handleSavePost handles HTTP requests for the SavePost method.
func (s *serviceServer) handleSavePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for SavePost")

	var req post.SavePostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for SavePost: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.SavePost(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for SavePost failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for SavePost: %v", err)
	}
}

// handleUpdateMeta handles HTTP requests for the UpdateMeta method.
func (s *serviceServer) handleUpdateMeta(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for UpdateMeta")

	var req post.MetaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for UpdateMeta: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.UpdateMeta(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for UpdateMeta failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for UpdateMeta: %v", err)
	}
}

// handleUpvotePost handles HTTP requests for the UpvotePost method.
func (s *serviceServer) handleUpvotePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	fmt.Println("Handling request for UpvotePost")

	var req post.UpvoteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("failed to decode request for UpvotePost: %v", err), http.StatusBadRequest)
		return
	}

	resp, err := s.serviceDelegate.UpvotePost(r.Context(), req)
	if err != nil {
		http.Error(w, fmt.Sprintf("service call for UpvotePost failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// If encoding fails, it's too late to send a different status code.
		log.Printf("failed to encode response for UpvotePost: %v", err)
	}
}

// Main function to set up the HTTP server
func main() {

	dbStore, err := database.NewDaprStore()

	postSvc := postservice.NewService(dbStore)

	if err != nil {
		logger.Fatalf("Failed to create dapr store: %v", err)
	}

	// --- Server Setup ---
	// The root dependency is the service delegate we need to wrap.
	serverInstance := &serviceServer{
		serviceDelegate: postSvc,
	}

	http.HandleFunc("/addcomment", serverInstance.handleAddComment)

	http.HandleFunc("/deletepost", serverInstance.handleDeletePost)

	http.HandleFunc("/readposts", serverInstance.handleReadPosts)

	http.HandleFunc("/savepost", serverInstance.handleSavePost)

	http.HandleFunc("/updatemeta", serverInstance.handleUpdateMeta)

	http.HandleFunc("/upvotepost", serverInstance.handleUpvotePost)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
