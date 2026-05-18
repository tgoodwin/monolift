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

// --- DTO golden-file plan constructors (SPRINT-0051 Phase 2) ---

// (T, U, error) shape: two non-error results + error, needs DTO.
func dtoTUErrorServerPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-parse",
		EnvServiceName:   "PARSE",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/parser",
			PackageName: "parser",
			FuncName:    "Parse",
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "name", JSONName: "name", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "count", JSONName: "count", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 2},
		},
		ServerPath: "/tmp/test/cmd/monolift-parse/main.go",
	}
	plan.ResultDTO = BuildResultDTO("Parse", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

func dtoTUErrorClientPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-parse",
		EnvServiceName:   "PARSE",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/parser",
			PackageName: "parser",
			FuncName:    "Parse",
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "name", JSONName: "name", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "count", JSONName: "count", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 2},
		},
		ClientPath: "/tmp/test/internal/parser/monolift_lift_PARSE.go",
	}
	plan.ResultDTO = BuildResultDTO("Parse", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

// ([]byte, int, int, error) shape: the M-4 processImage shape.
func dtoM4ServerPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-processimage",
		EnvServiceName:   "PROCESSIMAGE",
		SourceModulePath: "example.com/listmonk",
		CutPoint: CutPoint{
			PackagePath: "example.com/listmonk/internal/media",
			PackageName: "media",
			FuncName:    "ProcessImage",
		},
		BoundaryParams: []Param{
			{Name: "srcData", JSONName: "src_data", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "typ", JSONName: "typ", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 1},
		},
		Results: []Result{
			{Name: "data", JSONName: "data", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "width", JSONName: "width", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "height", JSONName: "height", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 2},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 3},
		},
		ServerPath: "/tmp/test/cmd/monolift-processimage/main.go",
	}
	plan.ResultDTO = BuildResultDTO("ProcessImage", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

func dtoM4ClientPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-processimage",
		EnvServiceName:   "PROCESSIMAGE",
		SourceModulePath: "example.com/listmonk",
		CutPoint: CutPoint{
			PackagePath: "example.com/listmonk/internal/media",
			PackageName: "media",
			FuncName:    "ProcessImage",
		},
		BoundaryParams: []Param{
			{Name: "srcData", JSONName: "src_data", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "typ", JSONName: "typ", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 1},
		},
		Results: []Result{
			{Name: "data", JSONName: "data", GoType: "[]byte", QualifiedGoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "width", JSONName: "width", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "height", JSONName: "height", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 2},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 3},
		},
		ClientPath: "/tmp/test/internal/media/monolift_lift_PROCESSIMAGE.go",
	}
	plan.ResultDTO = BuildResultDTO("ProcessImage", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

// (T, T) shape: two non-error results, no error.
func dtoTwoNonErrorServerPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-split",
		EnvServiceName:   "SPLIT",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/util",
			PackageName: "util",
			FuncName:    "Split",
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "first", JSONName: "first", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "second", JSONName: "second", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 1},
		},
		ServerPath: "/tmp/test/cmd/monolift-split/main.go",
	}
	plan.ResultDTO = BuildResultDTO("Split", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

func dtoTwoNonErrorClientPlan() *Plan {
	plan := &Plan{
		ServiceName:      "monolift-split",
		EnvServiceName:   "SPLIT",
		SourceModulePath: "example.com/test",
		CutPoint: CutPoint{
			PackagePath: "example.com/test/internal/util",
			PackageName: "util",
			FuncName:    "Split",
		},
		BoundaryParams: []Param{
			{Name: "input", JSONName: "input", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		Results: []Result{
			{Name: "first", JSONName: "first", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "second", JSONName: "second", GoType: "string", QualifiedGoType: "string", Codec: CodecPrimitive, Index: 1},
		},
		ClientPath: "/tmp/test/internal/util/monolift_lift_SPLIT.go",
	}
	plan.ResultDTO = BuildResultDTO("Split", plan.Results)
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: plan.ResultDTO.Name}
	return plan
}

// --- DTO golden-file tests ---

func TestRenderServerDTOTUErrorGolden(t *testing.T) {
	plan := dtoTUErrorServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "dto_tu_error_server.go.golden")
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

func TestRenderClientDTOTUErrorGolden(t *testing.T) {
	plan := dtoTUErrorClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "dto_tu_error_client.go.golden")
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

func TestRenderServerDTOM4Golden(t *testing.T) {
	plan := dtoM4ServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "dto_m4_processimage_server.go.golden")
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

func TestRenderClientDTOM4Golden(t *testing.T) {
	plan := dtoM4ClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "dto_m4_processimage_client.go.golden")
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

func TestRenderServerDTOTwoNonErrorGolden(t *testing.T) {
	plan := dtoTwoNonErrorServerPlan()
	files, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ServerPath]
	goldenPath := filepath.Join("testdata", "dto_two_nonerror_server.go.golden")
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

func TestRenderClientDTOTwoNonErrorGolden(t *testing.T) {
	plan := dtoTwoNonErrorClientPlan()
	files, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	got := files[plan.ClientPath]
	goldenPath := filepath.Join("testdata", "dto_two_nonerror_client.go.golden")
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
