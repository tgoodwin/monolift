package timelineservice

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	dapr "github.com/dapr/go-sdk/client"
	"github.com/tgoodwin/monolift/demo/monolith/database"
	"github.com/tgoodwin/monolift/demo/monolith/postservice"
	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
	timelineTypes "github.com/tgoodwin/monolift/demo/monolith/types/timeline"
	"github.com/tgoodwin/monolift/demo/monolith/util"

	"github.com/pkg/errors"
)

var logger = log.New(os.Stdout, "monolith-timelineservice: ", log.LstdFlags|log.Lshortfile)

const (
	userTimelineStoreName = "timeline-store"
	homeTimelineStoreName = "timeline-store"
	maxTimelineSize       = 1000 // Max number of posts in a timeline (like original Dapr version)
	maxRetries            = 5    // Max retries for optimistic concurrency
)

// TimelinePostEntry stores a post ID and its creation timestamp for sorting.
type TimelinePostEntry struct {
	PostId        string `json:"post_id"`
	PostTimestamp int64  `json:"post_timestamp"` // Unix milliseconds
}

// Service defines the interface for timeline-related operations.
type Service interface {
	// ReadTimeline retrieves post IDs for a user's timeline.
	// Full post content fetching might be orchestrated by the caller (e.g., frontend)
	// or this service could optionally depend on postservice to do it.
	// For now, it returns post IDs as per the original timeline-read-service.
	ReadTimeline(ctx context.Context, req timelineTypes.ReadReq) (timelineTypes.ReadResp, error)

	// UpdateTimeline adds or removes a post from a user's timeline.
	// This replaces the pub/sub mechanism of the original timeline-write-service.
	UpdateTimeline(ctx context.Context, req timelineTypes.UpdateReq) (timelineTypes.UpdateResp, error)
}

type timelineUpdateJob struct {
	Req    timelineTypes.UpdateReq
	PostId string // For logging
	UserId string // For logging
}

type service struct {
	db                 database.Store
	socialGraphService socialgraph.Service
	postService        postservice.Service

	numWorkers int // Number of worker goroutines
	bufferSize int // Buffer size for job channel

	jobChan chan timelineUpdateJob // For worker pool
}

// NewService creates a new timeline service instance.
func NewService(store database.Store, sgService socialgraph.Service, pService postservice.Service, numWorkers int, bufferSize int) Service {
	s := service{
		db:                 store,
		socialGraphService: sgService,
		postService:        pService,

		numWorkers: numWorkers,
		bufferSize: bufferSize,

		jobChan: make(chan timelineUpdateJob, bufferSize), // Buffer size can be adjusted
	}

	// start workers
	for i := 0; i < s.numWorkers; i++ {
		go s.worker(i)
	}

	return &s
}

// --- Key generation functions ---
func userTimelineKey(userId string) string {
	return userId + "-user"
}

func homeTimelineKey(userId string) string {
	return userId + "-home"
}

func (s *service) worker(id int) {
	logger.Printf("Timeline worker %d started.", id)
	for job := range s.jobChan {
		// serviceCallStart := time.Now()
		_, err := s.handleUpdateTimeline(context.Background(), job.Req)

		// util.ObserveHist(serviceCallLatHist.WithLabelValues("timelineservice", "UpdateTimeline"), float64(time.Since(serviceCallStart).Milliseconds()))

		if err != nil {
			logger.Printf("Timeline worker %d: failed to update timeline for user %s, post %s: %v", id, job.UserId, job.PostId, err)
		}
	}
}

// --- Helper to update a single timeline (user or home) ---
func (s *service) updateSpecificTimeline(ctx context.Context, storeName, timelineKey, postId string, postTimestamp int64, add bool) error {
	var currentEntries []TimelinePostEntry
	var etag string

	for i := 0; i < maxRetries; i++ {
		opStartTime := time.Now()
		item, err := s.db.GetState(ctx, storeName, timelineKey)
		util.ObserveHist(readStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if err != nil {
			return errors.Wrapf(err, "failed to get timeline from store %s for key %s", storeName, timelineKey)
		}

		if item == nil || item.Value == nil {
			currentEntries = []TimelinePostEntry{}
			etag = "" // ETag is empty for new items
		} else {
			if err := database.Unmarshal(item.Value, &currentEntries); err != nil {
				return errors.Wrapf(err, "failed to unmarshal timeline from store %s for key %s", storeName, timelineKey)
			}
			etag = item.Etag
		}

		if add {
			// Avoid duplicates
			found := false
			for _, entry := range currentEntries {
				if entry.PostId == postId {
					found = true
					break
				}
			}
			if !found {
				currentEntries = append(currentEntries, TimelinePostEntry{PostId: postId, PostTimestamp: postTimestamp})
				// Sort by timestamp descending (newest first)
				sort.Slice(currentEntries, func(i, j int) bool {
					return currentEntries[i].PostTimestamp > currentEntries[j].PostTimestamp
				})
				// Trim if exceeds max size
				if len(currentEntries) > maxTimelineSize {
					currentEntries = currentEntries[:maxTimelineSize]
				}
			}
		} else { // Remove
			var newEntries []TimelinePostEntry
			for _, entry := range currentEntries {
				if entry.PostId != postId {
					newEntries = append(newEntries, entry)
				}
			}
			currentEntries = newEntries
		}

		updatedData, err := database.Marshal(currentEntries)
		if err != nil {
			return errors.Wrapf(err, "failed to marshal updated timeline for store %s, key %s", storeName, timelineKey)
		}

		opStartTime = time.Now()
		// Use the more performant SaveBulkState
		itemToSave := &dapr.SetStateItem{
			Key:   timelineKey,
			Value: updatedData,
		}
		if etag != "" {
			itemToSave.Etag = &dapr.ETag{Value: etag}
		}
		saveErr := s.db.SaveBulkState(ctx, storeName, itemToSave)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if saveErr == nil {
			return nil // Success
		}

		// Note: Dapr's SaveBulkState might not return specific ETag mismatch errors.
		// The error handling here might need to be adjusted based on the actual errors returned by the Dapr client.
		// We'll keep the retry logic for now, assuming transient failures or potential transaction conflicts.
		if stdErrors.Is(saveErr, database.ErrETagMismatch) || stdErrors.Is(saveErr, database.ErrTransactionFailed) {
			logger.Printf("updateSpecificTimeline: concurrency error for store %s, key %s, retrying (%d/%d): %v", storeName, timelineKey, i+1, maxRetries, saveErr)
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
			continue
		}
		return errors.Wrapf(saveErr, "failed to save updated timeline to store %s for key %s", storeName, timelineKey)
	}
	return fmt.Errorf("failed to update timeline for store %s, key %s after %d retries", storeName, timelineKey, maxRetries)
}

func (s *service) ReadTimeline(ctx context.Context, req timelineTypes.ReadReq) (timelineTypes.ReadResp, error) {
	opStartTime := time.Now()
	readTimelineReqCtr.Inc()
	logger.Printf("ReadTimeline called for UserId: %s, UserTl: %t, EarlUnixMilli: %d, Posts: %d", req.UserId, req.UserTl, req.EarlUnixMilli, req.Posts)

	storeName := homeTimelineStoreName
	timelineKey := homeTimelineKey(req.UserId)
	if req.UserTl {
		storeName = userTimelineStoreName
		timelineKey = userTimelineKey(req.UserId)
	}

	storeReadStartTime := time.Now()
	item, err := s.db.GetState(ctx, storeName, timelineKey)
	util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
	if err != nil {
		return timelineTypes.ReadResp{}, errors.Wrapf(err, "ReadTimeline: failed to get timeline from store %s for key %s", storeName, timelineKey)
	}

	var timelineEntries []TimelinePostEntry
	if item != nil && item.Value != nil {
		if err := database.Unmarshal(item.Value, &timelineEntries); err != nil {
			return timelineTypes.ReadResp{}, errors.Wrapf(err, "ReadTimeline: failed to unmarshal timeline from store %s for key %s", storeName, timelineKey)
		}
	} else {
		// No timeline found, return empty
		util.ObserveHist(reqLatHist, float64(time.Since(opStartTime).Milliseconds()))
		return timelineTypes.ReadResp{SendUnixMilli: time.Now().UnixMilli(), PostIds: []string{}}, nil
	}

	// Filter and paginate
	// Timelines are stored newest first. req.EarlUnixMilli means "posts older than or equal to this timestamp"
	// So we iterate and collect posts whose timestamp is <= req.EarlUnixMilli
	var resultPostIds []string
	for _, entry := range timelineEntries {
		if entry.PostTimestamp <= req.EarlUnixMilli {
			resultPostIds = append(resultPostIds, entry.PostId)
			if len(resultPostIds) >= req.Posts {
				break
			}
		}
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return timelineTypes.ReadResp{SendUnixMilli: time.Now().UnixMilli(), PostIds: resultPostIds}, nil
}

func (s *service) UpdateTimeline(ctx context.Context, req timelineTypes.UpdateReq) (timelineTypes.UpdateResp, error) {
	// create a timeline job and enqueue it
	job := timelineUpdateJob{
		Req:    req,
		PostId: req.PostId,
		UserId: req.UserId,
	}
	s.jobChan <- job
	return timelineTypes.UpdateResp{}, nil
}

func (s *service) handleUpdateTimeline(ctx context.Context, req timelineTypes.UpdateReq) (timelineTypes.UpdateResp, error) {
	opStartTime := time.Now()
	updateTimelineReqCtr.Inc()
	logger.Printf("UpdateTimeline called for PosterId: %s, PostId: %s, Add: %t, PostTimestamp: %d", req.UserId, req.PostId, req.Add, req.ClientUnixMilli)

	// 1. Update the poster's own user timeline
	userTlKey := userTimelineKey(req.UserId)
	err := s.updateSpecificTimeline(ctx, userTimelineStoreName, userTlKey, req.PostId, req.ClientUnixMilli, req.Add)
	if err != nil {
		logger.Printf("UpdateTimeline: failed to update user timeline for user %s: %v", req.UserId, err)
		// Continue to update home timelines even if user timeline fails, or decide on stricter error handling.
	}

	// 2. Update home timelines of followers (only if adding a post)
	if req.Add {
		followersReq := socialgraph.GetReq{UserIds: []string{req.UserId}, SendUnixMilli: time.Now().UnixMilli()}
		followersResp, err := s.socialGraphService.GetFollowers(ctx, followersReq)
		if err != nil {
			logger.Printf("UpdateTimeline: failed to get followers for user %s: %v. Skipping home timeline updates.", req.UserId, err)
		} else if followerMap, ok := followersResp.FollowerIds[req.UserId]; ok {
			start := time.Now()
			for _, followerId := range followerMap {
				homeTlKey := homeTimelineKey(followerId)
				err := s.updateSpecificTimeline(ctx, homeTimelineStoreName, homeTlKey, req.PostId, req.ClientUnixMilli, req.Add)
				if err != nil {
					logger.Printf("UpdateTimeline: failed to update home timeline for follower %s (of user %s): %v", followerId, req.UserId, err)
				}
			}
			fmt.Println("elapsed time for updating followers' home timelines:", time.Since(start))
			// util.ObserveHist(followersUpdateLatHist, float64(time.Since(start).Milliseconds()))
		}
	}
	// Note: Deleting from followers' home timelines upon post deletion is more complex
	// and was not explicitly handled by the original Dapr timeline-write-service's simple pub/sub.
	// For simplicity and alignment, we only fan-out additions. Deletions affect the user's own timeline.

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return timelineTypes.UpdateResp{SendUnixMilli: time.Now().UnixMilli()}, nil
}
