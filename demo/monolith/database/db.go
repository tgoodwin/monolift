package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// StateItem represents a single state item with an ETag.
type StateItem struct {
	Value []byte
	Etag  string
	// LastUpdated time.Time // Optional: for debugging or TTL later
}

// Store defines the interface for our key-value persistence layer.
// It's simplified to focus on core KV operations for the monolith.
// The 'storeName' acts as a namespace for keys.
type Store interface {
	// SaveState saves data for a given key.
	// If etag is provided and non-empty, it performs an optimistic concurrency check.
	// If etag is nil or empty, it overwrites. A new ETag is generated upon successful save.
	SaveState(ctx context.Context, storeName, key string, data []byte, etag *string) (newEtag string, err error)

	// GetState retrieves data and ETag for a given key.
	// Returns (nil, nil) if the key is not found.
	GetState(ctx context.Context, storeName, key string) (*StateItem, error)

	// DeleteState deletes data for a given key.
	// If etag is provided and non-empty, it performs an optimistic concurrency check.
	DeleteState(ctx context.Context, storeName, key string, etag *string) error

	// TODO: Consider adding Bulk operations later if needed for performance:
	// GetBulkState(ctx context.Context, storeName string, keys []string) ([]*StateItem, error)
	// SaveBulkState(ctx context.Context, storeName string, items map[string][]byte) error // Or a slice of StateItems
}

// InMemoryKVStore is an in-memory implementation of the Store interface.
// It uses a simple map and a mutex for thread-safety.
type InMemoryKVStore struct {
	mu     sync.RWMutex
	stores map[string]map[string]StateItem // storeName -> key -> StateItem
}

// NewInMemoryKVStore creates a new InMemoryKVStore.
func NewInMemoryKVStore() *InMemoryKVStore {
	return &InMemoryKVStore{
		stores: make(map[string]map[string]StateItem),
	}
}

// generateEtag creates a simple ETag based on current time.
func generateEtag() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}

// SaveState saves data for a given key.
func (s *InMemoryKVStore) SaveState(ctx context.Context, storeName, key string, data []byte, etag *string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.stores[storeName]; !ok {
		s.stores[storeName] = make(map[string]StateItem)
	}
	store := s.stores[storeName]

	if etag != nil && *etag != "" {
		currentItem, exists := store[key]
		if !exists {
			// Dapr's SaveState with ETag would typically fail if the key doesn't exist yet
			// because there's no existing ETag to match.
			return "", fmt.Errorf("ETag specified for key '%s' in store '%s' that does not exist", key, storeName)
		}
		if currentItem.Etag != *etag {
			return "", fmt.Errorf("ETag mismatch for key '%s' in store '%s': provided '%s', current '%s'", key, storeName, *etag, currentItem.Etag)
		}
	}

	newEtag := generateEtag()
	store[key] = StateItem{
		Value: data,
		Etag:  newEtag,
	}
	return newEtag, nil
}

// GetState retrieves data for a given key.
func (s *InMemoryKVStore) GetState(ctx context.Context, storeName, key string) (*StateItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	store, storeExists := s.stores[storeName]
	if !storeExists {
		return nil, nil // Store itself doesn't exist, so key cannot exist
	}

	item, exists := store[key]
	if !exists {
		return nil, nil // Return nil item if not found, consistent with Dapr's GetState
	}
	// Return a copy to prevent modification of the internal map's value if it were a pointer type
	return &StateItem{Value: item.Value, Etag: item.Etag}, nil
}

// DeleteState deletes data for a given key.
func (s *InMemoryKVStore) DeleteState(ctx context.Context, storeName, key string, etag *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, storeExists := s.stores[storeName]
	if !storeExists {
		return nil // Store doesn't exist, so nothing to delete
	}

	if etag != nil && *etag != "" {
		currentItem, exists := store[key]
		if !exists {
			// If ETag is specified but key doesn't exist, it's effectively a no-op or an indication of a stale request.
			// Dapr's behavior here can vary based on store; for simplicity, we'll allow it (idempotent).
			// If strict ETag checking on non-existent key is desired, an error could be returned.
			return nil
		}
		if currentItem.Etag != *etag {
			return fmt.Errorf("ETag mismatch for key '%s' in store '%s' during delete: provided '%s', current '%s'", key, storeName, *etag, currentItem.Etag)
		}
	}

	delete(store, key)
	return nil
}

// Helper to marshal data to JSON bytes for storage
func Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// Helper to unmarshal JSON bytes from storage
func Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
