package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tgoodwin/monolift/demo/monolith/types/post"
)

// postserviceClient implements the postservice.Service interface by making HTTP calls to a remote service.
type postserviceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewServiceClient creates a new client for the Service service.
// It adheres to the postservice.Service interface.
func NewpostserviceClient(baseURL string) *postserviceClient {
	return &postserviceClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// AddComment makes an RPC call to the AddComment method of the remote service.
func (c *postserviceClient) AddComment(ctx context.Context, req post.CommentReq) (post.UpdatePostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to marshal request for AddComment: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/addcomment", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to create request for AddComment: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("request for AddComment failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.UpdatePostResp{}, fmt.Errorf("received non-200 status code %d for AddComment", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.UpdatePostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to unmarshal response for AddComment: %w", err)
	}

	return serviceResp, nil
}

// DeletePost makes an RPC call to the DeletePost method of the remote service.
func (c *postserviceClient) DeletePost(ctx context.Context, req post.DelPostReq) (post.UpdatePostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to marshal request for DeletePost: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/deletepost", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to create request for DeletePost: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("request for DeletePost failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.UpdatePostResp{}, fmt.Errorf("received non-200 status code %d for DeletePost", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.UpdatePostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to unmarshal response for DeletePost: %w", err)
	}

	return serviceResp, nil
}

// ReadPosts makes an RPC call to the ReadPosts method of the remote service.
func (c *postserviceClient) ReadPosts(ctx context.Context, req post.ReadPostReq) (post.ReadPostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.ReadPostResp{}, fmt.Errorf("failed to marshal request for ReadPosts: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/readposts", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.ReadPostResp{}, fmt.Errorf("failed to create request for ReadPosts: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.ReadPostResp{}, fmt.Errorf("request for ReadPosts failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.ReadPostResp{}, fmt.Errorf("received non-200 status code %d for ReadPosts", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.ReadPostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.ReadPostResp{}, fmt.Errorf("failed to unmarshal response for ReadPosts: %w", err)
	}

	return serviceResp, nil
}

// SavePost makes an RPC call to the SavePost method of the remote service.
func (c *postserviceClient) SavePost(ctx context.Context, req post.SavePostReq) (post.UpdatePostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to marshal request for SavePost: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/savepost", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to create request for SavePost: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("request for SavePost failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.UpdatePostResp{}, fmt.Errorf("received non-200 status code %d for SavePost", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.UpdatePostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to unmarshal response for SavePost: %w", err)
	}

	return serviceResp, nil
}

// UpdateMeta makes an RPC call to the UpdateMeta method of the remote service.
func (c *postserviceClient) UpdateMeta(ctx context.Context, req post.MetaReq) (post.UpdatePostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to marshal request for UpdateMeta: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/updatemeta", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to create request for UpdateMeta: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("request for UpdateMeta failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.UpdatePostResp{}, fmt.Errorf("received non-200 status code %d for UpdateMeta", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.UpdatePostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to unmarshal response for UpdateMeta: %w", err)
	}

	return serviceResp, nil
}

// UpvotePost makes an RPC call to the UpvotePost method of the remote service.
func (c *postserviceClient) UpvotePost(ctx context.Context, req post.UpvoteReq) (post.UpdatePostResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to marshal request for UpvotePost: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/upvotepost", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to create request for UpvotePost: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("request for UpvotePost failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return post.UpdatePostResp{}, fmt.Errorf("received non-200 status code %d for UpvotePost", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp post.UpdatePostResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return post.UpdatePostResp{}, fmt.Errorf("failed to unmarshal response for UpvotePost: %w", err)
	}

	return serviceResp, nil
}
