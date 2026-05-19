package activation_listmonk_processimage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const mediaPath = "/api/media"

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(host, "/")+"/admin/login", nil)
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
	step, err := Workload{}.Request(ctx, host, mediaPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{mediaPath}
}

func (Workload) Verify(ctx context.Context, host string, expected harness.Transcript) error {
	got, err := Workload{}.Action(ctx, host)
	if err != nil {
		return err
	}
	return harness.Transcript{}.Compare(expected, got, nil)
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != mediaPath {
		return harness.Step{}, fmt.Errorf("unsupported listmonk workload path %s", path)
	}
	data, err := os.ReadFile("targets/activation_listmonk_processimage/testdata/fixture.png")
	if err != nil {
		return harness.Step{}, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "monolift-processimage-fixture.png")
	if err != nil {
		return harness.Step{}, err
	}
	if _, err := part.Write(data); err != nil {
		return harness.Step{}, err
	}
	if err := writer.Close(); err != nil {
		return harness.Step{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(host, "/")+mediaPath, &body)
	if err != nil {
		return harness.Step{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.SetBasicAuth("admin", "adminpass123")
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return harness.Step{}, fmt.Errorf("POST %s: %w", mediaPath, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	bodyJSON := map[string]any{"body": string(raw)}
	if resp.StatusCode == http.StatusOK {
		bodyJSON = summarizeMediaResponse(raw)
	}
	return harness.Step{
		Method:   http.MethodPost,
		Path:     mediaPath,
		Status:   resp.StatusCode,
		BodyJSON: bodyJSON,
	}, nil
}

func summarizeMediaResponse(raw []byte) map[string]any {
	var envelope struct {
		Data struct {
			ContentType string         `json:"content_type"`
			ThumbURL    string         `json:"thumb_url"`
			URL         string         `json:"url"`
			Meta        map[string]any `json:"meta"`
		} `json:"data"`
	}
	_ = json.Unmarshal(raw, &envelope)
	return map[string]any{
		"content_type": envelope.Data.ContentType,
		"has_thumb":    envelope.Data.ThumbURL != "",
		"has_url":      envelope.Data.URL != "",
		"width":        envelope.Data.Meta["width"],
		"height":       envelope.Data.Meta["height"],
	}
}
