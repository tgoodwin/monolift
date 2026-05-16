package activation_pocketbase_createthumb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	healthPath        = "/api/health"
	workloadPath      = "/api/files/demo1/_uploaded_/monolift.png?thumb=100x100"
	uploadCollection  = "demo1"
	uploadFileField   = "file_many"
	uploadFilename    = "300_WlbFWSGmW9.png"
	uploadImagePath   = "evaluation/pocketbase/tests/data/storage/wsmn24bux7wo113/84nmscqy84lsi1t/300_WlbFWSGmW9.png"
	thumbExpectedSize = 100
	superuserEmail    = "admin@example.com"
	superuserPassword = "Monolift123!"
)

var uploadCounter atomic.Uint64

type Workload struct{}

func (Workload) Setup(ctx context.Context, host string) error {
	base := strings.TrimRight(host, "/")
	deadline := time.Now().Add(2 * time.Minute)
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
				return nil
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
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, workloadPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{workloadPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != workloadPath {
		return harness.Step{}, fmt.Errorf("unsupported pocketbase thumbnail workload path %s", path)
	}
	base := strings.TrimRight(host, "/")
	token, err := authToken(ctx, base)
	if err != nil {
		return harness.Step{}, err
	}
	recordID, filename, err := createImageRecord(ctx, base, token)
	if err != nil {
		return harness.Step{}, err
	}
	filePath := fmt.Sprintf("/api/files/%s/%s/%s?thumb=100x100", uploadCollection, recordID, filename)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+filePath, nil)
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
	contentType := resp.Header.Get("Content-Type")
	width, height, decodeErr := pngDimensions(body)
	if resp.StatusCode == http.StatusOK && decodeErr != nil {
		return harness.Step{}, fmt.Errorf("decode thumbnail response: %w", decodeErr)
	}
	if resp.StatusCode == http.StatusOK && (width != thumbExpectedSize || height != thumbExpectedSize) {
		return harness.Step{}, fmt.Errorf("thumbnail dimensions=%dx%d want %dx%d", width, height, thumbExpectedSize, thumbExpectedSize)
	}
	return harness.Step{
		Method: http.MethodGet,
		Path:   workloadPath,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": contentType,
		},
		BodyJSON: map[string]any{
			"thumb_width":  width,
			"thumb_height": height,
			"content_type": contentType,
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

func (Workload) VerifyBehavior(_ context.Context, _ string, transcript harness.Transcript) error {
	if len(transcript.Steps) == 0 {
		return fmt.Errorf("missing thumbnail workload step")
	}
	body, ok := transcript.Steps[0].BodyJSON.(map[string]any)
	if !ok {
		return fmt.Errorf("thumbnail workload body has type %T", transcript.Steps[0].BodyJSON)
	}
	width, widthOK := numericBodyValue(body["thumb_width"])
	height, heightOK := numericBodyValue(body["thumb_height"])
	if !widthOK || !heightOK || width != thumbExpectedSize || height != thumbExpectedSize {
		return fmt.Errorf("thumbnail dimensions body=%+v want %dx%d", body, thumbExpectedSize, thumbExpectedSize)
	}
	if contentType, _ := body["content_type"].(string); !strings.HasPrefix(contentType, "image/png") {
		return fmt.Errorf("thumbnail content_type=%q want image/png", contentType)
	}
	return nil
}

func createImageRecord(ctx context.Context, base, token string) (string, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("text", fmt.Sprintf("monolift-thumb-%d", uploadCounter.Add(1))); err != nil {
		return "", "", err
	}
	fileHeader := make(textproto.MIMEHeader)
	fileHeader.Set("Content-Disposition", `form-data; name="`+uploadFileField+`"; filename="`+uploadFilename+`"`)
	fileHeader.Set("Content-Type", "image/png")
	part, err := writer.CreatePart(fileHeader)
	if err != nil {
		return "", "", err
	}
	imageData, err := uploadImage()
	if err != nil {
		return "", "", err
	}
	if _, err := part.Write(imageData); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/collections/"+uploadCollection+"/records", &body)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("POST /api/collections/%s/records status=%d body=%s", uploadCollection, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	var out struct {
		ID       string   `json:"id"`
		FileMany []string `json:"file_many"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", "", fmt.Errorf("decode create record response: %w: %s", err, strings.TrimSpace(string(respBody)))
	}
	if out.ID == "" || len(out.FileMany) == 0 || out.FileMany[0] == "" {
		return "", "", fmt.Errorf("create record response missing id/file_many: %s", strings.TrimSpace(string(respBody)))
	}
	return out.ID, out.FileMany[0], nil
}

func authToken(ctx context.Context, base string) (string, error) {
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

func uploadImage() ([]byte, error) {
	return os.ReadFile(harness.FromRepoRoot(uploadImagePath))
}

func pngDimensions(data []byte) (int, int, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func numericBodyValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}
