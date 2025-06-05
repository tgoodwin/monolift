package postservice

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"dapr-apps/socialnet/monolith/database"
	"dapr-apps/socialnet/monolith/types/post"
	"dapr-apps/socialnet/monolith/util"

	"github.com/pkg/errors"
)

var logger = log.New(os.Stdout, "monolith-postservice: ", log.LstdFlags|log.Lshortfile)

const (
	postContentStoreName  = "post_content"
	postMetaStoreName     = "post_meta"
	postCommentsStoreName = "post_comments"
	postUpvotesStoreName  = "post_upvotes"
	maxRetries            = 5 // Max retries for optimistic concurrency
)

// Service defines the interface for post-related operations.
type Service interface {
	SavePost(ctx context.Context, req post.SavePostReq) (post.UpdatePostResp, error)
	DeletePost(ctx context.Context, req post.DelPostReq) (post.UpdatePostResp, error)
	UpdateMeta(ctx context.Context, req post.MetaReq) (post.UpdatePostResp, error)
	AddComment(ctx context.Context, req post.CommentReq) (post.UpdatePostResp, error)
	UpvotePost(ctx context.Context, req post.UpvoteReq) (post.UpdatePostResp, error)
	ReadPosts(ctx context.Context, req post.ReadPostReq) (post.ReadPostResp, error)
}

type service struct {
	db database.Store
}

// NewService creates a new post service instance.
func NewService(store database.Store) Service {
	return &service{
		db: store,
	}
}

// --- Key generation functions (from original post/main.go) ---
// These will be used in Phase 3 with the storage implementation.

// contKey returns the content store key given a post id
func contKey(postId string) string {
	return postId + "-ct"
}

// metaKey returns the metadata store key given a post id
func metaKey(postId string) string {
	return postId + "-me"
}

// commKey returns the comment store key given a post id
func commKey(postId string) string {
	return postId + "-cm"
}

// upvoteKey returns the upvote store key given a post id
func upvoteKey(postId string) string {
	return postId + "-up"
}

// --- Interface Implementations (Stubs for now) ---

func (s *service) SavePost(ctx context.Context, req post.SavePostReq) (post.UpdatePostResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("SavePost called with PostId: %s", req.PostId)

	// 1. Save Post Content
	contentData, err := database.Marshal(req.PostCont)
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to marshal post content")
	}
	storeWriteStartTime := time.Now()
	_, err = s.db.SaveState(ctx, postContentStoreName, contKey(req.PostId), contentData, nil)
	util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to save post content")
	}

	// 2. Initialize and Save Post Metadata
	initialMeta := post.PostMeta{
		Sentiment: "",                      // Default empty
		Objects:   make(map[string]string), // Default empty map
		// Views and UpvotesCount are not part of PostMeta
	}
	metaData, err := database.Marshal(initialMeta)
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to marshal initial post meta")
	}
	storeWriteStartTime = time.Now()
	_, err = s.db.SaveState(ctx, postMetaStoreName, metaKey(req.PostId), metaData, nil)
	util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to save initial post meta")
	}

	// 3. Initialize and Save Empty Comments List
	commentsData, err := database.Marshal([]post.Comment{})
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to marshal empty comments")
	}
	storeWriteStartTime = time.Now()
	_, err = s.db.SaveState(ctx, postCommentsStoreName, commKey(req.PostId), commentsData, nil)
	util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to save empty comments")
	}

	// 4. Initialize and Save Empty Upvotes List
	upvotesData, err := database.Marshal([]string{}) // List of user IDs who upvoted
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to marshal empty upvotes")
	}
	storeWriteStartTime = time.Now()
	_, err = s.db.SaveState(ctx, postUpvotesStoreName, upvoteKey(req.PostId), upvotesData, nil)
	util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
	if err != nil {
		return post.UpdatePostResp{}, errors.Wrap(err, "SavePost: failed to save empty upvotes")
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds()) // Total time for the operation
	util.ObserveHist(saveReqLatHist, serviceProcessingDuration)

	return post.UpdatePostResp{SendUnixMilli: time.Now().UnixMilli()}, nil
}

func (s *service) DeletePost(ctx context.Context, req post.DelPostReq) (post.UpdatePostResp, error) {
	// startTime := time.Now()
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("DeletePost called with PostId: %s", req.PostId)

	keysAndStores := []struct {
		storeName string
		key       string
	}{
		{postContentStoreName, contKey(req.PostId)},
		{postMetaStoreName, metaKey(req.PostId)},
		{postCommentsStoreName, commKey(req.PostId)},
		{postUpvotesStoreName, upvoteKey(req.PostId)},
	}

	for _, item := range keysAndStores {
		storeWriteStartTime := time.Now()
		err := s.db.DeleteState(ctx, item.storeName, item.key, nil) // No ETag needed for forceful delete
		util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
		if err != nil {
			// Log error but continue trying to delete other parts
			logger.Printf("DeletePost: failed to delete key %s from store %s: %v", item.key, item.storeName, err)
		}
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(updateReqLatHist, serviceProcessingDuration)
	return post.UpdatePostResp{SendUnixMilli: time.Now().UnixMilli()}, nil
}

func (s *service) UpdateMeta(ctx context.Context, req post.MetaReq) (post.UpdatePostResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("UpdateMeta called for PostId: %s", req.PostId)

	key := metaKey(req.PostId)
	var currentMeta post.PostMeta
	var etag *string

	for i := 0; i < maxRetries; i++ {
		storeReadStartTime := time.Now()
		item, err := s.db.GetState(ctx, postMetaStoreName, key)
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "UpdateMeta: failed to get post meta")
		}
		if item == nil || item.Value == nil {
			return post.UpdatePostResp{}, fmt.Errorf("UpdateMeta: post meta not found for PostId: %s", req.PostId)
		}

		if err := database.Unmarshal(item.Value, &currentMeta); err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "UpdateMeta: failed to unmarshal post meta")
		}
		etag = &item.Etag

		// Apply updates
		// UpdateMeta is for sentiment and objects, as per original dapr post.MetaReq
		currentMeta.Sentiment = req.Sentiment
		currentMeta.Objects = req.Objects

		updatedMetaData, err := database.Marshal(currentMeta)
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "UpdateMeta: failed to marshal updated post meta")
		}

		storeWriteStartTime := time.Now()
		newEtag, err := s.db.SaveState(ctx, postMetaStoreName, key, updatedMetaData, etag)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
		if err == nil {
			logger.Printf("UpdateMeta successful for PostId: %s, new ETag: %s", req.PostId, newEtag)
			serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
			util.ObserveHist(updateReqLatHist, serviceProcessingDuration)
			return post.UpdatePostResp{SendUnixMilli: time.Now().UnixMilli()}, nil
		}

		if strings.Contains(err.Error(), "ETag mismatch") { // Check if it's an ETag error
			logger.Printf("UpdateMeta: ETag mismatch for PostId %s, retrying (%d/%d)", req.PostId, i+1, maxRetries)
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond) // Exponential backoff
			continue
		}
		return post.UpdatePostResp{}, errors.Wrap(err, "UpdateMeta: failed to save updated post meta")
	}

	return post.UpdatePostResp{}, fmt.Errorf("UpdateMeta: failed to update post meta after %d retries for PostId: %s", maxRetries, req.PostId)
}

func (s *service) AddComment(ctx context.Context, req post.CommentReq) (post.UpdatePostResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("AddComment called for PostId: %s, Comment User: %s", req.PostId, req.Comm.UserId)

	key := commKey(req.PostId)
	var currentComments []post.Comment
	var etag *string

	for i := 0; i < maxRetries; i++ {
		storeReadStartTime := time.Now()
		item, err := s.db.GetState(ctx, postCommentsStoreName, key)
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "AddComment: failed to get comments")
		}
		if item == nil || item.Value == nil {
			// This shouldn't happen if SavePost initializes it.
			// If it can, initialize with an empty slice here.
			currentComments = []post.Comment{}
			etag = nil
		} else {
			if err := database.Unmarshal(item.Value, &currentComments); err != nil {
				return post.UpdatePostResp{}, errors.Wrap(err, "AddComment: failed to unmarshal comments")
			}
			tempEtag := item.Etag
			etag = &tempEtag
		}

		currentComments = append(currentComments, req.Comm)

		updatedCommentsData, err := database.Marshal(currentComments)
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "AddComment: failed to marshal updated comments")
		}

		storeWriteStartTime := time.Now()
		newEtag, err := s.db.SaveState(ctx, postCommentsStoreName, key, updatedCommentsData, etag)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
		if err == nil {
			logger.Printf("AddComment successful for PostId: %s, new ETag: %s", req.PostId, newEtag)
			serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
			util.ObserveHist(updateReqLatHist, serviceProcessingDuration)
			return post.UpdatePostResp{SendUnixMilli: time.Now().UnixMilli()}, nil
		}

		if strings.Contains(err.Error(), "ETag mismatch") {
			logger.Printf("AddComment: ETag mismatch for PostId %s, retrying (%d/%d)", req.PostId, i+1, maxRetries)
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
			continue
		}
		return post.UpdatePostResp{}, errors.Wrap(err, "AddComment: failed to save updated comments")
	}

	return post.UpdatePostResp{}, fmt.Errorf("AddComment: failed to add comment after %d retries for PostId: %s", maxRetries, req.PostId)
}

func (s *service) UpvotePost(ctx context.Context, req post.UpvoteReq) (post.UpdatePostResp, error) {
	opStartTime := time.Now()
	updateCtr.Inc()
	logger.Printf("UpvotePost called for PostId: %s, User: %s", req.PostId, req.UserId)

	upvotersKey := upvoteKey(req.PostId)
	var currentUpvoters []string
	var etag *string

	for i := 0; i < maxRetries; i++ {
		storeReadStartTime := time.Now()
		item, err := s.db.GetState(ctx, postUpvotesStoreName, upvotersKey)
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "UpvotePost: failed to get upvoters list")
		}
		if item == nil || item.Value == nil {
			currentUpvoters = []string{}
			etag = nil
		} else {
			if err := database.Unmarshal(item.Value, &currentUpvoters); err != nil {
				return post.UpdatePostResp{}, errors.Wrap(err, "UpvotePost: failed to unmarshal upvoters list")
			}
			tempEtag := item.Etag
			etag = &tempEtag
		}

		// Logic to add/remove user from upvoters (idempotent)
		found := false
		var newUpvoters []string
		for _, u := range currentUpvoters {
			if u == req.UserId {
				found = true
			} else {
				newUpvoters = append(newUpvoters, u)
			}
		}
		if found { // User already upvoted, so this is an "un-upvote"
			currentUpvoters = newUpvoters
		} else { // User has not upvoted, so add them
			currentUpvoters = append(currentUpvoters, req.UserId)
		}

		updatedUpvotersData, err := database.Marshal(currentUpvoters)
		if err != nil {
			return post.UpdatePostResp{}, errors.Wrap(err, "UpvotePost: failed to marshal updated upvoters list")
		}

		storeWriteStartTime := time.Now()
		newEtag, err := s.db.SaveState(ctx, postUpvotesStoreName, upvotersKey, updatedUpvotersData, etag)
		util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
		if err == nil {
			logger.Printf("UpvotePost: upvoters list updated for PostId: %s, User: %s. New ETag: %s", req.PostId, req.UserId, newEtag)
			// No update to PostMeta for UpvotesCount, as it's derived from len(Upvotes)
			serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
			util.ObserveHist(updateReqLatHist, serviceProcessingDuration)
			return post.UpdatePostResp{SendUnixMilli: time.Now().UnixMilli()}, nil
		}

		if strings.Contains(err.Error(), "ETag mismatch") {
			logger.Printf("UpvotePost: ETag mismatch for PostId %s, retrying (%d/%d)", req.PostId, i+1, maxRetries)
			time.Sleep(time.Duration(20*(i+1)) * time.Millisecond)
			continue
		}
		return post.UpdatePostResp{}, errors.Wrap(err, "UpvotePost: failed to save updated upvoters list")
	}

	return post.UpdatePostResp{}, fmt.Errorf("UpvotePost: failed to upvote post after %d retries for PostId: %s", maxRetries, req.PostId)
}

func (s *service) ReadPosts(ctx context.Context, req post.ReadPostReq) (post.ReadPostResp, error) {
	opStartTime := time.Now()
	readCtr.Inc()
	logger.Printf("ReadPosts called for %d PostIds", len(req.PostIds))

	resultPosts := make(map[string]post.Post)

	for _, postId := range req.PostIds {
		var p post.Post
		p.PostId = postId

		// 1. Read Content
		storeReadStartTime := time.Now()
		contentItem, err := s.db.GetState(ctx, postContentStoreName, contKey(postId))
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("ReadPosts: failed to get content for PostId %s: %v. Skipping post.", postId, err)
			continue
		}
		if contentItem == nil || contentItem.Value == nil {
			logger.Printf("ReadPosts: content not found for PostId %s. Skipping post.", postId)
			continue
		}
		if err := database.Unmarshal(contentItem.Value, &p.Content); err != nil {
			logger.Printf("ReadPosts: failed to unmarshal content for PostId %s: %v. Skipping post.", postId, err)
			continue
		}

		// 2. Read Meta
		storeReadStartTime = time.Now()
		metaItem, err := s.db.GetState(ctx, postMetaStoreName, metaKey(postId))
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("ReadPosts: failed to get meta for PostId %s: %v. Using zero meta.", postId, err)
		} else if metaItem != nil && metaItem.Value != nil {
			if err := database.Unmarshal(metaItem.Value, &p.Meta); err != nil { // p.Meta is of type post.PostMeta
				logger.Printf("ReadPosts: failed to unmarshal meta for PostId %s: %v. Using zero meta.", postId, err)
			}
		}

		// 3. Read Comments
		storeReadStartTime = time.Now()
		commentsItem, err := s.db.GetState(ctx, postCommentsStoreName, commKey(postId))
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("ReadPosts: failed to get comments for PostId %s: %v. Using empty comments.", postId, err)
		} else if commentsItem != nil && commentsItem.Value != nil {
			if err := database.Unmarshal(commentsItem.Value, &p.Comments); err != nil {
				logger.Printf("ReadPosts: failed to unmarshal comments for PostId %s: %v. Using empty comments.", postId, err)
			}
		}

		// 4. Read Upvotes (list of user IDs)
		storeReadStartTime = time.Now()
		upvotesItem, err := s.db.GetState(ctx, postUpvotesStoreName, upvoteKey(postId))
		util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
		if err != nil {
			logger.Printf("ReadPosts: failed to get upvotes for PostId %s: %v. Using empty upvotes.", postId, err)
		} else if upvotesItem != nil && upvotesItem.Value != nil {
			if err := database.Unmarshal(upvotesItem.Value, &p.Upvotes); err != nil {
				logger.Printf("ReadPosts: failed to unmarshal upvotes for PostId %s: %v. Using empty upvotes.", postId, err)
			}
		}
		// The upvote count is implicitly len(p.Upvotes).
		// The Post struct in types/post/src.go does not have a dedicated UpvotesCount field in its Meta.

		resultPosts[postId] = p
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(updateReqLatHist, serviceProcessingDuration)

	serviceProcessingDuration = float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(readReqLatHist, serviceProcessingDuration)

	return post.ReadPostResp{SendUnixMilli: time.Now().UnixMilli(), Posts: resultPosts}, nil
}
