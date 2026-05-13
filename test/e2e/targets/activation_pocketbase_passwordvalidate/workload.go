package activation_pocketbase_passwordvalidate

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

const healthPath = "/api/health"
const collectionsPath = "/api/collections?filter=" + "name%7E%27_superusers%27"

const (
	superuserEmail    = "admin@example.com"
	superuserPassword = "Monolift123!"
)

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	base := strings.TrimRight(host, "/")
	deadline := time.Now().Add(2 * time.Minute)

	// Phase 1: wait for health endpoint.
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+healthPath, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				goto authReady
			}
			lastErr = fmt.Errorf("GET %s status=%d", healthPath, resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("health not ready: %w", lastErr)

authReady:
	// Phase 2: wait for superuser auth to work. After an env-off rollout the
	// pod restarts with a fresh emptyDir; the entrypoint re-creates the
	// superuser via "pocketbase superuser upsert" before serving, but
	// PocketBase may need additional time after the health endpoint is live
	// before the auth subsystem is fully functional.
	for time.Now().Before(deadline) {
		_, err := authTokenOnce(ctx, base)
		if err == nil {
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("auth not ready after health: %w", lastErr)
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, collectionsPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{collectionsPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != collectionsPath {
		return harness.Step{}, fmt.Errorf("unsupported pocketbase workload path %s", path)
	}
	base := strings.TrimRight(host, "/")
	token, err := authToken(ctx, base)
	if err != nil {
		return harness.Step{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return harness.Step{}, err
	}
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return harness.Step{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		TotalItems int `json:"totalItems"`
		Items      []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return harness.Step{}, fmt.Errorf("decode collections response: %w: %s", err, strings.TrimSpace(string(body)))
	}
	names := make([]string, 0, len(data.Items))
	for _, item := range data.Items {
		names = append(names, item.Name)
	}
	return harness.Step{
		Method: http.MethodGet,
		Path:   path,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		BodyJSON: map[string]any{
			"totalItems": data.TotalItems,
			"names":      names,
		},
	}, nil
}

func authToken(ctx context.Context, base string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 15; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		token, err := authTokenOnce(ctx, base)
		if err == nil {
			return token, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("auth failed after 15 attempts: %w", lastErr)
}

func authTokenOnce(ctx context.Context, base string) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"identity": superuserEmail,
		"password": superuserPassword,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/collections/_superusers/auth-with-password", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("POST /api/collections/_superusers/auth-with-password status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", err
	}
	if data.Token == "" {
		return "", fmt.Errorf("auth response missing token")
	}
	return data.Token, nil
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}
