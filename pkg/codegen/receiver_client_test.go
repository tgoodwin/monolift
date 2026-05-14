package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// --- Plan constructors for receiver client golden-file tests ---

func receiverBoundaryValueClientPlan() *Plan {
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
		ClientPath: "/tmp/test/modules/caddyhttp/monolift_lift_funcmarkdown.go",
	}
}

func receiverFactoryPointerClientPlan() *Plan {
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
		ClientPath: "/tmp/test/modules/auth/password/hash/monolift_lift_hashwithsaltbytes.go",
	}
}

// --- Golden-file tests (2F.3-2F.4) ---

func TestRenderClientReceiverBoundaryValueGolden(t *testing.T) {
	plan := receiverBoundaryValueClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "receiver_boundary_value_client.go.golden")
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

func TestRenderClientReceiverFactoryPointerGolden(t *testing.T) {
	plan := receiverFactoryPointerClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "receiver_factory_pointer_client.go.golden")
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
