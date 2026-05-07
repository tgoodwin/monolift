package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderClientRefreshFeedGolden(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "miniflux_refreshfeed_client.go.golden")
	if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("rendered client does not match %s", goldenPath)
	}
}
