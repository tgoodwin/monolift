package activation_mattermost_publiclinkhash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	pingPath     = "/api/v4/system/ping"
	fileLinkPath = "/api/v4/files/link"
	testPassword = "Monolift123!"
)

type Workload struct{}

type idResponse struct {
	ID string `json:"id"`
}

type fileUploadResponse struct {
	FileInfos []idResponse `json:"file_infos"`
}

func (Workload) Setup(ctx context.Context, host string) error {
	deadline := time.Now().Add(3 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(host, "/")+pingPath, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("GET %s status=%d", pingPath, resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return lastErr
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, fileLinkPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{fileLinkPath}
}

func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	host = strings.TrimRight(host, "/")
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	email := "ml-" + suffix + "@example.com"
	username := "ml" + suffix
	if _, _, _, err := postJSON(ctx, host, "/api/v4/users", "", map[string]any{
		"email":    email,
		"username": username,
		"password": testPassword,
	}, nil); err != nil {
		return harness.Step{}, err
	}

	_, headers, _, err := postJSON(ctx, host, "/api/v4/users/login", "", map[string]any{
		"login_id": email,
		"password": testPassword,
	}, nil)
	if err != nil {
		return harness.Step{}, err
	}
	token := headers.Get("Token")
	if token == "" {
		return harness.Step{}, fmt.Errorf("login response missing Token header")
	}

	var team idResponse
	teamName := "mlt" + suffix
	if _, _, _, err := postJSON(ctx, host, "/api/v4/teams", token, map[string]any{
		"display_name": "Monolift " + suffix,
		"name":         teamName,
		"type":         "O",
	}, &team); err != nil {
		return harness.Step{}, err
	}
	if team.ID == "" {
		return harness.Step{}, fmt.Errorf("create team returned empty id")
	}

	var channel idResponse
	if _, _, _, err := postJSON(ctx, host, "/api/v4/channels", token, map[string]any{
		"team_id":      team.ID,
		"display_name": "Public Link " + suffix,
		"name":         "mlc" + suffix,
		"type":         "O",
	}, &channel); err != nil {
		return harness.Step{}, err
	}
	if channel.ID == "" {
		return harness.Step{}, fmt.Errorf("create channel returned empty id")
	}

	var upload fileUploadResponse
	if _, _, _, err := uploadFile(ctx, host, token, channel.ID, &upload); err != nil {
		return harness.Step{}, err
	}
	if len(upload.FileInfos) == 0 || upload.FileInfos[0].ID == "" {
		return harness.Step{}, fmt.Errorf("upload returned no file id")
	}
	fileID := upload.FileInfos[0].ID

	if _, _, _, err := postJSON(ctx, host, "/api/v4/posts", token, map[string]any{
		"channel_id": channel.ID,
		"message":    "monolift public link",
		"file_ids":   []string{fileID},
	}, nil); err != nil {
		return harness.Step{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, host+"/api/v4/files/"+fileID+"/link", nil)
	if err != nil {
		return harness.Step{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return harness.Step{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var linkResp map[string]string
	if len(body) > 0 {
		_ = json.Unmarshal(body, &linkResp)
	}
	link := linkResp["link"]
	return harness.Step{
		Method: http.MethodGet,
		Path:   path,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		BodyJSON: map[string]any{
			"link_public": strings.Contains(link, "/files/") && strings.Contains(link, "/public"),
			"link_hashed": strings.Contains(link, "?h="),
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

func postJSON(ctx context.Context, host, path, token string, payload any, out any) (int, http.Header, []byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+path, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Header, body, fmt.Errorf("POST %s status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp.StatusCode, resp.Header, body, fmt.Errorf("decode POST %s: %w", path, err)
		}
	}
	return resp.StatusCode, resp.Header, body, nil
}

func uploadFile(ctx context.Context, host, token, channelID string, out any) (int, http.Header, []byte, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("channel_id", channelID); err != nil {
		return 0, nil, nil, err
	}
	filePart, err := writer.CreateFormFile("files", "public-link.txt")
	if err != nil {
		return 0, nil, nil, err
	}
	if _, err := io.Copy(filePart, strings.NewReader("monolift mattermost public link\n")); err != nil {
		return 0, nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return 0, nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/api/v4/files", body)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return resp.StatusCode, resp.Header, respBody, fmt.Errorf("POST /api/v4/files status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return resp.StatusCode, resp.Header, respBody, fmt.Errorf("decode POST /api/v4/files: %w", err)
		}
	}
	return resp.StatusCode, resp.Header, respBody, nil
}
