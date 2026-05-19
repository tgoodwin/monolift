package codegen

import (
	"os"
	"path/filepath"
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

// TestAdapterExtractionRespectsTransportPolicy proves that the rendered
// inline-payload guard is read from plan.AdapterPlan.MaxInlinePayloadBytes
// rather than a hardcoded literal. SPRINT-0052 task 1.7.
func TestAdapterExtractionRespectsTransportPolicy(t *testing.T) {
	tmp := t.TempDir()
	plan := processImageAdapterPlan(tmp, filepath.Join(tmp, "media.go"))
	plan.AdapterPlan.MaxInlinePayloadBytes = 1024 // intentionally non-default

	limit := adapterInlinePayloadLimit(plan.AdapterPlan)
	if limit != 1024 {
		t.Fatalf("adapterInlinePayloadLimit = %d, want 1024", limit)
	}
	lines := adapterExtractionLines(plan, normalizedAdapterPlan(plan))
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "if len(input) > 1024 {") {
		t.Fatalf("extraction lines did not honor MaxInlinePayloadBytes:\n%s", joined)
	}
	if !strings.Contains(joined, "1024 byte limit") {
		t.Fatalf("extraction error message did not honor MaxInlinePayloadBytes:\n%s", joined)
	}
	if strings.Contains(joined, "8388608") || strings.Contains(joined, "8*1024*1024") {
		t.Fatalf("extraction lines still contain the legacy 8 MiB literal:\n%s", joined)
	}

	// Zero on the plan falls back to defaultInlinePayloadBytes (8 MiB) so
	// legacy plans round-trip identically.
	plan.AdapterPlan.MaxInlinePayloadBytes = 0
	limit = adapterInlinePayloadLimit(plan.AdapterPlan)
	if limit != int64(defaultInlinePayloadBytes) {
		t.Fatalf("adapterInlinePayloadLimit fallback = %d, want %d", limit, defaultInlinePayloadBytes)
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

func TestRenderAdapterProcessImageWrapperAndNormalizedHelper(t *testing.T) {
	tmp := t.TempDir()
	cutFile := filepath.Join(tmp, "media.go")
	if err := os.WriteFile(cutFile, []byte(`package media

import (
	"bytes"
	"mime/multipart"

	"github.com/disintegration/imaging"
)

const thumbnailSize = 250

func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {
	src, err := file.Open()
	if err != nil {
		return nil, 0, 0, err
	}
	defer src.Close()

	img, err := imaging.Decode(src)
	if err != nil {
		return nil, 0, 0, err
	}

	var (
		thumb = imaging.Resize(img, thumbnailSize, 0, imaging.Lanczos)
		out   bytes.Buffer
	)
	if err := imaging.Encode(&out, thumb, imaging.PNG); err != nil {
		return nil, 0, 0, err
	}

	b := img.Bounds().Max
	return bytes.NewReader(out.Bytes()), b.X, b.Y, nil
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	plan := processImageAdapterPlan(tmp, cutFile)
	clientFiles, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	client := string(clientFiles[plan.ClientPath])
	for _, want := range []string{
		"func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error)",
		"input, err := io.ReadAll(fileSrc)",
		"if len(input) > 8388608 {",
		`fmt.Errorf("monolift: adapter payload exceeds 8388608 byte limit")`,
		"r0, r1, r2, appErr, transportErr := monoliftRemoteprocessImage(input)",
		"return bytes.NewReader(r0), r1, r2, appErr",
		"return nil, 0, 0, fmt.Errorf(\"monolift: extracted service unavailable\")",
	} {
		if !strings.Contains(client, want) {
			t.Fatalf("client missing %q:\n%s", want, client)
		}
	}
	serverFiles, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	server := string(serverFiles[plan.ServerPath])
	if !strings.Contains(server, "Input []byte `json:\"input\"`") {
		t.Fatalf("server request was not normalized:\n%s", server)
	}
	if !strings.Contains(server, "Thumbnail      []byte `json:\"thumbnail\"`") {
		t.Fatalf("server response DTO was not normalized:\n%s", server)
	}
	adapterFiles, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}
	adapter := string(adapterFiles[AdapterFilePath(plan)])
	if !strings.Contains(adapter, "func MonoliftInvokeProcessImage(input []byte) ([]byte, int, int, error)") {
		t.Fatalf("adapter invoke signature was not normalized:\n%s", adapter)
	}
	if !strings.Contains(adapter, "return monoliftNormalizedprocessImage(input)") {
		t.Fatalf("adapter does not call normalized helper:\n%s", adapter)
	}
	helper := string(adapterFiles[NormalizedHelperFilePath(plan)])
	if !strings.Contains(helper, "func monoliftNormalizedprocessImage(input []byte) ([]byte, int, int, error)") {
		t.Fatalf("normalized helper signature missing:\n%s", helper)
	}
	if !strings.Contains(helper, "img, err := imaging.Decode(bytes.NewReader(input))") {
		t.Fatalf("normalized helper did not rewrite decode prologue:\n%s", helper)
	}
	if strings.Contains(helper, "file.Open()") || strings.Contains(helper, "defer src.Close()") || strings.Contains(helper, "return bytes.NewReader(out.Bytes())") {
		t.Fatalf("normalized helper still contains awkward boundary operations:\n%s", helper)
	}
}

func processImageAdapterPlan(dir, cutFile string) *Plan {
	plan := &Plan{
		SourceModuleRoot: "/tmp/test",
		ServiceName:      "monolift-processimage",
		EnvServiceName:   "PROCESSIMAGE",
		CutPoint: CutPoint{
			PackageName: "media",
			PackagePath: "example.com/listmonk/internal/media",
			PackageDir:  dir,
			FuncName:    "processImage",
			File:        cutFile,
		},
		BoundaryParams: []Param{
			{Name: "file", JSONName: "file", GoType: "*multipart.FileHeader", QualifiedGoType: "*mime/multipart.FileHeader", TypePackagePath: "mime/multipart", Codec: CodecJSON, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "*bytes.Reader", QualifiedGoType: "*bytes.Reader", TypePackagePath: "bytes", Codec: CodecJSON, Index: 0},
			{Name: "result1", JSONName: "result1", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "result2", JSONName: "result2", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 2},
			{Name: "err", JSONName: "error", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 3},
		},
		ClientPath: filepath.Join(dir, "monolift_lift_PROCESSIMAGE.go"),
		ServerPath: filepath.Join(dir, "cmd", "monolift-processimage", "main.go"),
		AdapterPlan: &AdapterPlan{
			SourceFunction:  "processImage",
			HostSignature:   "(*multipart.FileHeader) (*bytes.Reader, int, int, error)",
			RemoteSignature: "([]byte) ([]byte, int, int, error)",
			InputTransforms: []AdapterPattern{{
				Name:      "multipart_file_read_all",
				ParamName: "file",
				FromType:  "*multipart.FileHeader",
				ToType:    "[]byte",
			}},
			OutputTransforms: []AdapterPattern{{
				Name:     "bytes_reader_return",
				FromType: "*bytes.Reader",
				ToType:   "[]byte",
			}},
			TransportPolicy: AdapterTransportInlineJSONBytes,
		},
	}
	normalized := normalizedAdapterPlan(plan)
	plan.ResultDTO = normalized.ResultDTO
	return plan
}
