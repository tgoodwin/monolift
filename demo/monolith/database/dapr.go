package database

import (
	"context"
	"fmt"

	dapr "github.com/dapr/go-sdk/client"
)

var _ Store = (*DaprStore)(nil)

// DaprStore implements the Store interface using the Dapr client.
type DaprStore struct {
	client dapr.Client
}

// NewDaprStore creates a new DaprStore instance.
func NewDaprStore() (*DaprStore, error) {
	client, err := dapr.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create dapr client: %w", err)
	}
	return &DaprStore{client: client}, nil
}

// SaveState saves data for a given key in the specified storeName (namespace).
// If etag is provided and non-empty, it performs an optimistic concurrency check.
// Returns the new ETag upon successful save.
func (s *DaprStore) SaveState(ctx context.Context, storeName, key string, data []byte, etag *string) (string, error) {
	item := &dapr.SetStateItem{
		Key:   key,
		Value: data,
	}
	if etag != nil {
		item.Etag = &dapr.ETag{
			Value: *etag,
		}
	}

	err := s.client.SaveState(ctx, storeName, key, data, nil)
	if err != nil {
		return "", err
	}
	// Dapr does not return the new ETag on save, so we have to do a get
	newItem, err := s.GetState(ctx, storeName, key)
	if err != nil {
		return "", err
	}
	return newItem.Etag, nil
}

// GetState retrieves data and ETag for a given key from the specified storeName.
// Returns (nil, nil) if the key is not found.
func (s *DaprStore) GetState(ctx context.Context, storeName, key string) (*StateItem, error) {
	item, err := s.client.GetState(ctx, storeName, key, nil)
	if err != nil {
		return nil, err
	}
	if item == nil || item.Value == nil {
		return nil, nil
	}
	return &StateItem{
		Value: item.Value,
		Etag:  item.Etag,
	}, nil
}

// DeleteState deletes data for a given key in the specified storeName.
// If etag is provided and non-empty, it performs an optimistic concurrency check.
func (s *DaprStore) DeleteState(ctx context.Context, storeName, key string, etag *string) error {
	if etag != nil && *etag != "" {
		daprEtag := &dapr.ETag{
			Value: *etag,
		}
		return s.client.DeleteStateWithETag(ctx, storeName, key, daprEtag, nil, nil)
	}
	return s.client.DeleteState(ctx, storeName, key, nil)
}
