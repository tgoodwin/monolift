package codegen

import (
	"strings"
	"testing"
)

// TestClientTemplatesShareTransportPlumbing is the golden cross-check for
// SPRINT-0052 task 2.8 (B-16). clientTemplate and adapterClientTemplate are a
// deliberate fork (different view types, different result-shape handling), but
// the transport/plumbing they share — endpoint resolution, request encoding,
// the HTTP POST headers and client, the status check, response decode, and the
// env-gate / fail-mode handling — MUST stay byte-identical so the fork cannot
// silently drift in the part that actually talks to the extracted service.
// Each line below is asserted present, verbatim, in both templates; editing one
// without the other fails this test.
func TestClientTemplatesShareTransportPlumbing(t *testing.T) {
	shared := []string{
		`endpoint := os.Getenv("{{ .EndpointEnv }}")`,
		`endpoint = "{{ .DefaultEndpoint }}"`,
		`var body bytes.Buffer`,
		`if err := json.NewEncoder(&body).Encode(payload); err != nil {`,
		`req, err := http.NewRequest(http.MethodPost, endpoint, &body)`,
		`req.Header.Set("Content-Type", "application/json")`,
		`client := &http.Client{Timeout: 30 * time.Second}`,
		`resp, err := client.Do(req)`,
		`defer resp.Body.Close()`,
		`if resp.StatusCode < 200 || resp.StatusCode >= 300 {`,
		`var decoded monoliftInvokeResponse`,
		`if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {`,
		`os.Getenv("{{ .EnabledEnv }}") != "on"`,
		`os.Getenv("MONOLIFT_LIFT_FAILMODE") == "closed"`,
	}
	for _, line := range shared {
		if !strings.Contains(clientTemplate, line) {
			t.Errorf("clientTemplate is missing shared transport line:\n  %s", line)
		}
		if !strings.Contains(adapterClientTemplate, line) {
			t.Errorf("adapterClientTemplate is missing shared transport line:\n  %s", line)
		}
	}
}
