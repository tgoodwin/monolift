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
	client := &http.Client{Timeout: 10 * time.Second}
	paths := []string{"/static/hello.txt", "/proxy?x=1", "/headers"}
	transcript := harness.Transcript{Steps: make([]harness.Step, 0, len(paths))}
	for _, path := range paths {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(host, "/")+path, nil)
		if err != nil {
			return harness.Transcript{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return harness.Transcript{}, fmt.Errorf("%s %s: %w", http.MethodGet, path, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return harness.Transcript{}, readErr
		}
		transcript.Steps = append(transcript.Steps, harness.Step{
			Method: http.MethodGet,
			Path:   path,
			Status: resp.StatusCode,
			Headers: map[string]string{
				"Content-Type": resp.Header.Get("Content-Type"),
				"X-Caddy":      resp.Header.Get("X-Caddy"),
			},
			BodyJSON: map[string]any{"body": strings.TrimSpace(string(body))},
		})
	}
	return transcript, nil
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}
