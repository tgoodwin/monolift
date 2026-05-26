package activation_miniflux_extractcontent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	adminUsername = "admin"
	adminPassword = "test123"
	feedURL       = "http://rss-feed-server/index.xml"
	// articleURL points at the in-cluster page miniflux scrapes during
	// fetch-content. nginx serves article.html as text/html and ignores the
	// query string, so a per-call ?monolift_resource=N suffix gives each
	// imported entry a fresh URL (avoiding feed-level dedup) while still
	// resolving to the same deterministic page.
	articleURL = "http://rss-feed-server/article.html"
	// fetchContentPath is a stable transcript label; the real request path
	// carries a per-run entry id, which must not leak into the compare.
	fetchContentPath = "/v1/entries/fetch-content"
	contentMarker    = "MONOLIFT-EXTRACT-MARKER"
)

type Workload struct{}

type workloadState struct {
	feedID int64
}

var states sync.Map
var entrySeq int64

func (Workload) Setup(ctx context.Context, host string) error {
	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 10 * time.Second}}
	feedID, err := client.ensureFeed(ctx, feedURL)
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
	return []string{fetchContentPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != fetchContentPath {
		return harness.Step{}, fmt.Errorf("unsupported miniflux workload path %s", path)
	}
	stateValue, ok := states.Load(host)
	if !ok {
		return harness.Step{}, fmt.Errorf("miniflux workload setup missing for %s", host)
	}
	state := stateValue.(*workloadState)
	seq := int(atomic.AddInt64(&entrySeq, 1))

	client := apiClient{base: strings.TrimRight(host, "/"), client: &http.Client{Timeout: 20 * time.Second}}
	entryID, _, err := client.importEntry(ctx, state.feedID, seq)
	if err != nil {
		return harness.Step{}, err
	}
	content, status, err := client.fetchContent(ctx, entryID)
	if err != nil {
		return harness.Step{}, err
	}
	// fetch-content drives ScrapeWebsite -> readability.ExtractContent (lifted),
	// then minify + sanitize. The marker proves the scraped page's body
	// survived the cross-network round trip; the absence of <script proves
	// ExtractContent/sanitize stripped it. Emit booleans rather than the raw
	// HTML so baseline (miniflux:latest) and lifted (pinned source) compare
	// equal regardless of incidental formatting differences. Do NOT hard-error
	// on a missing marker: in fail-closed mode the lifted shim returns the zero
	// value, miniflux keeps the imported placeholder, and the route still
	// returns 200 — which the fail-mode assertions require to be a non-5xx
	// response. Correctness in env-on is gated by the stage-8 direct-invoke
	// oracle-compare and the env-on/baseline transcript compare (both expect
	// content_has_marker=true), so a broken lift is still caught there.
	hasMarker := strings.Contains(content, contentMarker)
	scriptStripped := !strings.Contains(content, "<script")
	return harness.Step{
		Method: http.MethodGet,
		Path:   fetchContentPath,
		Status: status,
		BodyJSON: map[string]any{
			"content_has_marker": hasMarker,
			"script_stripped":    scriptStripped,
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

func (c apiClient) ensureFeed(ctx context.Context, target string) (int64, error) {
	feeds, err := c.feeds(ctx)
	if err != nil {
		return 0, err
	}
	for _, feed := range feeds {
		if feed.FeedURL == target {
			return feed.ID, nil
		}
	}
	return c.createFeed(ctx, target)
}

func (c apiClient) feeds(ctx context.Context) ([]apiFeed, error) {
	var feeds []apiFeed
	if err := c.doJSON(ctx, http.MethodGet, "/v1/feeds", nil, http.StatusOK, &feeds); err != nil {
		return nil, err
	}
	return feeds, nil
}

func (c apiClient) createFeed(ctx context.Context, target string) (int64, error) {
	var out struct {
		FeedID int64 `json:"feed_id"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/feeds", map[string]any{"feed_url": target}, http.StatusCreated, &out); err != nil {
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
		"url":         fmt.Sprintf("%s?monolift_resource=%d", articleURL, seq),
		"title":       "Monolift extract-content entry",
		"content":     "<p>placeholder import body, replaced by fetch-content</p>",
		"status":      "read",
		"external_id": fmt.Sprintf("monolift-extract-%d", seq),
	}, http.StatusCreated, &out)
	return out.ID, status, err
}

func (c apiClient) fetchContent(ctx context.Context, entryID int64) (string, int, error) {
	var out struct {
		Content     string `json:"content"`
		ReadingTime int    `json:"reading_time"`
	}
	status, err := c.doJSONStatus(ctx, http.MethodGet, fmt.Sprintf("/v1/entries/%d/fetch-content", entryID), nil, http.StatusOK, &out)
	return out.Content, status, err
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
