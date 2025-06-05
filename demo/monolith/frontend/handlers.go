package frontend

import (
	"context" // Added for service calls
	"dapr-apps/socialnet/monolith/database"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"dapr-apps/socialnet/monolith/postservice"
	"dapr-apps/socialnet/monolith/socialgraph" // Import for socialgraph.Service
	"dapr-apps/socialnet/monolith/timelineservice"
	postTypes "dapr-apps/socialnet/monolith/types/post"
	timelineTypes "dapr-apps/socialnet/monolith/types/timeline" // Import timeline types
	userTypes "dapr-apps/socialnet/monolith/types/user"         // Import user types for API requests
	"dapr-apps/socialnet/monolith/userservice"                  // Import for userservice.Service
	"dapr-apps/socialnet/monolith/util"
)

const (
	imageStoreName = "image-store" // Consistent with original Dapr component name
)

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			logger.Printf("Error encoding JSON response: %v", err)
			// http.Error already sent header, so just log
		}
	}
}

type APIHandlers struct {
	PostService        postservice.Service
	SocialGraphService socialgraph.Service
	UserService        userservice.Service
	TimelineService    timelineservice.Service
	DBStore            database.Store
}

func RegisterHandlers(mux *http.ServeMux,
	postSvc postservice.Service,
	socialGraphSvc socialgraph.Service,
	userSvc userservice.Service,
	timelineSvc timelineservice.Service,
	dbStore database.Store) {
	h := &APIHandlers{
		PostService:        postSvc,
		SocialGraphService: socialGraphSvc,
		UserService:        userSvc,
		TimelineService:    timelineSvc,
		DBStore:            dbStore,
	}

	// from original Dapr implementation
	mux.HandleFunc("/save", h.SaveHandler)
	mux.HandleFunc("/del", h.DelHandler)
	mux.HandleFunc("/comment", h.CommentHandler)
	mux.HandleFunc("/upvote", h.UpvoteHandler)
	mux.HandleFunc("/image", h.ImageHandler)
	mux.HandleFunc("/timeline", h.TimelineHandler)

	// User service handlers
	mux.HandleFunc("/register", h.RegisterHandler)
	mux.HandleFunc("/login", h.LoginHandler)
}

// SaveHandler saves a new post
func (h *APIHandlers) SaveHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	saveReqCtr.Inc()
	logger.Println("SaveHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	// Expect multipart form data
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max memory
		logger.Printf("SaveHandler ParseMultipartForm err: %s", err.Error())
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	// Get text and user_id from form values
	userId := r.FormValue("user_id")
	text := r.FormValue("text")
	// send_unix_ms might be needed for PostId, but it's not standard in multipart.
	// We'll generate PostId based on current time and user_id, similar to original Dapr.
	// Or, if the client *must* provide it, it would be another form value.
	// Let's use current time for PostId generation for simplicity, aligning with util.PostId signature.
	sendUnixMilli := time.Now().UnixMilli() // Use server time for PostId generation

	if userId == "" || text == "" {
		http.Error(w, "user_id and text form values are required", http.StatusBadRequest)
		return
	}

	postId := util.PostId(userId, sendUnixMilli)

	// Step 2: Handle and save images from file parts
	files := r.MultipartForm.File["images"]
	imageIds := make([]string, len(files))

	for i, fileHeader := range files {
		imageIds[i] = util.ImageId(postId, i)

		file, err := fileHeader.Open()
		defer file.Close()
		if err != nil {
			logger.Printf("SaveHandler: error opening image file %s: %v", fileHeader.Filename, err)
			http.Error(w, "Invalid image data", http.StatusBadRequest)
			return
		}
		imageData, err := io.ReadAll(file)

		err = expensiveProcessingStep(imageData)

		storeWriteStartTime := time.Now()
		_, err = h.DBStore.SaveState(context.Background(), imageStoreName, imageIds[i], imageData, nil)
		util.ObserveHist(writeImageStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("SaveHandler: error saving image %s to store: %v", imageIds[i], err)
			http.Error(w, "Failed to save image", http.StatusInternalServerError)
			return
		}
		logger.Printf("SaveHandler: Saved image %s", imageIds[i])
	}

	// Step 3: Invoke post service to save post content
	postServiceReq := postTypes.SavePostReq{
		PostId: postId, // Generated PostId
		PostCont: postTypes.PostCont{
			UserId: userId,   // From form value
			Text:   text,     // From form value
			Images: imageIds, // Pass generated image IDs
		},
		SendUnixMilli: time.Now().UnixMilli(), // Timestamp for the service call itself
	}

	_, err := h.PostService.SavePost(context.Background(), postServiceReq)
	if err != nil {
		logger.Printf("SaveHandler error calling PostService.SavePost: %v", err)
		http.Error(w, "Failed to save post", http.StatusInternalServerError)
		return
	}

	// Update timeline for the user who made the post
	// In a full system, this would fan out to followers as well,
	// potentially handled within timelineservice or orchestrated here.
	timelineUpdateReq := timelineTypes.UpdateReq{
		UserId:        userId, // From form value
		PostId:        postId,
		Add:           true,
		ImageIncluded: len(imageIds) > 0,
		// ClientUnixMilli: req.SendUnixMilli, // Original client timestamp from frontend request
		SendUnixMilli: time.Now().UnixMilli(),
	}
	err = h.TimelineService.UpdateTimeline(context.Background(), timelineUpdateReq)
	if err != nil {
		// Log error, but don't fail the entire Save operation for a timeline update failure
		logger.Printf("SaveHandler: failed to update timeline for user %s, post %s: %v", userId, postId, err)
	}
	// 4. Publish timeline update event (Phase 4 - Go channels)
	// 5. Publish object detection event (Phase 4 - Go channels, stubbed)
	// 6. Publish sentiment analysis event (Phase 4 - Go channels, stubbed)
	logger.Printf("SaveHandler: PostId %s, User %s, Text: %s, Image IDs: %v", postId, userId, text, imageIds)

	resp := UpdateResp{PostId: postId} // Use type from frontend/types.go
	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("save"), float64(time.Since(startTime).Milliseconds()))
}

// DelHandler deletes a post
func (h *APIHandlers) DelHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	delReqCtr.Inc() // Assuming you'll add delReqCtr similar to saveReqCtr
	logger.Println("DelHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DelReq // Use type from frontend/types.go
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("DelHandler json.Unmarshal err: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	postServiceReq := postTypes.DelPostReq{
		PostId:        req.PostId,
		SendUnixMilli: time.Now().UnixMilli(),
	}

	_, err := h.PostService.DeletePost(context.Background(), postServiceReq)
	if err != nil {
		logger.Printf("DelHandler error calling PostService.DeletePost: %v", err)
		http.Error(w, "Failed to delete post", http.StatusInternalServerError)
		return
	}

	// Update timeline for the user whose post was deleted
	timelineUpdateReq := timelineTypes.UpdateReq{
		UserId:          req.UserId, // User who owned the post
		PostId:          req.PostId,
		Add:             false, // Deleting the post
		ImageIncluded:   false, // Not critical for delete, set to false
		ClientUnixMilli: req.SendUnixMilli,
		SendUnixMilli:   time.Now().UnixMilli(),
	}
	err = h.TimelineService.UpdateTimeline(context.Background(), timelineUpdateReq)
	if err != nil {
		logger.Printf("DelHandler: failed to update timeline for user %s, post %s: %v", req.UserId, req.PostId, err)
	}

	logger.Printf("DelHandler: PostId %s, User %s", req.PostId, req.UserId)

	resp := UpdateResp{PostId: req.PostId} // Use type from frontend/types.go
	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("delete"), float64(time.Since(startTime).Milliseconds()))
}

// CommentHandler adds a comment to a post
func (h *APIHandlers) CommentHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	commentReqCtr.Inc()
	logger.Println("CommentHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FrontendCommentAPIRq // Use type from frontend/types.go
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("CommentHandler json.Unmarshal err: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Construct postservice's CommentReq (this will now unambiguously be types.CommentReq from common.go)
	postServiceReq := postTypes.CommentReq{
		PostId: req.PostId,
		Comm: postTypes.Comment{
			CommentId: util.CommentId(req.UserId, time.Now().UnixMilli()), // Generate comment ID here
			UserId:    req.UserId,
			ReplyTo:   req.ReplyTo,
			Text:      req.Text,
		},
		SendUnixMilli: time.Now().UnixMilli(),
	}

	_, err := h.PostService.AddComment(context.Background(), postServiceReq)
	if err != nil {
		logger.Printf("CommentHandler error calling PostService.AddComment: %v", err)
		http.Error(w, "Failed to add comment", http.StatusInternalServerError)
		return
	}
	logger.Printf("CommentHandler: PostId %s, User %s, Text: %s", req.PostId, req.UserId, req.Text)

	resp := UpdateResp{PostId: req.PostId} // Use type from frontend/types.go
	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("comment"), float64(time.Since(startTime).Milliseconds()))
}

// UpvoteHandler upvotes a post
func (h *APIHandlers) UpvoteHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	upvoteReqCtr.Inc()
	logger.Println("UpvoteHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req postTypes.UpvoteReq // Use type from types/post package
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("UpvoteHandler json.Unmarshal err: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// The postTypes.UpvoteReq is the same for frontend and postservice
	postServiceReq := req
	postServiceReq.SendUnixMilli = time.Now().UnixMilli() // Update timestamp for service call

	_, err := h.PostService.UpvotePost(context.Background(), postServiceReq)
	if err != nil {
		logger.Printf("UpvoteHandler error calling PostService.UpvotePost: %v", err)
		http.Error(w, "Failed to upvote post", http.StatusInternalServerError)
		return
	}
	logger.Printf("UpvoteHandler: PostId %s, User %s", req.PostId, req.UserId)

	resp := UpdateResp{PostId: req.PostId} // Use type from frontend/types.go
	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("upvote"), float64(time.Since(startTime).Milliseconds()))
}

// ImageHandler returns the given image
func (h *APIHandlers) ImageHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	imageReqCtr.Inc()
	logger.Println("ImageHandler received request")

	imageId := r.URL.Query().Get("id")
	if imageId == "" {
		http.Error(w, "Image ID 'id' query parameter is required", http.StatusBadRequest)
		return
	}

	storeReadStartTime := time.Now()
	item, err := h.DBStore.GetState(context.Background(), imageStoreName, imageId)
	util.ObserveHist(readImageStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))

	if err != nil {
		logger.Printf("ImageHandler: error getting image %s from store: %v", imageId, err)
		http.Error(w, "Failed to retrieve image", http.StatusInternalServerError)
		return
	}
	if item == nil || item.Value == nil {
		logger.Printf("ImageHandler: image %s not found", imageId)
		http.Error(w, "Image not found", http.StatusNotFound)
		return
	}

	// The original Dapr frontend encodes to base64 before sending.
	// For direct image serving, we might set Content-Type and write bytes.
	// Here, we'll match the original and return base64.
	w.Header().Set("Content-Type", "text/plain") // Or application/octet-stream if sending raw bytes
	w.Write([]byte(base64.StdEncoding.EncodeToString(item.Value)))
	util.ObserveHist(e2eReqLatHist.WithLabelValues("image"), float64(time.Since(startTime).Milliseconds()))
}

// TimelineHandler reads all the post in a given timeline
func (h *APIHandlers) TimelineHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	tlReqCtr.Inc()
	logger.Println("TimelineHandler received request")

	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	userId := r.URL.Query().Get("user_id")
	if userId == "" {
		http.Error(w, "user_id query parameter is required", http.StatusBadRequest)
		return
	}

	userTlStr := r.URL.Query().Get("user_tl")
	userTl := false // Default to home timeline
	if userTlStr != "" {
		var err error
		userTl, err = strconv.ParseBool(userTlStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid user_tl value: %s. Must be true or false.", userTlStr), http.StatusBadRequest)
			return
		}
	}

	earlUnixMilliStr := r.URL.Query().Get("earl_unix_milli")
	earlUnixMilli := time.Now().UnixMilli() // Default to current time
	if earlUnixMilliStr != "" {
		var err error
		earlUnixMilli, err = strconv.ParseInt(earlUnixMilliStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid earl_unix_milli value. Must be an integer.", http.StatusBadRequest)
			return
		}
	}

	postsCountStr := r.URL.Query().Get("posts")
	postsCount := 10 // Default number of posts
	if postsCountStr != "" {
		var err error
		postsCount, err = strconv.Atoi(postsCountStr)
		if err != nil || postsCount <= 0 {
			http.Error(w, "Invalid posts value. Must be a positive integer.", http.StatusBadRequest)
			return
		}
	}

	timelineReadReq := timelineTypes.ReadReq{
		UserId:        userId,
		UserTl:        userTl,
		EarlUnixMilli: earlUnixMilli,
		Posts:         postsCount,
		SendUnixMilli: time.Now().UnixMilli(),
	}

	timelineResp, err := h.TimelineService.ReadTimeline(context.Background(), timelineReadReq)
	if err != nil {
		logger.Printf("TimelineHandler: Error reading timeline: %v", err)
		http.Error(w, "Failed to read timeline", http.StatusInternalServerError)
		return
	}

	if len(timelineResp.PostIds) == 0 {
		writeJSONResponse(w, http.StatusOK, []postTypes.Post{}) // Return empty list if no posts
		return
	}

	// Fetch full post details for the IDs obtained from the timeline
	postReadReq := postTypes.ReadPostReq{
		PostIds:       timelineResp.PostIds,
		SendUnixMilli: time.Now().UnixMilli(),
	}
	postReadResp, err := h.PostService.ReadPosts(context.Background(), postReadReq)
	if err != nil {
		logger.Printf("TimelineHandler: Error reading posts: %v", err)
		http.Error(w, "Failed to read posts for timeline", http.StatusInternalServerError)
		return
	}

	// Convert map to slice for ordered response, though order isn't guaranteed by map iteration.
	// The timelineResp.PostIds provides the order.
	var postsDetails []postTypes.Post
	for _, postId := range timelineResp.PostIds {
		if post, ok := postReadResp.Posts[postId]; ok {
			postsDetails = append(postsDetails, post)
		}
	}

	writeJSONResponse(w, http.StatusOK, postsDetails)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("timeline"), float64(time.Since(startTime).Milliseconds()))
}

// RegisterHandler handles new user registration
func (h *APIHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	logger.Println("RegisterHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req userTypes.RegisterReq // Use type from types/user package
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("RegisterHandler json.Unmarshal err: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// The userTypes.RegisterReq matches what the userservice expects.
	resp, err := h.UserService.Register(context.Background(), req)
	if err != nil {
		logger.Printf("RegisterHandler error calling UserService.Register: %v", err)
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("register"), float64(time.Since(startTime).Milliseconds()))
}

// LoginHandler handles user login
func (h *APIHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	logger.Println("LoginHandler received request")

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

	var req userTypes.LoginReq // Use type from types/user package
	// For Login, the frontend API request structure matches the userservice request structure.
	// In a real app, the frontend might receive username/password, then the LoginHandler
	// would construct the userTypes.LoginReq. Here, we assume they are the same for simplicity.
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Printf("LoginHandler json.Unmarshal err: %s", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	resp, err := h.UserService.Login(context.Background(), req)
	if err != nil {
		// This path might not be hit often with the current stubbed Login always succeeding.
		logger.Printf("LoginHandler error calling UserService.Login: %v", err)
		http.Error(w, "Login failed", http.StatusInternalServerError) // Or http.StatusUnauthorized
		return
	}

	writeJSONResponse(w, http.StatusOK, resp)
	util.ObserveHist(e2eReqLatHist.WithLabelValues("login"), float64(time.Since(startTime).Milliseconds()))
}

// @monolift extractionType=lambda thresholdType = maxConcurrentInvocations threholdValue = 10
func expensiveProcessingStep(imageData []byte) error {
	return nil
}
