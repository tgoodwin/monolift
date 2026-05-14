package activation_miniflux_parsefeed

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
	adminPassword = "test123"
	refreshPath   = "/v1/feeds/refresh"
)

type Workload struct{}

type workloadState struct {
	feedID int64
}

var states sync.Map

func (Workload) Setup(ctx context.Context, host string) error {
	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 10 * time.Second}}
	feedID, err := client.ensureFeed(ctx, "http://rss-feed-server/index.xml")
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
	return []string{refreshPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != refreshPath {
		return harness.Step{}, fmt.Errorf("unsupported miniflux workload path %s", path)
	}
	stateValue, ok := states.Load(host)
	if !ok {
		return harness.Step{}, fmt.Errorf("miniflux workload setup missing for %s", host)
	}
	state := stateValue.(*workloadState)

	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 30 * time.Second}}

	// Trigger feed refresh — internally calls ParseFeed on the fetched XML.
	status, err := client.refreshFeed(ctx, state.feedID)
	if err != nil {
		return harness.Step{}, err
	}

	// Verify entries were created by fetching them.
	entries, err := client.feedEntries(ctx, state.feedID)
	if err != nil {
		return harness.Step{}, err
	}

	return harness.Step{
		Method: http.MethodPut,
		Path:   refreshPath,
		Status: status,
		BodyJSON: map[string]any{
			"entry_count": len(entries),
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

type apiFeed struct {
	ID      int64  `json:"id"`
	FeedURL string `json:"feed_url"`
}

type apiEntry struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

func (c apiClient) ensureFeed(ctx context.Context, feedURL string) (int64, error) {
	feeds, err := c.feeds(ctx)
	if err != nil {
		return 0, err
	}
	for _, feed := range feeds {
		if feed.FeedURL == feedURL {
			return feed.ID, nil
		}
	}
	return c.createFeed(ctx, feedURL)
}

func (c apiClient) feeds(ctx context.Context) ([]apiFeed, error) {
	var feeds []apiFeed
	if err := c.doJSON(ctx, http.MethodGet, "/v1/feeds", nil, http.StatusOK, &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
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

func (c apiClient) refreshFeed(ctx context.Context, feedID int64) (int, error) {
	path := fmt.Sprintf("/v1/feeds/%d/refresh", feedID)
	return c.doStatus(ctx, http.MethodPut, path, nil, http.StatusNoContent)
}

func (c apiClient) feedEntries(ctx context.Context, feedID int64) ([]apiEntry, error) {
	path := fmt.Sprintf("/v1/feeds/%d/entries", feedID)
	var out struct {
		Entries []apiEntry `json:"entries"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

func (c apiClient) doJSON(ctx context.Context, method, path string, payload any, wantStatus int, out any) error {
	_, err := c.doJSONStatus(ctx, method, path, payload, wantStatus, out)
	return err
}

func (c apiClient) doStatus(ctx context.Context, method, path string, payload any, wantStatus int) (int, error) {
	return c.doJSONStatus(ctx, method, path, payload, wantStatus, nil)
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
