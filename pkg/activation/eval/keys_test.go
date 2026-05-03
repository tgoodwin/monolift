package eval

import (
	"path/filepath"
	"testing"
)

func TestTraceStepFunctionKey(t *testing.T) {
	root := repoRoot(t)
	cases, err := EnumerateCases(
		filepath.Join(root, "docs/research/activation-paths/traces"),
		filepath.Join(root, "evaluation/MANIFEST.yaml"),
		filepath.Join(root, "evaluation"),
	)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Case{}
	for _, c := range cases {
		byID[c.Trace.ID] = c
	}
	tests := []struct {
		id       string
		step     int
		pkgPath  string
		receiver string
		name     string
	}{
		{"miniflux/M-1", 1, "miniflux.app/v2/internal/cli", "", "Parse"},
		{"listmonk/M-1", 4, "github.com/knadh/listmonk/internal/manager", "*Manager", "NewCampaignMessage"},
		{"caddy/M-1", 13, "github.com/caddyserver/caddy/v2/modules/caddyhttp/templates", "TemplateContext", "funcMarkdown"},
		{"mattermost/M-1", 10, "github.com/mattermost/mattermost/server/v8/platform/services/docextractor", "", "Extract"},
	}
	for _, tt := range tests {
		c := byID[tt.id]
		key, err := TraceStepFunctionKey(c.Trace.Steps[tt.step], c.Target)
		if err != nil {
			t.Fatalf("%s step %d: %v", tt.id, tt.step, err)
		}
		if key.PackagePath != tt.pkgPath || key.Receiver != tt.receiver || key.FuncName != tt.name {
			t.Fatalf("%s step %d key = %+v", tt.id, tt.step, key)
		}
	}
}
