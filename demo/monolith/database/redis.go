package database

import (
	"context"
	"encoding/json"
	"errors" // For errors.Is
	"fmt"

	"github.com/redis/go-redis/v9"
)

var _ Store = (*RedisStore)(nil)

// redisItemInternal is the structure stored in Redis, containing the actual value and ETag.
type redisItemInternal struct {
	Value []byte `json:"value"`
	Etag  string `json:"etag"`
}

type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a new RedisStore instance.
// It pings the Redis server to ensure connectivity.
// 'addr' is the Redis server address (e.g., "localhost:6379").
// 'password' is the Redis password (can be empty if no password is set).
// 'db' is the Redis database number to use.
func NewRedisStore(ctx context.Context, addr string, password string, db int) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s (DB %d): %w", addr, db, err)
	}

	return &RedisStore{client: rdb}, nil
}

// buildKey creates a namespaced key for Redis.
func (s *RedisStore) buildKey(storeName, key string) string {
	return fmt.Sprintf("%s:%s", storeName, key)
}

// SaveState saves data for a given key in the specified storeName (namespace).
// If etag is provided and non-empty, it performs an optimistic concurrency check.
// Returns the new ETag upon successful save.
func (s *RedisStore) SaveState(ctx context.Context, storeName, userKey string, data []byte, etag *string) (string, error) {
	redisKey := s.buildKey(storeName, userKey)
	newEtagVal := generateEtag() // Uses package-private generateEtag from db.go

	itemToStore := redisItemInternal{
		Value: data,
		Etag:  newEtagVal,
	}
	jsonData, err := json.Marshal(itemToStore)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data for key '%s' in store '%s': %w", userKey, storeName, err)
	}

	if etag != nil && *etag != "" {
		// Optimistic concurrency with ETag
		txErr := s.client.Watch(ctx, func(tx *redis.Tx) error {
			val, errGet := tx.Get(ctx, redisKey).Result()
			if errGet == redis.Nil {
				return fmt.Errorf("%w: for key '%s' in store '%s'", ErrKeyNotFoundForETag, userKey, storeName)
			}
			if errGet != nil {
				return fmt.Errorf("failed to get current item for ETag check on key '%s' in store '%s': %w", userKey, storeName, errGet)
			}

			var currentItem redisItemInternal
			if errUnmarshal := json.Unmarshal([]byte(val), &currentItem); errUnmarshal != nil {
				return fmt.Errorf("failed to unmarshal current item for ETag check on key '%s' in store '%s': %w", userKey, storeName, errUnmarshal)
			}

			if currentItem.Etag != *etag {
				return fmt.Errorf("%w: for key '%s' in store '%s', provided ETag '%s', current ETag '%s'", ErrETagMismatch, userKey, storeName, *etag, currentItem.Etag)
			}

			// ETag matches, proceed with set
			_, errPipe := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, redisKey, jsonData, 0) // No expiration
				return nil
			})
			return errPipe // Returns redis.TxFailedErr if WATCH was triggered before EXEC
		}, redisKey)

		if txErr != nil {
			if errors.Is(txErr, redis.TxFailedErr) {
				// Wrap the specific redis transaction error with our generic one for the service layer.
				return "", fmt.Errorf("%w: %v", ErrTransactionFailed, txErr)
			}
			// Other errors from the transaction (like ErrETagMismatch) are returned as is.
			return "", txErr
		}
		return newEtagVal, nil
	}

	// No ETag provided, direct overwrite
	if errSet := s.client.Set(ctx, redisKey, jsonData, 0).Err(); errSet != nil {
		return "", fmt.Errorf("failed to save state for key '%s' in store '%s': %w", userKey, storeName, errSet)
	}
	return newEtagVal, nil
}

// GetState retrieves data and ETag for a given key from the specified storeName.
// Returns (nil, nil) if the key is not found.
func (s *RedisStore) GetState(ctx context.Context, storeName, userKey string) (*StateItem, error) {
	redisKey := s.buildKey(storeName, userKey)
	val, err := s.client.Get(ctx, redisKey).Bytes()

	if err == redis.Nil {
		return nil, nil // Key not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get state for key '%s' in store '%s': %w", userKey, storeName, err)
	}

	var item redisItemInternal
	if errUnmarshal := json.Unmarshal(val, &item); errUnmarshal != nil {
		return nil, fmt.Errorf("failed to unmarshal data for key '%s' in store '%s': %w", userKey, storeName, errUnmarshal)
	}

	return &StateItem{Value: item.Value, Etag: item.Etag}, nil
}

// DeleteState deletes data for a given key in the specified storeName.
// If etag is provided and non-empty, it performs an optimistic concurrency check.
func (s *RedisStore) DeleteState(ctx context.Context, storeName, userKey string, etag *string) error {
	redisKey := s.buildKey(storeName, userKey)

	if etag != nil && *etag != "" {
		txErr := s.client.Watch(ctx, func(tx *redis.Tx) error {
			val, errGet := tx.Get(ctx, redisKey).Result()
			if errGet == redis.Nil { // Key does not exist, ETag was specified. This is a no-op.
				return nil
			}
			if errGet != nil {
				return fmt.Errorf("failed to get current item for ETag check on key '%s' in store '%s' for delete: %w", userKey, storeName, errGet)
			}

			var currentItem redisItemInternal
			if errUnmarshal := json.Unmarshal([]byte(val), &currentItem); errUnmarshal != nil {
				return fmt.Errorf("failed to unmarshal current item for ETag check on key '%s' in store '%s' for delete: %w", userKey, storeName, errUnmarshal)
			}

			if currentItem.Etag != *etag {
				return fmt.Errorf("%w: for key '%s' in store '%s' during delete, provided ETag '%s', current ETag '%s'", ErrETagMismatch, userKey, storeName, *etag, currentItem.Etag)
			}

			_, errPipe := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Del(ctx, redisKey)
				return nil
			})
			return errPipe // Returns redis.TxFailedErr if WATCH was triggered before EXEC
		}, redisKey)
		if txErr != nil {
			if errors.Is(txErr, redis.TxFailedErr) {
				return fmt.Errorf("%w: %v", ErrTransactionFailed, txErr)
			}
			return txErr
		}
		return nil
	}

	// No ETag provided, direct delete
	// Del on a non-existent key returns 0 and err is nil, which is fine.
	if err := s.client.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("failed to delete state for key '%s' in store '%s': %w", userKey, storeName, err)
	}
	return nil
}
