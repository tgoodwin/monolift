package timelineservice

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"dapr-apps/socialnet/monolith/database"
	"dapr-apps/socialnet/monolith/postservice"
	"dapr-apps/socialnet/monolith/socialgraph"
	socialGraphTypes "dapr-apps/socialnet/monolith/types/socialgraph"
	timelineTypes "dapr-apps/socialnet/monolith/types/timeline"
	"dapr-apps/socialnet/monolith/util"

	"github.com/pkg/errors"
)

var logger = log.New(os.Stdout, "monolith-timelineservice: ", log.LstdFlags|log.Lshortfile)

const (
	userTimelineStoreName = "user_timeline"
	homeTimelineStoreName = "home_timeline"
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
	UpdateTimeline(ctx context.Context, req timelineTypes.UpdateReq) error
}

type service struct {
	db                 database.Store
	socialGraphService socialgraph.Service
	postService        postservice.Service
}

// NewService creates a new timeline service instance.
func NewService(store database.Store, sgService socialgraph.Service, pService postservice.Service) Service {
	return &service{
		db:                 store,
		socialGraphService: sgService,
		postService:        pService,
	}
}

// --- Key generation functions ---
func userTimelineKey(userId string) string {
	return userId
}

func homeTimelineKey(userId string) string {
	return userId
}

// --- Helper to update a single timeline (user or home) ---
func (s *service) updateSpecificTimeline(ctx context.Context, storeName, timelineKey, postId string, postTimestamp int64, add bool) error {
	var currentEntries []TimelinePostEntry
	var etag *string

	for i := 0; i < maxRetries; i++ {
		opStartTime := time.Now()
		item, err := s.db.GetState(ctx, storeName, timelineKey)
		util.ObserveHist(readStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if err != nil {
			return errors.Wrapf(err, "failed to get timeline from store %s for key %s", storeName, timelineKey)
		}

		if item == nil || item.Value == nil {
			currentEntries = []TimelinePostEntry{}
			etag = nil
		} else {
			if err := database.Unmarshal(item.Value, &currentEntries); err != nil {
				return errors.Wrapf(err, "failed to unmarshal timeline from store %s for key %s", storeName, timelineKey)
			}
			tempEtag := item.Etag
			etag = &tempEtag
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
		_, saveErr := s.db.SaveState(ctx, storeName, timelineKey, updatedData, etag)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if saveErr == nil {
			return nil // Success
		}

		if strings.Contains(saveErr.Error(), "ETag mismatch") {
			logger.Printf("updateSpecificTimeline: ETag mismatch for store %s, key %s, retrying (%d/%d)", storeName, timelineKey, i+1, maxRetries)
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

func (s *service) UpdateTimeline(ctx context.Context, req timelineTypes.UpdateReq) error {
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
		followersReq := socialGraphTypes.GetReq{UserIds: []string{req.UserId}, SendUnixMilli: time.Now().UnixMilli()}
		followersResp, err := s.socialGraphService.GetFollowers(ctx, followersReq)
		if err != nil {
			logger.Printf("UpdateTimeline: failed to get followers for user %s: %v. Skipping home timeline updates.", req.UserId, err)
		} else if followerMap, ok := followersResp.FollowerIds[req.UserId]; ok {
			for _, followerId := range followerMap {
				homeTlKey := homeTimelineKey(followerId)
				err := s.updateSpecificTimeline(ctx, homeTimelineStoreName, homeTlKey, req.PostId, req.ClientUnixMilli, req.Add)
				if err != nil {
					logger.Printf("UpdateTimeline: failed to update home timeline for follower %s (of user %s): %v", followerId, req.UserId, err)
				}
			}
		}
	}
	// Note: Deleting from followers' home timelines upon post deletion is more complex
	// and was not explicitly handled by the original Dapr timeline-write-service's simple pub/sub.
	// For simplicity and alignment, we only fan-out additions. Deletions affect the user's own timeline.

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return nil
}
