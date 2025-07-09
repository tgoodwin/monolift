package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tgoodwin/monolift/demo/monolith/socialgraph"
)

// socialgraphClient implements the socialgraph.Service interface by making HTTP calls to a remote service.
type socialgraphClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewServiceClient creates a new client for the Service service.
// It adheres to the socialgraph.Service interface.
func NewsocialgraphClient(baseURL string) *socialgraphClient {
	return &socialgraphClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Follow makes an RPC call to the Follow method of the remote service.
func (c *socialgraphClient) Follow(ctx context.Context, req socialgraph.FollowReq) (socialgraph.UpdateResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to marshal request for Follow: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/follow", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to create request for Follow: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("request for Follow failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return socialgraph.UpdateResp{}, fmt.Errorf("received non-200 status code %d for Follow", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp socialgraph.UpdateResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to unmarshal response for Follow: %w", err)
	}

	return serviceResp, nil
}

// GetFollowees makes an RPC call to the GetFollowees method of the remote service.
func (c *socialgraphClient) GetFollowees(ctx context.Context, req socialgraph.GetReq) (socialgraph.GetFollowResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return socialgraph.GetFollowResp{}, fmt.Errorf("failed to marshal request for GetFollowees: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/getfollowees", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return socialgraph.GetFollowResp{}, fmt.Errorf("failed to create request for GetFollowees: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return socialgraph.GetFollowResp{}, fmt.Errorf("request for GetFollowees failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return socialgraph.GetFollowResp{}, fmt.Errorf("received non-200 status code %d for GetFollowees", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp socialgraph.GetFollowResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return socialgraph.GetFollowResp{}, fmt.Errorf("failed to unmarshal response for GetFollowees: %w", err)
	}

	return serviceResp, nil
}

// GetFollowers makes an RPC call to the GetFollowers method of the remote service.
func (c *socialgraphClient) GetFollowers(ctx context.Context, req socialgraph.GetReq) (socialgraph.GetFollowerResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return socialgraph.GetFollowerResp{}, fmt.Errorf("failed to marshal request for GetFollowers: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/getfollowers", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return socialgraph.GetFollowerResp{}, fmt.Errorf("failed to create request for GetFollowers: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return socialgraph.GetFollowerResp{}, fmt.Errorf("request for GetFollowers failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return socialgraph.GetFollowerResp{}, fmt.Errorf("received non-200 status code %d for GetFollowers", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp socialgraph.GetFollowerResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return socialgraph.GetFollowerResp{}, fmt.Errorf("failed to unmarshal response for GetFollowers: %w", err)
	}

	return serviceResp, nil
}

// GetRecommendations makes an RPC call to the GetRecommendations method of the remote service.
func (c *socialgraphClient) GetRecommendations(ctx context.Context, req socialgraph.GetRecmdReq) (socialgraph.GetRecmdResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return socialgraph.GetRecmdResp{}, fmt.Errorf("failed to marshal request for GetRecommendations: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/getrecommendations", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return socialgraph.GetRecmdResp{}, fmt.Errorf("failed to create request for GetRecommendations: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return socialgraph.GetRecmdResp{}, fmt.Errorf("request for GetRecommendations failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return socialgraph.GetRecmdResp{}, fmt.Errorf("received non-200 status code %d for GetRecommendations", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp socialgraph.GetRecmdResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return socialgraph.GetRecmdResp{}, fmt.Errorf("failed to unmarshal response for GetRecommendations: %w", err)
	}

	return serviceResp, nil
}

// Unfollow makes an RPC call to the Unfollow method of the remote service.
func (c *socialgraphClient) Unfollow(ctx context.Context, req socialgraph.UnfollowReq) (socialgraph.UpdateResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to marshal request for Unfollow: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/unfollow", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to create request for Unfollow: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("request for Unfollow failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return socialgraph.UpdateResp{}, fmt.Errorf("received non-200 status code %d for Unfollow", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp socialgraph.UpdateResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return socialgraph.UpdateResp{}, fmt.Errorf("failed to unmarshal response for Unfollow: %w", err)
	}

	return serviceResp, nil
}
