package caddy

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
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
	return []string{"/static/hello.txt", "/proxy?x=1", "/headers"}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(host, "/")+path, nil)
	if err != nil {
		return harness.Step{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return harness.Step{}, fmt.Errorf("%s %s: %w", http.MethodGet, path, err)
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return harness.Step{}, readErr
	}
	return harness.Step{
		Method: http.MethodGet,
		Path:   path,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
			"X-Caddy":      resp.Header.Get("X-Caddy"),
		},
		BodyJSON: map[string]any{"body": strings.TrimSpace(string(body))},
	}, nil
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}
