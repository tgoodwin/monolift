package activation_listmonk_sanitizeuri

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const loginPath = "/admin/login"

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	healthURL := strings.TrimRight(host, "/") + "/admin/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("listmonk health check: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("listmonk health check: status %d", resp.StatusCode)
	}
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
	return []string{loginPath}
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != loginPath {
		return harness.Step{}, fmt.Errorf("unsupported listmonk workload path %s", path)
	}
	client := &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	base := strings.TrimRight(host, "/")
	form := url.Values{}
	form.Set("username", "admin")
	form.Set("password", "admin")
	form.Set("next", "https://evil.com/dashboard?x=1")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+loginPath, strings.NewReader(form.Encode()))
	if err != nil {
		return harness.Step{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return harness.Step{}, fmt.Errorf("POST %s: %w", loginPath, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	location := resp.Header.Get("Location")
	return harness.Step{
		Method: http.MethodPost,
		Path:   loginPath,
		Status: resp.StatusCode,
		BodyJSON: map[string]any{
			"location": location,
			"body":     string(body),
		},
	}, nil
}
