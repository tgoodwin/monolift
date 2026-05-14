package codegen

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// --- Plan constructors for golden-file tests ---

func stringErrorServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-sanitizehtml",
		EnvServiceName:   "SANITIZEHTML",
		SourceModulePath: "miniflux.app/v2",
		CutPoint: CutPoint{
			PackagePath: "miniflux.app/v2/internal/reader/sanitizer",
			PackageName: "sanitizer",
			FuncName:    "SanitizeHTML",
		},
		BoundaryParams: []Param{
			{Name: "rawHTML", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 1},
		},
		ServerPath: "/tmp/test/cmd/monolift-sanitizehtml/main.go",
	}
}

func stringErrorClientPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-sanitizehtml",
		EnvServiceName:   "SANITIZEHTML",
		SourceModulePath: "miniflux.app/v2",
		CutPoint: CutPoint{
			PackagePath: "miniflux.app/v2/internal/reader/sanitizer",
			PackageName: "sanitizer",
			FuncName:    "SanitizeHTML",
		},
		BoundaryParams: []Param{
			{Name: "rawHTML", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 1},
		},
		ClientPath: "/tmp/test/internal/reader/sanitizer/monolift_lift_SANITIZEHTML.go",
	}
}

func boolServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-validate",
		EnvServiceName:   "VALIDATE",
		SourceModulePath: "github.com/pocketbase/pocketbase",
		CutPoint: CutPoint{
			PackagePath: "github.com/pocketbase/pocketbase/core",
			PackageName: "core",
			FuncName:    "Validate",
		},
		BoundaryParams: []Param{
			{Name: "value", JSONName: "value", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "bool", QualifiedGoType: "bool", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "bool"},
		ServerPath:  "/tmp/test/cmd/monolift-validate/main.go",
	}
}

func voidServerPlan() *Plan {
	return &Plan{
		ServiceName:      "monolift-process",
		EnvServiceName:   "PROCESS",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/worker",
			PackageName: "worker",
			FuncName:    "Process",
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results:    nil,
		ServerPath: "/tmp/test/cmd/monolift-process/main.go",
	}
}

// --- Golden-file tests (2D.7-2D.10) ---

func TestRenderServerStringErrorGolden(t *testing.T) {
	plan := stringErrorServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "multireturn_string_error_server.go.golden")
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

func TestRenderClientStringErrorGolden(t *testing.T) {
	plan := stringErrorClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "multireturn_string_error_client.go.golden")
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

func TestRenderServerBoolGolden(t *testing.T) {
	plan := boolServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "multireturn_bool_server.go.golden")
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

func TestRenderServerVoidGolden(t *testing.T) {
	plan := voidServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "multireturn_void_server.go.golden")
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

// --- Round-trip tests (2D.11-2D.12) ---

// multiReturnResponse mirrors the generated invokeResponse for (string, error).
type multiReturnResponse struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

func TestMultiReturnRoundTripNilError(t *testing.T) {
	// Server side: function returned ("hello", nil)
	resp := multiReturnResponse{Result: "hello", Error: ""}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	// Client side: decode
	var decoded multiReturnResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Result != "hello" {
		t.Fatalf("expected result 'hello', got %q", decoded.Result)
	}
	if decoded.Error != "" {
		t.Fatalf("expected no error, got %q", decoded.Error)
	}

	// Reconstruct Go return values
	var appErr error
	if decoded.Error != "" {
		appErr = errors.New(decoded.Error)
	}
	if appErr != nil {
		t.Fatalf("expected nil error, got %v", appErr)
	}
}

func TestMultiReturnRoundTripNonNilError(t *testing.T) {
	// Server side: function returned ("", errors.New("bad input"))
	resp := multiReturnResponse{Result: "", Error: "bad input"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	// Client side: decode
	var decoded multiReturnResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Result != "" {
		t.Fatalf("expected empty result, got %q", decoded.Result)
	}
	if decoded.Error != "bad input" {
		t.Fatalf("expected error 'bad input', got %q", decoded.Error)
	}

	// Reconstruct Go return values
	var appErr error
	if decoded.Error != "" {
		appErr = errors.New(decoded.Error)
	}
	if appErr == nil {
		t.Fatal("expected non-nil error")
	}
	if appErr.Error() != "bad input" {
		t.Fatalf("expected error message 'bad input', got %q", appErr.Error())
	}
}
