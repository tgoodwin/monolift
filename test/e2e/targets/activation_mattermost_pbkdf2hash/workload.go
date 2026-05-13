package activation_mattermost_pbkdf2hash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	pingPath     = "/api/v4/system/ping"
	loginPath    = "/api/v4/users/login"
	testPassword = "Monolift123!"
)

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	deadline := time.Now().Add(3 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(host, "/")+pingPath, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("GET %s status=%d", pingPath, resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return lastErr
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, loginPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{loginPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	host = strings.TrimRight(host, "/")
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	email := "ml-" + suffix + "@example.com"
	username := "ml" + suffix

	// Create user — triggers PBKDF2.Hash internally.
	createStatus, _, _, err := postJSON(ctx, host, "/api/v4/users", "", map[string]any{
		"email":    email,
		"username": username,
		"password": testPassword,
	}, nil)
	if err != nil {
		return harness.Step{}, fmt.Errorf("create user: %w", err)
	}
	if createStatus >= 300 {
		return harness.Step{}, fmt.Errorf("create user status=%d", createStatus)
	}

	// Login — triggers PBKDF2.CompareHashAndPassword, verifying the hash
	// produced by Hash is valid.
	loginStatus, headers, _, err := postJSON(ctx, host, loginPath, "", map[string]any{
		"login_id": email,
		"password": testPassword,
	}, nil)
	if err != nil {
		return harness.Step{}, fmt.Errorf("login: %w", err)
	}
	token := headers.Get("Token")

	return harness.Step{
		Method: http.MethodPost,
		Path:   path,
		Status: loginStatus,
		Headers: map[string]string{
			"Content-Type": headers.Get("Content-Type"),
		},
		BodyJSON: map[string]any{
			"authenticated": token != "",
		},
	}, nil
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}

func postJSON(ctx context.Context, host, path, token string, payload any, out any) (int, http.Header, []byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+path, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Header, body, fmt.Errorf("POST %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.StatusCode, resp.Header, body, fmt.Errorf("decode POST %s: %w", path, err)
		}
	}
	return resp.StatusCode, resp.Header, body, nil
}
