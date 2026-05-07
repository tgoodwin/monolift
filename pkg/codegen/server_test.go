package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRenderServerRefreshFeedGolden(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "miniflux_refreshfeed_server.go.golden")
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
		t.Fatalf("rendered server does not match %s", goldenPath)
	}
}

func TestRenderedRefreshFeedServerGoVet(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := os.MkdirTemp(plan.SourceModuleRoot, ".monolift-vet-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)
	plan.OutputDir = tmp
	plan.ServerPath = filepath.Join(tmp, "cmd", plan.ServiceName, "main.go")
	plan.ManifestPath = filepath.Join(tmp, ManifestName)
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := artifactsFromRendered("server", files)
	if _, err := WriteArtifacts(plan, artifacts, ""); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "vet", "./cmd/"+plan.ServiceName)
	cmd.Dir = tmp
	cmd.Env = append(os.Environ(), "GOCACHE=/tmp/monolift-gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go vet generated server: %v\n%s", err, out)
	}
}
