package socialgraph

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"dapr-apps/socialnet/monolith/database"
	social "dapr-apps/socialnet/monolith/types/socialgraph"
	"dapr-apps/socialnet/monolith/util"

	"github.com/pkg/errors"
)

var logger = log.New(os.Stdout, "monolith-socialgraph: ", log.LstdFlags|log.Lshortfile)

const (
	followeesStoreName = "socialgraph_followees"
	followersStoreName = "socialgraph_followers"
	maxRetries         = 5    // Max retries for optimistic concurrency
	maxFollows         = 2000 // Max number of followees/followers (from original Dapr socialgraph)
)

// Service defines the interface for social graph operations.
type Service interface {
	GetFollowees(ctx context.Context, req social.GetReq) (social.GetFollowResp, error)
	GetFollowers(ctx context.Context, req social.GetReq) (social.GetFollowerResp, error)
	GetRecommendations(ctx context.Context, req social.GetRecmdReq) (social.GetRecmdResp, error)
	Follow(ctx context.Context, req social.FollowReq) (social.UpdateResp, error)
	Unfollow(ctx context.Context, req social.UnfollowReq) (social.UpdateResp, error) // Corrected return type
}

type service struct {
	db database.Store
}

// NewService creates a new social graph service instance.
func NewService(store database.Store) Service {
	return &service{
		db: store,
	}
}

// followKey generates the key for a user's followee list.
func followKey(userId string) string {
	return userId // storeName provides namespacing
}

// followerKey generates the key for a user's follower list.
func followerKey(userId string) string {
	return userId // storeName provides namespacing
}

// updateFollowList is a helper function for updating a follow/follower list in the store.
// It handles adding or removing a value from a list of strings with ETag-based optimistic concurrency.
func (s *service) updateFollowList(ctx context.Context, storeName, key, valueToUpdate string, add bool, maxListSize int) error {
	var currentList []string
	var etag *string

	for i := 0; i < maxRetries; i++ {
		opStartTime := time.Now()
		item, err := s.db.GetState(ctx, storeName, key)
		util.ObserveHist(readStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if err != nil {
			return errors.Wrapf(err, "updateFollowList: failed to get list from store %s for key %s", storeName, key)
		}

		if item == nil || item.Value == nil {
			currentList = []string{}
			etag = nil
		} else {
			if err := database.Unmarshal(item.Value, &currentList); err != nil {
				return errors.Wrapf(err, "updateFollowList: failed to unmarshal list from store %s for key %s", storeName, key)
			}
			tempEtag := item.Etag
			etag = &tempEtag
		}

		found := false
		foundIdx := -1
		for idx, val := range currentList {
			if val == valueToUpdate {
				found = true
				foundIdx = idx
				break
			}
		}

		if add {
			if !found {
				currentList = append(currentList, valueToUpdate)
				if maxListSize > 0 && len(currentList) > maxListSize {
					// Trim from the beginning if exceeding max size (oldest entries)
					// This matches the original Dapr socialgraph logic (newest are appended)
					currentList = currentList[len(currentList)-maxListSize:]
				}
			} else {
				return nil // Already present, no update needed
			}
		} else { // Remove
			if found {
				currentList = append(currentList[:foundIdx], currentList[foundIdx+1:]...)
			} else {
				return nil // Not found, no update needed
			}
		}

		updatedData, err := database.Marshal(currentList)
		if err != nil {
			return errors.Wrapf(err, "updateFollowList: failed to marshal updated list for store %s, key %s", storeName, key)
		}

		opStartTime = time.Now()
		_, saveErr := s.db.SaveState(ctx, storeName, key, updatedData, etag)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(opStartTime).Milliseconds()))
		if saveErr == nil {
			return nil // Success
		}

		if strings.Contains(saveErr.Error(), "ETag mismatch") {
			logger.Printf("updateFollowList: ETag mismatch for store %s, key %s, retrying (%d/%d)", storeName, key, i+1, maxRetries)
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
			continue
		}
		return errors.Wrapf(saveErr, "updateFollowList: failed to save updated list to store %s for key %s", storeName, key)
	}
	return fmt.Errorf("updateFollowList: failed to update list for store %s, key %s after %d retries", storeName, key, maxRetries)
}

// --- Interface Implementations (Stubs for now) ---

func (s *service) GetFollowees(ctx context.Context, req social.GetReq) (social.GetFollowResp, error) {
	opStartTime := time.Now()
	readCtr.Inc()
	logger.Printf("GetFollowees called for UserIds: %v", req.UserIds)

	respMap := make(map[string][]string)
	for _, userId := range req.UserIds {
		key := followKey(userId)
		storeReadStartTime := time.Now()
		item, err := s.db.GetState(ctx, followeesStoreName, key)
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("GetFollowees: error getting followees for user %s: %v", userId, err)
			respMap[userId] = []string{} // Return empty list on error for this user
			continue
		}
		if item != nil && item.Value != nil {
			var followeeList []string
			if err := database.Unmarshal(item.Value, &followeeList); err != nil {
				logger.Printf("GetFollowees: error unmarshalling followees for user %s: %v", userId, err)
				respMap[userId] = []string{}
			} else {
				respMap[userId] = followeeList
			}
		} else {
			respMap[userId] = []string{} // No entry found
		}
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return social.GetFollowResp{SendUnixMilli: time.Now().UnixMilli(), FollowIds: respMap}, nil
}

func (s *service) GetFollowers(ctx context.Context, req social.GetReq) (social.GetFollowerResp, error) {
	opStartTime := time.Now()
	readCtr.Inc()
	logger.Printf("GetFollowers called for UserIds: %v", req.UserIds)

	respMap := make(map[string][]string)
	for _, userId := range req.UserIds {
		key := followerKey(userId)
		storeReadStartTime := time.Now()
		item, err := s.db.GetState(ctx, followersStoreName, key)
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("GetFollowers: error getting followers for user %s: %v", userId, err)
			respMap[userId] = []string{}
			continue
		}
		if item != nil && item.Value != nil {
			var followerList []string
			if err := database.Unmarshal(item.Value, &followerList); err != nil {
				logger.Printf("GetFollowers: error unmarshalling followers for user %s: %v", userId, err)
				respMap[userId] = []string{}
			} else {
				respMap[userId] = followerList
			}
		} else {
			respMap[userId] = []string{}
		}
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return social.GetFollowerResp{SendUnixMilli: time.Now().UnixMilli(), FollowerIds: respMap}, nil
}

func (s *service) GetRecommendations(ctx context.Context, req social.GetRecmdReq) (social.GetRecmdResp, error) {
	opStartTime := time.Now()
	recmdCtr.Inc()
	logger.Printf("GetRecommendations called for UserIds: %v", req.UserIds)

	// The original recommendation logic is essentially the same as GetFollowees for this benchmark.
	// We'll call GetFollowees internally.
	getFolloweesReq := social.GetReq{UserIds: req.UserIds, SendUnixMilli: req.SendUnixMilli}
	followeesResp, err := s.GetFollowees(ctx, getFolloweesReq) // This will record its own reqLatHist
	if err != nil {
		return social.GetRecmdResp{}, errors.Wrap(err, "GetRecommendations: failed to get followees for recommendations")
	}

	// Latency calculation for recommendations specifically
	// The original Dapr service had a specific way to measure this if req.Record was true.
	// Here, we measure the time spent in this function *after* GetFollowees.
	// The `req.Latency` from the original Dapr version was for a different purpose (passing through upstream latency).
	// For simplicity, we'll measure the local processing time for the recommendation aspect.
	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	if req.Record {
		util.ObserveHist(recmdLatHist, serviceProcessingDuration)
	}

	return social.GetRecmdResp{SendUnixMilli: time.Now().UnixMilli(), FollowIds: followeesResp.FollowIds, Latency: time.Now().UnixMilli() - req.SendUnixMilli}, nil
}

func (s *service) Follow(ctx context.Context, req social.FollowReq) (social.UpdateResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("Follow called: User %s wants to follow %s", req.UserId, req.FollowId)

	if req.UserId == req.FollowId {
		logger.Printf("Follow: User %s cannot follow themselves. Operation skipped.", req.UserId)
		// Original Dapr service allowed self-follow for simplicity in user registration.
		// If strict "no self-follow" is desired *except* for registration,
		// userservice.Register would be the only place calling Follow with UserId == FollowId.
		// For now, we allow it as per original behavior.
	}

	// 1. Add FollowId to UserId's followee list
	userKey := followKey(req.UserId)
	err := s.updateFollowList(ctx, followeesStoreName, userKey, req.FollowId, true, maxFollows)
	if err != nil {
		return social.UpdateResp{}, errors.Wrapf(err, "Follow: failed to update followee list for user %s", req.UserId)
	}

	// 2. Add UserId to FollowId's follower list
	followeeKey := followerKey(req.FollowId)
	err = s.updateFollowList(ctx, followersStoreName, followeeKey, req.UserId, true, maxFollows)
	if err != nil {
		// Potentially inconsistent state. Consider compensating action or logging severity.
		return social.UpdateResp{}, errors.Wrapf(err, "Follow: failed to update follower list for user %s", req.FollowId)
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return social.UpdateResp{SendUnixMilli: time.Now().UnixMilli()}, nil
}

func (s *service) Unfollow(ctx context.Context, req social.UnfollowReq) (social.UpdateResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("Unfollow called: User %s wants to unfollow %s", req.UserId, req.UnfollowId)

	// 1. Remove UnfollowId from UserId's followee list
	userKey := followKey(req.UserId)
	err := s.updateFollowList(ctx, followeesStoreName, userKey, req.UnfollowId, false, maxFollows)
	if err != nil {
		return social.UpdateResp{}, errors.Wrapf(err, "Unfollow: failed to update followee list for user %s", req.UserId)
	}

	// 2. Remove UserId from UnfollowId's follower list
	unfolloweeKey := followerKey(req.UnfollowId)
	err = s.updateFollowList(ctx, followersStoreName, unfolloweeKey, req.UserId, false, maxFollows)
	if err != nil {
		return social.UpdateResp{}, errors.Wrapf(err, "Unfollow: failed to update follower list for user %s", req.UnfollowId)
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return social.UpdateResp{SendUnixMilli: time.Now().UnixMilli()}, nil
}
