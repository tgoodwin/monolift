package activation_pocketbase_s256challenge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	healthPath          = "/api/health"
	usersCollectionPath = "/api/collections/users"
	authMethodsPath     = "/api/collections/users/auth-methods"

	superuserEmail    = "admin@example.com"
	superuserPassword = "Monolift123!"
)

type Workload struct{}

// Setup waits for pocketbase, then enables OAuth2 with a PKCE provider on the
// users collection so that GET .../auth-methods computes a code challenge via
// the lifted S256Challenge. The seeding path (superuser auth + collection
// update) does not touch S256Challenge, so Setup succeeds even in fail-closed
// and env-off modes where the lifted symbol is unavailable.
func (Workload) Setup(ctx context.Context, host string) error {
	base := strings.TrimRight(host, "/")
	if err := waitForHealth(ctx, base); err != nil {
		return err
	}
	token, status, err := authTokenOrStatus(ctx, base)
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("superuser auth failed status=%d", status)
	}
	return enablePKCEProvider(ctx, base, token)
}

func (Workload) Action(ctx context.Context, host string) (harness.Transcript, error) {
	step, err := Workload{}.Request(ctx, host, authMethodsPath)
	if err != nil {
		return harness.Transcript{}, err
	}
	return harness.Transcript{Steps: []harness.Step{step}}, nil
}

func (Workload) Paths() []string {
	return []string{authMethodsPath}
}

// Request drives the public auth-methods route and self-verifies the lifted
// S256Challenge: the response carries both the (non-lifted) random codeVerifier
// and the codeChallenge it produced, so challenge == S256(verifier) proves the
// round trip without depending on the per-request random value. It records
// derived booleans rather than the random values so env-on/env-off/baseline
// transcripts compare equal, and it never hard-errors on an empty/mismatched
// challenge — in fail-closed mode the route still returns 200 with an empty
// challenge, which the fail-mode assertions require to be a non-5xx response.
func (Workload) Request(ctx context.Context, host, path string) (harness.Step, error) {
	if path != authMethodsPath {
		return harness.Step{}, fmt.Errorf("unsupported pocketbase workload path %s", path)
	}
	base := strings.TrimRight(host, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return harness.Step{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return harness.Step{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data struct {
		OAuth2 struct {
			Enabled   bool `json:"enabled"`
			Providers []struct {
				Name          string `json:"name"`
				CodeVerifier  string `json:"codeVerifier"`
				CodeChallenge string `json:"codeChallenge"`
			} `json:"providers"`
		} `json:"oauth2"`
	}
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &data); err != nil {
			return harness.Step{}, fmt.Errorf("decode auth-methods response: %w: %s", err, strings.TrimSpace(string(body)))
		}
	}

	providerPresent := len(data.OAuth2.Providers) > 0
	var verifier, challenge string
	if providerPresent {
		verifier = data.OAuth2.Providers[0].CodeVerifier
		challenge = data.OAuth2.Providers[0].CodeChallenge
	}
	challengeNonempty := challenge != ""
	challengeMatches := challengeNonempty && s256(verifier) == challenge

	return harness.Step{
		Method: http.MethodGet,
		Path:   authMethodsPath,
		Status: resp.StatusCode,
		Headers: map[string]string{
			"Content-Type": resp.Header.Get("Content-Type"),
		},
		BodyJSON: map[string]any{
			"provider_present":   providerPresent,
			"challenge_nonempty": challengeNonempty,
			"challenge_matches":  challengeMatches,
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

// s256 mirrors pocketbase's security.S256Challenge: the test module has no
// replace directive for pocketbase, so the package cannot be imported here. The
// rigorous byte-exact comparison lives in the in-cluster oracle (which does
// import it); this inline copy only powers the route-level self-check.
func s256(code string) string {
	sum := sha256.Sum256([]byte(code))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func enablePKCEProvider(ctx context.Context, base, token string) error {
	payload, err := json.Marshal(map[string]any{
		"oauth2": map[string]any{
			"enabled": true,
			"providers": []map[string]any{
				{"name": "google", "clientId": "e2e-dummy", "clientSecret": "e2e-dummy"},
			},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, base+usersCollectionPath, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("PATCH %s status=%d: %s", usersCollectionPath, resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return nil
}

func waitForHealth(ctx context.Context, base string) error {
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

func authTokenOrStatus(ctx context.Context, base string) (string, int, error) {
	var lastStatus int
	for attempt := 0; attempt < 30; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", 0, ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
		token, status, err := authTokenOnce(ctx, base)
		if err != nil {
			return "", 0, err
		}
		if token != "" {
			return token, 0, nil
		}
		lastStatus = status
	}
	return "", lastStatus, nil
}

func authTokenOnce(ctx context.Context, base string) (string, int, error) {
	payload, err := json.Marshal(map[string]string{
		"identity": superuserEmail,
		"password": superuserPassword,
	})
	if err != nil {
		return "", 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/collections/_superusers/auth-with-password", bytes.NewReader(payload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, nil
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		return "", 0, err
	}
	if data.Token == "" {
		return "", 0, fmt.Errorf("auth response missing token")
	}
	return data.Token, 0, nil
}
