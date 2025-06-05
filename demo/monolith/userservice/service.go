package userservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"time"

	"dapr-apps/socialnet/monolith/database"
	"dapr-apps/socialnet/monolith/socialgraph"
	socialGraphTypes "dapr-apps/socialnet/monolith/types/socialgraph"
	userTypes "dapr-apps/socialnet/monolith/types/user"
	"dapr-apps/socialnet/monolith/util"

	"github.com/pkg/errors"
)

var logger = log.New(os.Stdout, "monolith-userservice: ", log.LstdFlags|log.Lshortfile)

const (
	userCredentialsStoreName = "user_credentials"
	// maxRetries could be added if optimistic concurrency was needed for user data, but not for simple credential storage.
)

// Service defines the interface for user-related operations.
// @monolift trigger=CPU threshold=0.5
type Service interface {
	Register(ctx context.Context, req userTypes.RegisterReq) (userTypes.RegisterResp, error)
	Login(ctx context.Context, req userTypes.LoginReq) (userTypes.LoginResp, error)
}

type service struct {
	socialGraphService socialgraph.Service
	db                 database.Store // For user credentials
}

// NewService creates a new user service instance.
func NewService(sgService socialgraph.Service, store database.Store) Service {
	return &service{
		socialGraphService: sgService,
		db:                 store,
	}
}

// userCredKey generates the key for storing user credentials.
func userCredKey(userId string) string {
	return userId // Simple key, as storeName provides namespacing
}

// --- Helper for password hashing (from original user/main.go) ---
func hash(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// --- Interface Implementations (Stubs for now) ---

func (s *service) Register(ctx context.Context, req userTypes.RegisterReq) (userTypes.RegisterResp, error) {
	opStartTime := time.Now()
	registerReqCtr.Inc()
	logger.Printf("Register called for UserId: %s", req.UserId)

	// 1. Check if user already exists
	credKey := userCredKey(req.UserId)
	storeReadStartTime := time.Now()
	existingItem, err := s.db.GetState(ctx, userCredentialsStoreName, credKey)
	util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
	if err != nil {
		return userTypes.RegisterResp{}, errors.Wrap(err, "Register: failed to check for existing user")
	}
	if existingItem != nil && existingItem.Value != nil {
		logger.Printf("Register: UserId %s already exists", req.UserId)
		// It's important to return a specific, non-sensitive error here.
		// The original Dapr user service didn't explicitly return an "already exists" error,
		// it would just overwrite. For a monolith, explicit is better.
		return userTypes.RegisterResp{Success: false}, fmt.Errorf("user_id '%s' already registered", req.UserId)
	}

	// 2. Hash password and store credentials
	hashedPassword := hash(req.Password)
	credData, err := database.Marshal(hashedPassword) // Storing the hash as a JSON string
	if err != nil {
		return userTypes.RegisterResp{}, errors.Wrap(err, "Register: failed to marshal hashed password")
	}

	storeWriteStartTime := time.Now()
	_, err = s.db.SaveState(ctx, userCredentialsStoreName, credKey, credData, nil) // No ETag needed for new user
	util.ObserveHist(writeStoreLatHist, float64(time.Since(storeWriteStartTime).Milliseconds()))
	if err != nil {
		return userTypes.RegisterResp{}, errors.Wrap(err, "Register: failed to save user credentials")
	}
	logger.Printf("Register: UserId %s credentials stored", req.UserId)

	// After successful registration, make the user follow themselves.
	followReq := socialGraphTypes.FollowReq{
		UserId:        req.UserId,
		FollowId:      req.UserId,             // User follows themselves
		SendUnixMilli: time.Now().UnixMilli(), // Use current time for this internal request
	}
	if _, followErr := s.socialGraphService.Follow(ctx, followReq); followErr != nil {
		// Log the error, but registration might still be considered successful
		// if the primary goal (storing credentials) succeeded.
		// However, if this fails, the user state is inconsistent.
		// For now, we'll log and proceed, but in a production system, this might warrant a rollback or compensating action.
		logger.Printf("Register: failed to make user %s follow themselves: %v. Registration of credentials succeeded.", req.UserId, followErr)
		// Depending on strictness, you could return an error here:
		// return userTypes.RegisterResp{}, fmt.Errorf("credentials registered, but failed to update social graph: %w", followErr)
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return userTypes.RegisterResp{SendUnixMilli: time.Now().UnixMilli(), Success: true}, nil
}

func (s *service) Login(ctx context.Context, req userTypes.LoginReq) (userTypes.LoginResp, error) {
	opStartTime := time.Now()
	loginReqCtr.Inc()
	logger.Printf("Login attempt for UserId: %s", req.UserId)

	credKey := userCredKey(req.UserId)
	storeReadStartTime := time.Now()
	item, err := s.db.GetState(ctx, userCredentialsStoreName, credKey)
	util.ObserveHist(readStoreLatHist, float64(time.Since(storeReadStartTime).Milliseconds()))
	if err != nil {
		return userTypes.LoginResp{Success: false}, errors.Wrap(err, "Login: database error retrieving credentials")
	}
	if item == nil || item.Value == nil {
		logger.Printf("Login: UserId %s not found or no credentials stored", req.UserId)
		return userTypes.LoginResp{SendUnixMilli: time.Now().UnixMilli(), Success: false}, nil // User not found
	}

	var storedHashedPassword string
	if err := database.Unmarshal(item.Value, &storedHashedPassword); err != nil {
		return userTypes.LoginResp{Success: false}, errors.Wrap(err, "Login: failed to unmarshal stored password")
	}

	providedHashedPassword := hash(req.Password)

	success := storedHashedPassword == providedHashedPassword
	if !success {
		logger.Printf("Login: Password mismatch for UserId %s", req.UserId)
	} else {
		logger.Printf("Login: Successful for UserId %s", req.UserId)
	}

	serviceProcessingDuration := float64(time.Since(opStartTime).Milliseconds())
	util.ObserveHist(reqLatHist, serviceProcessingDuration)
	return userTypes.LoginResp{SendUnixMilli: time.Now().UnixMilli(), Success: success, UserId: req.UserId}, nil
}
