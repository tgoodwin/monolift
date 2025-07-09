package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tgoodwin/monolift/demo/monolith/types/timeline"
)

// timelineserviceClient implements the timelineservice.Service interface by making HTTP calls to a remote service.
type timelineserviceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewServiceClient creates a new client for the Service service.
// It adheres to the timelineservice.Service interface.
func NewtimelineserviceClient(baseURL string) *timelineserviceClient {
	return &timelineserviceClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// ReadTimeline makes an RPC call to the ReadTimeline method of the remote service.
func (c *timelineserviceClient) ReadTimeline(ctx context.Context, req timeline.ReadReq) (timeline.ReadResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return timeline.ReadResp{}, fmt.Errorf("failed to marshal request for ReadTimeline: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/readtimeline", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return timeline.ReadResp{}, fmt.Errorf("failed to create request for ReadTimeline: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return timeline.ReadResp{}, fmt.Errorf("request for ReadTimeline failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return timeline.ReadResp{}, fmt.Errorf("received non-200 status code %d for ReadTimeline", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp timeline.ReadResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return timeline.ReadResp{}, fmt.Errorf("failed to unmarshal response for ReadTimeline: %w", err)
	}

	return serviceResp, nil
}

// UpdateTimeline makes an RPC call to the UpdateTimeline method of the remote service.
func (c *timelineserviceClient) UpdateTimeline(ctx context.Context, req timeline.UpdateReq) (timeline.UpdateResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return timeline.UpdateResp{}, fmt.Errorf("failed to marshal request for UpdateTimeline: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/updatetimeline", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return timeline.UpdateResp{}, fmt.Errorf("failed to create request for UpdateTimeline: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return timeline.UpdateResp{}, fmt.Errorf("request for UpdateTimeline failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return timeline.UpdateResp{}, fmt.Errorf("received non-200 status code %d for UpdateTimeline", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp timeline.UpdateResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return timeline.UpdateResp{}, fmt.Errorf("failed to unmarshal response for UpdateTimeline: %w", err)
	}

	return serviceResp, nil
}
