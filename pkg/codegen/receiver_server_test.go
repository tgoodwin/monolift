package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// --- Plan constructors for receiver server golden-file tests ---

func receiverBoundaryValueServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-funcmarkdown",
		EnvServiceName:   "FUNCMARKDOWN",
		SourceModulePath: "github.com/caddyserver/caddy/v2",
		CutPoint: CutPoint{
			PackagePath: "github.com/caddyserver/caddy/v2/modules/caddyhttp",
			PackageName: "caddyhttp",
			FuncName:    "funcMarkdown",
			Receiver:    "TemplateContext",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:    "TemplateContext",
			IsPointer: false,
			Policy:    ReceiverBoundary,
			Codec:     CodecJSON,
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "any", QualifiedGoType: "any", Codec: CodecJSON, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 1},
		},
		ServerPath: "/tmp/test/cmd/monolift-funcmarkdown/main.go",
	}
}

func receiverFactoryPointerServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-hashwithsaltbytes",
		EnvServiceName:   "HASHWITHSALTBYTES",
		SourceModulePath: "code.gitea.io/gitea",
		CutPoint: CutPoint{
			PackagePath: "code.gitea.io/gitea/modules/auth/password/hash",
			PackageName: "hash",
			FuncName:    "HashWithSaltBytes",
			Receiver:    "*Argon2Hasher",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:      "*Argon2Hasher",
			IsPointer:   true,
			Policy:      ReceiverFactory,
			FactoryFunc: "NewArgon2Hasher",
		},
		BoundaryParams: []Param{
			{Name: "password", JSONName: "password", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "salt", JSONName: "salt", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 1},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 1},
		},
		ServerPath: "/tmp/test/cmd/monolift-hashwithsaltbytes/main.go",
	}
}

// --- Golden-file tests (2E.5-2E.6) ---

func TestRenderServerReceiverBoundaryValueGolden(t *testing.T) {
	plan := receiverBoundaryValueServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "receiver_boundary_value_server.go.golden")
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

func TestRenderServerReceiverFactoryPointerGolden(t *testing.T) {
	plan := receiverFactoryPointerServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "receiver_factory_pointer_server.go.golden")
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
