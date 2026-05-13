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

func streamingBytesServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-processstream",
		EnvServiceName:   "PROCESSSTREAM",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/stream",
			PackageName: "stream",
			FuncName:    "ProcessStream",
		},
		BoundaryParams: []Param{
			{Name: "baseURL", JSONName: "base_url", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "r", JSONName: "r", GoType: "io.ReadSeeker", QualifiedGoType: "io.ReadSeeker", TypePackagePath: "io", Codec: CodecStreamingBytes, Index: 1},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
		ServerPath:  "/tmp/test/cmd/monolift-processstream/main.go",
	}
}

func TestRenderServerStreamingBytesGolden(t *testing.T) {
	plan := streamingBytesServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "streaming_bytes_server.go.golden")
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
		t.Fatalf("rendered server does not match %s\ngot:\n%s", goldenPath, got)
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
