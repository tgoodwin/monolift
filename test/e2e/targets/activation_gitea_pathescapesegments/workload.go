package activation_gitea_pathescapesegments

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	giteaUsername = "monolift"
	giteaPassword = "Monolift123!"
	giteaRepo     = "path-escape-demo"
	repoFilePath  = "/" + giteaUsername + "/" + giteaRepo + "/src/branch/main/README.md"
)

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	client, err := newGiteaClient(host)
	if err != nil {
		return err
	}
	if err := client.waitReady(ctx); err != nil {
		return err
	}
	if ok, err := client.repoReady(ctx); err != nil {
		return err
	} else if ok {
		return nil
	}
	if err := client.ensureUser(ctx); err != nil {
		return err
	}
	return client.ensureRepo(ctx)
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, repoFilePath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{repoFilePath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	client, err := newGiteaClient(host)
	if err != nil {
		return harness.Step{}, err
	}
	resp, body, err := client.do(ctx, http.MethodGet, path, nil, false)
	if err != nil {
		return harness.Step{}, err
	}
	return harness.Step{
		Method: http.MethodGet,
		Path:   path,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		BodyJSON: map[string]any{"bytes": len(body)},
	}, nil
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}

type giteaClient struct {
	base   string
	client *http.Client
}

func newGiteaClient(host string) (giteaClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return giteaClient{}, err
	}
	return giteaClient{
		base: strings.TrimRight(host, "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
		},
	}, nil
}

func (c giteaClient) waitReady(ctx context.Context) error {
	deadline := time.Now().Add(2 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, body, err := c.do(ctx, http.MethodGet, "/api/healthz", nil, false)
		if err == nil && resp.StatusCode == http.StatusOK {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("GET /api/healthz status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return lastErr
}

func (c giteaClient) repoReady(ctx context.Context) (bool, error) {
	resp, _, err := c.do(ctx, http.MethodGet, repoFilePath, nil, false)
	if err != nil {
		return false, err
	}
	return resp.StatusCode == http.StatusOK, nil
}

func (c giteaClient) ensureUser(ctx context.Context) error {
	form := url.Values{}
	form.Set("user_name", giteaUsername)
	form.Set("email", "monolift@example.org")
	form.Set("password", giteaPassword)
	form.Set("retype", giteaPassword)
	resp, body, err := c.do(ctx, http.MethodPost, "/user/sign_up", strings.NewReader(form.Encode()), false, "application/x-www-form-urlencoded")
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusOK {
		return nil
	}
	return fmt.Errorf("POST /user/sign_up status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c giteaClient) ensureRepo(ctx context.Context) error {
	payload := map[string]any{
		"name":           giteaRepo,
		"auto_init":      true,
		"default_branch": "main",
		"readme":         "Default",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, body, err := c.do(ctx, http.MethodPost, "/api/v1/user/repos", bytes.NewReader(data), true, "application/json")
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusConflict, http.StatusUnprocessableEntity:
		return nil
	default:
		return fmt.Errorf("POST /api/v1/user/repos status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
}

func (c giteaClient) do(ctx context.Context, method, path string, body io.Reader, basicAuth bool, contentType ...string) (*http.Response, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, nil, err
	}
	if basicAuth {
		req.SetBasicAuth(giteaUsername, giteaPassword)
	}
	if len(contentType) > 0 && contentType[0] != "" {
		req.Header.Set("Content-Type", contentType[0])
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, data, nil
}
