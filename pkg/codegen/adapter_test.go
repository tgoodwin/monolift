package codegen

import (
	"strings"
	"testing"
)

func TestRenderAdapterUnexportedMethod(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{
			PackageName: "caddyhttp",
			PackagePath: "github.com/caddyserver/caddy/v2/modules/caddyhttp",
			PackageDir:  "/tmp/test/modules/caddyhttp",
			FuncName:    "funcMarkdown",
			Receiver:    "TemplateContext",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:    "TemplateContext",
			IsPointer: false,
			Policy:    ReceiverBoundary,
		},
		BoundaryParams: []Param{
			{Name: "input", GoType: "any", Codec: CodecJSON, Index: 0},
		},
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "err", GoType: "error", Codec: CodecPrimitive, Index: 1},
		},
		EnvServiceName: "FUNCMARKDOWN",
	}

	files, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}

	adapterPath := AdapterFilePath(plan)
	content, ok := files[adapterPath]
	if !ok {
		t.Fatalf("adapter not found at %s", adapterPath)
	}

	src := string(content)

	// Adapter must be exported with correct name.
	if !strings.Contains(src, "func MonoliftInvokeFuncMarkdown(") {
		t.Fatalf("expected exported MonoliftInvokeFuncMarkdown, got:\n%s", src)
	}

	// Receiver flattened to first parameter.
	if !strings.Contains(src, "recv TemplateContext") {
		t.Fatalf("expected receiver param 'recv TemplateContext', got:\n%s", src)
	}

	// Calls renamed original on the receiver.
	if !strings.Contains(src, "recv.monoliftOriginalfuncMarkdown(input)") {
		t.Fatalf("expected call to recv.monoliftOriginalfuncMarkdown(input), got:\n%s", src)
	}

	// Multi-return type.
	if !strings.Contains(src, "(string, error)") {
		t.Fatalf("expected (string, error) return type, got:\n%s", src)
	}

	// Package declaration.
	if !strings.Contains(src, "package caddyhttp") {
		t.Fatalf("expected package caddyhttp, got:\n%s", src)
	}
}

func TestRenderAdapterExportedFunction(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{
			PackageName: "sanitizer",
			PackagePath: "miniflux.app/v2/internal/reader/sanitizer",
			PackageDir:  "/tmp/test/internal/reader/sanitizer",
			FuncName:    "SanitizeHTML",
		},
		BoundaryParams: []Param{
			{Name: "rawHTML", GoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		EnvServiceName: "SANITIZEHTML",
	}

	files, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}

	adapterPath := AdapterFilePath(plan)
	content, ok := files[adapterPath]
	if !ok {
		t.Fatalf("adapter not found at %s", adapterPath)
	}

	src := string(content)

	// Adapter must be exported with correct name.
	if !strings.Contains(src, "func MonoliftInvokeSanitizeHTML(") {
		t.Fatalf("expected MonoliftInvokeSanitizeHTML, got:\n%s", src)
	}

	// No receiver parameter for standalone functions.
	if strings.Contains(src, "recv ") {
		t.Fatalf("expected no receiver param for standalone function, got:\n%s", src)
	}

	// Calls renamed original directly.
	if !strings.Contains(src, "monoliftOriginalSanitizeHTML(rawHTML)") {
		t.Fatalf("expected call to monoliftOriginalSanitizeHTML(rawHTML), got:\n%s", src)
	}

	// Single return type (no parens).
	if !strings.Contains(src, ") string {") {
		t.Fatalf("expected single string return type, got:\n%s", src)
	}

	// Package declaration.
	if !strings.Contains(src, "package sanitizer") {
		t.Fatalf("expected package sanitizer, got:\n%s", src)
	}
}

func TestRenderAdapterPointerReceiver(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{
			PackageName: "hash",
			PackagePath: "code.gitea.io/gitea/modules/auth/password/hash",
			PackageDir:  "/tmp/test/modules/auth/password/hash",
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
			{Name: "password", GoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "salt", GoType: "[]byte", Codec: CodecJSON, Index: 1},
		},
		Results: []Result{
			{Name: "result", GoType: "[]byte", Codec: CodecJSON, Index: 0},
		},
		EnvServiceName: "HASHWITHSALTBYTES",
	}

	files, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}

	adapterPath := AdapterFilePath(plan)
	content, ok := files[adapterPath]
	if !ok {
		t.Fatalf("adapter not found at %s", adapterPath)
	}

	src := string(content)

	// Pointer receiver as first param.
	if !strings.Contains(src, "recv *Argon2Hasher") {
		t.Fatalf("expected pointer receiver param, got:\n%s", src)
	}

	// Calls renamed method on receiver.
	if !strings.Contains(src, "recv.monoliftOriginalHashWithSaltBytes(password, salt)") {
		t.Fatalf("expected method call on receiver, got:\n%s", src)
	}

	// Exported adapter name.
	if !strings.Contains(src, "func MonoliftInvokeHashWithSaltBytes(") {
		t.Fatalf("expected MonoliftInvokeHashWithSaltBytes, got:\n%s", src)
	}
}

func TestAdapterFuncName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"funcMarkdown", "MonoliftInvokeFuncMarkdown"},
		{"SanitizeHTML", "MonoliftInvokeSanitizeHTML"},
		{"HashWithSaltBytes", "MonoliftInvokeHashWithSaltBytes"},
		{"", "MonoliftInvoke"},
	}
	for _, tt := range tests {
		got := adapterFuncName(tt.input)
		if got != tt.want {
			t.Errorf("adapterFuncName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
