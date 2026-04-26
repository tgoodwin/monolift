package miniflux

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	adminUsername = "admin"
	adminPassword = "password"
	importPath    = "/v1/feeds/current/entries/import"
	entryContent  = "<article><p>Monolift deterministic reading time content for the imported entry.</p></article>"
)

type Workload struct{}

type workloadState struct {
	feedID int64
	next   int
}

var states sync.Map

func (Workload) Setup(ctx context.Context, host string) error {
	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 10 * time.Second}}
	user, err := client.me(ctx)
	if err != nil {
		return err
	}
	show := true
	if !user.ShowReadingTime {
		if err := client.updateUser(ctx, user.ID, map[string]any{"show_reading_time": show}); err != nil {
			return err
		}
	}
	feedID, err := client.createFeed(ctx, "http://rss-feed-server/index.xml")
	if err != nil {
		return err
	}
	states.Store(host, &workloadState{feedID: feedID})
	return nil
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	paths := Workload{}.Paths()
	transcript := harness.Transcript{Steps: make([]harness.Step, 0, len(paths))}
	for _, path := range paths {
		step, err := Workload{}.Request(ctx, host, path)
		if err != nil {
			return harness.Transcript{}, err
		}
		transcript.Steps = append(transcript.Steps, step)
	}
	return transcript, nil
}

func (Workload) Paths() []string {
	return []string{importPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != importPath {
		return harness.Step{}, fmt.Errorf("unsupported miniflux workload path %s", path)
	}
	stateValue, ok := states.Load(host)
	if !ok {
		return harness.Step{}, fmt.Errorf("miniflux workload setup missing for %s", host)
	}
	state := stateValue.(*workloadState)
	state.next++
	seq := state.next

	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 10 * time.Second}}
	entryID, status, err := client.importEntry(ctx, state.feedID, seq)
	if err != nil {
		return harness.Step{}, err
	}
	entry, err := client.entry(ctx, entryID)
	if err != nil {
		return harness.Step{}, err
	}
	if entry.ReadingTime == 0 {
		return harness.Step{}, fmt.Errorf("imported entry reading_time=0")
	}
	return harness.Step{
		Method: http.MethodPost,
		Path:   importPath,
		Status: status,
		BodyJSON: map[string]any{
			"reading_time": entry.ReadingTime,
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

type apiClient struct {
	base   string
	client *http.Client
}

type apiUser struct {
	ID              int64 `json:"id"`
	ShowReadingTime bool  `json:"show_reading_time"`
}

type apiEntry struct {
	ID          int64 `json:"id"`
	ReadingTime int   `json:"reading_time"`
}

func (c apiClient) me(ctx context.Context) (apiUser, error) {
	var user apiUser
	if err := c.doJSON(ctx, http.MethodGet, "/v1/me", nil, http.StatusOK, &user); err != nil {
		return apiUser{}, err
	}
	return user, nil
}

func (c apiClient) updateUser(ctx context.Context, userID int64, payload map[string]any) error {
	var user apiUser
	return c.doJSON(ctx, http.MethodPut, fmt.Sprintf("/v1/users/%d", userID), payload, http.StatusOK, &user)
}

func (c apiClient) createFeed(ctx context.Context, feedURL string) (int64, error) {
	var out struct {
		FeedID int64 `json:"feed_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/feeds", map[string]any{"feed_url": feedURL}, http.StatusCreated, &out); err != nil {
		return 0, err
	}
	return out.FeedID, nil
}

func (c apiClient) importEntry(ctx context.Context, feedID int64, seq int) (int64, int, error) {
	var out struct {
		ID int64 `json:"id"`
	}
	path := fmt.Sprintf("/v1/feeds/%d/entries/import", feedID)
	status, err := c.doJSONStatus(ctx, http.MethodPost, path, map[string]any{
		"url":         fmt.Sprintf("https://example.org/monolift-entry-%d", seq),
		"title":       "Monolift imported entry",
		"content":     entryContent,
		"status":      "read",
		"external_id": fmt.Sprintf("monolift-entry-%d", seq),
	}, http.StatusCreated, &out)
	return out.ID, status, err
}

func (c apiClient) entry(ctx context.Context, entryID int64) (apiEntry, error) {
	var entry apiEntry
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/entries/%d", entryID), nil, http.StatusOK, &entry); err != nil {
		return apiEntry{}, err
	}
	return entry, nil
}

func (c apiClient) doJSON(ctx context.Context, method, path string, payload any, wantStatus int, out any) error {
	_, err := c.doJSONStatus(ctx, method, path, payload, wantStatus, out)
	return err
}

func (c apiClient) doJSONStatus(ctx context.Context, method, path string, payload any, wantStatus int, out any) (int, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return 0, err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(adminUsername, adminPassword)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, fmt.Errorf("%s %s status=%d want %d body=%s", method, path, resp.StatusCode, wantStatus, strings.TrimSpace(string(data)))
	}
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}
