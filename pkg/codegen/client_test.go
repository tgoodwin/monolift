package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func streamingBytesClientPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-readcontent",
		EnvServiceName:   "READCONTENT",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/stream",
			PackageName: "stream",
			FuncName:    "ReadContent",
		},
		BoundaryParams: []Param{
			{Name: "r", JSONName: "r", GoType: "io.Reader", QualifiedGoType: "io.Reader", TypePackagePath: "io", Codec: CodecStreamingBytes, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
		ClientPath:  "/tmp/test/internal/stream/monolift_lift_readcontent.go",
	}
}

func TestRenderClientStreamingBytesGolden(t *testing.T) {
	plan := streamingBytesClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "streaming_bytes_client.go.golden")
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
		t.Fatalf("rendered client does not match %s\ngot:\n%s", goldenPath, got)
	}
}

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
