package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tgoodwin/monolift/demo/monolith/types/user"
)

// userserviceClient implements the userservice.Service interface by making HTTP calls to a remote service.
type userserviceClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewServiceClient creates a new client for the Service service.
// It adheres to the userservice.Service interface.
func NewuserserviceClient(baseURL string) *userserviceClient {
	return &userserviceClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// Login makes an RPC call to the Login method of the remote service.
func (c *userserviceClient) Login(ctx context.Context, req user.LoginReq) (user.LoginResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return user.LoginResp{}, fmt.Errorf("failed to marshal request for Login: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/login", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return user.LoginResp{}, fmt.Errorf("failed to create request for Login: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return user.LoginResp{}, fmt.Errorf("request for Login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user.LoginResp{}, fmt.Errorf("received non-200 status code %d for Login", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp user.LoginResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return user.LoginResp{}, fmt.Errorf("failed to unmarshal response for Login: %w", err)
	}

	return serviceResp, nil
}

// Register makes an RPC call to the Register method of the remote service.
func (c *userserviceClient) Register(ctx context.Context, req user.RegisterReq) (user.RegisterResp, error) {
	// 1. Marshal the request body
	reqBodyBytes, err := json.Marshal(req)
	if err != nil {
		return user.RegisterResp{}, fmt.Errorf("failed to marshal request for Register: %w", err)
	}

	// 2. Create and send the HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/register", bytes.NewBuffer(reqBodyBytes))
	if err != nil {
		return user.RegisterResp{}, fmt.Errorf("failed to create request for Register: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return user.RegisterResp{}, fmt.Errorf("request for Register failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return user.RegisterResp{}, fmt.Errorf("received non-200 status code %d for Register", resp.StatusCode)
	}

	// 3. Unmarshal the response
	var serviceResp user.RegisterResp
	if err := json.NewDecoder(resp.Body).Decode(&serviceResp); err != nil {
		return user.RegisterResp{}, fmt.Errorf("failed to unmarshal response for Register: %w", err)
	}

	return serviceResp, nil
}
