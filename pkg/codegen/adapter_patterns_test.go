package codegen

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/activation"
	"golang.org/x/tools/go/ssa"
)

// parseFuncBody parses a standalone function declaration and returns its body
// block plus the FileSet, for exercising the pattern body rewriters directly.
func parseFuncBody(t *testing.T, funcSrc string) (*ast.BlockStmt, *token.FileSet) {
	t.Helper()
	src := "package p\n" + strings.TrimSpace(funcSrc) + "\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse func body: %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
			return fn.Body, fset
		}
	}
	t.Fatal("no function declaration found")
	return nil, nil
}

func printBody(t *testing.T, body *ast.BlockStmt, fset *token.FileSet) string {
	t.Helper()
	var sb strings.Builder
	if err := printer.Fprint(&sb, fset, body); err != nil {
		t.Fatalf("print body: %v", err)
	}
	return sb.String()
}

// TestMultipartRewriteInputBody covers the pattern-owned AST rewrite for
// multipart_file_read_all: the matched prologue is removed and uses of the
// opened reader become bytes.NewReader(input); mismatched shapes refuse
// rather than partial-apply. SPRINT-0052 task 1.2.
func TestMultipartRewriteInputBody(t *testing.T) {
	pattern := multipartFileReadAllPattern{}

	t.Run("matched prologue rewrites and removes Open/guard/defer", func(t *testing.T) {
		body, fset := parseFuncBody(t, `
func f(file *multipart.FileHeader) ([]byte, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return io.ReadAll(src)
}`)
		if !pattern.rewriteInputBody(body, "file", "input") {
			t.Fatal("rewriteInputBody returned false on matched prologue")
		}
		out := printBody(t, body, fset)
		if strings.Contains(out, "file.Open()") || strings.Contains(out, "src.Close()") {
			t.Fatalf("prologue not removed:\n%s", out)
		}
		if !strings.Contains(out, "io.ReadAll(bytes.NewReader(input))") {
			t.Fatalf("reader use not rewritten:\n%s", out)
		}
	})

	t.Run("no Open call refuses", func(t *testing.T) {
		body, _ := parseFuncBody(t, `
func f(file *multipart.FileHeader) error {
	_ = file.Filename
	return nil
}`)
		if pattern.rewriteInputBody(body, "file", "input") {
			t.Fatal("rewriteInputBody should refuse when there is no Open() prologue")
		}
	})

	t.Run("Open without following err guard refuses", func(t *testing.T) {
		body, _ := parseFuncBody(t, `
func f(file *multipart.FileHeader) ([]byte, error) {
	src, err := file.Open()
	defer src.Close()
	_ = err
	return io.ReadAll(src)
}`)
		if pattern.rewriteInputBody(body, "file", "input") {
			t.Fatal("rewriteInputBody should refuse when the Open() error guard is missing")
		}
	})
}

// TestBytesReaderRewriteOutputBody covers the pattern-owned AST rewrite for
// bytes_reader_return: bytes.NewReader(X) return slots become X; absence of
// such a slot refuses. SPRINT-0052 task 1.2.
func TestBytesReaderRewriteOutputBody(t *testing.T) {
	pattern := bytesReaderReturnPattern{}

	t.Run("rewrites bytes.NewReader return slot", func(t *testing.T) {
		body, fset := parseFuncBody(t, `
func f() (*bytes.Reader, int, error) {
	var out bytes.Buffer
	return bytes.NewReader(out.Bytes()), 7, nil
}`)
		if !pattern.rewriteOutputBody(body) {
			t.Fatal("rewriteOutputBody returned false on bytes.NewReader return")
		}
		out := printBody(t, body, fset)
		if strings.Contains(out, "bytes.NewReader(") {
			t.Fatalf("bytes.NewReader not unwrapped:\n%s", out)
		}
		if !strings.Contains(out, "return out.Bytes(), 7, nil") {
			t.Fatalf("return slot not rewritten to []byte:\n%s", out)
		}
	})

	t.Run("no bytes.NewReader return refuses", func(t *testing.T) {
		body, _ := parseFuncBody(t, `
func f(r *bytes.Reader) (*bytes.Reader, error) {
	return r, nil
}`)
		if pattern.rewriteOutputBody(body) {
			t.Fatal("rewriteOutputBody should refuse when no return slot is bytes.NewReader(...)")
		}
	})
}

// loadAdapterSSA loads Go source into an SSA program scoped to a single
// package. Used by the adapter-pattern tests to feed synthetic helper
// functions through TryAdapterPass. The source must declare a package main
// (the simplest standalone unit) and contain the function under test.
func loadAdapterSSA(t *testing.T, source string) *ssa.Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module adaptertest\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(strings.TrimSpace(source)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := activation.Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	program.BuildSSA()
	return program.SSAProgram
}

func findSSAFunction(t *testing.T, prog *ssa.Program, name string) *ssa.Function {
	t.Helper()
	for _, pkg := range prog.AllPackages() {
		if pkg == nil {
			continue
		}
		for _, member := range pkg.Members {
			if fn, ok := member.(*ssa.Function); ok && fn.Name() == name {
				return fn
			}
		}
	}
	t.Fatalf("function %q not found in SSA program", name)
	return nil
}

// TestTryAdapterPass_ProcessImageGolden is the load-bearing integration
// test for Phase 3.8. A synthetic stand-in for listmonk's processImage
// (same signature, same body shape) is fed through TryAdapterPass, and
// the resulting AdapterPlan is compared against a committed golden JSON.
func TestTryAdapterPass_ProcessImageGolden(t *testing.T) {
	prog := loadAdapterSSA(t, processImageSyntheticSource)
	fn := findSSAFunction(t, prog, "processImage")

	plan, refusals := TryAdapterPass(AdapterContext{
		Fn:               fn,
		FunctionExported: false,
	})
	if len(refusals) > 0 {
		t.Fatalf("TryAdapterPass refused processImage: %+v", refusals)
	}
	if plan == nil {
		t.Fatal("TryAdapterPass returned nil plan")
	}

	got, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	goldenPath := filepath.Join("testdata", "adapter_processimage_plan.golden.json")
	if shouldUpdateGoldens() {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(got, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDENS=1 to regenerate)", goldenPath, err)
	}
	if strings.TrimSpace(string(want)) != strings.TrimSpace(string(got)) {
		t.Fatalf("AdapterPlan diverged from golden %s\n--- want ---\n%s\n--- got ---\n%s", goldenPath, want, got)
	}
}

func shouldUpdateGoldens() bool {
	v := strings.TrimSpace(os.Getenv("UPDATE_GOLDENS"))
	return v == "1" || strings.EqualFold(v, "true")
}

// processImageSyntheticSource mirrors evaluation/listmonk/cmd/media.go's
// processImage in shape and SSA producers. Uses bytes.Buffer / bytes.NewReader
// and a multipart.FileHeader so the patterns can match without depending on
// the imaging library.
const processImageSyntheticSource = `
package main

import (
	"bytes"
	"io"
	"mime/multipart"
)

func processImage(file *multipart.FileHeader) (*bytes.Reader, int, int, error) {
	src, err := file.Open()
	if err != nil {
		return nil, 0, 0, err
	}
	defer src.Close()

	raw, err := io.ReadAll(src)
	if err != nil {
		return nil, 0, 0, err
	}

	var out bytes.Buffer
	if _, err := out.Write(raw); err != nil {
		return nil, 0, 0, err
	}
	return bytes.NewReader(out.Bytes()), len(raw), len(raw) / 2, nil
}

func main() {
	_ = processImage
}
`

// TestTryAdapterPass_MultipartInputAccepted exercises the positive path for
// the multipart_file_read_all pattern in isolation, asserting both the
// input transform and the use-shape proof.
func TestTryAdapterPass_MultipartInputAccepted(t *testing.T) {
	src := `
package main

import (
	"io"
	"mime/multipart"
)

func helper(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func main() { _ = helper }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "helper")
	plan, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: false})
	if len(refusals) > 0 {
		t.Fatalf("expected acceptance, got refusals: %+v", refusals)
	}
	if plan == nil || len(plan.InputTransforms) != 1 {
		t.Fatalf("expected one input transform, got plan=%+v", plan)
	}
	if plan.InputTransforms[0].Name != "multipart_file_read_all" {
		t.Errorf("input transform = %s, want multipart_file_read_all", plan.InputTransforms[0].Name)
	}
	requireProofSatisfied(t, plan.Proofs, RefusalAdapterUseShape)
	requireProofSatisfied(t, plan.Proofs, RefusalAdapterFiniteInput)
	requireProofSatisfied(t, plan.Proofs, RefusalAdapterLocalLifecycle)
}

// TestTryAdapterPass_BytesReaderReturnAccepted exercises the positive path
// for the bytes_reader_return pattern.
func TestTryAdapterPass_BytesReaderReturnAccepted(t *testing.T) {
	src := `
package main

import "bytes"

func makeReader(data []byte) (*bytes.Reader, error) {
	return bytes.NewReader(data), nil
}

func main() { _ = makeReader }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "makeReader")
	plan, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: false})
	if len(refusals) > 0 {
		t.Fatalf("expected acceptance, got refusals: %+v", refusals)
	}
	if plan == nil || len(plan.OutputTransforms) != 1 {
		t.Fatalf("expected one output transform, got plan=%+v", plan)
	}
	if plan.OutputTransforms[0].Name != "bytes_reader_return" {
		t.Errorf("output transform = %s, want bytes_reader_return", plan.OutputTransforms[0].Name)
	}
	requireProofSatisfied(t, plan.Proofs, RefusalAdapterReturnRehydration)
}

// TestTryAdapterPass_NegativeFixtures captures the must-refuse cases from
// Phase 3.6: multiple Open, Filename/Header/Size access, *multipart.File
// in returns, io.Writer params, channels, *os.File, function values.
func TestTryAdapterPass_NegativeFixtures(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		funcName     string
		wantRefusal  string
		wantContains string
	}{
		{
			name: "multiple Open calls",
			source: `
package main

import (
	"io"
	"mime/multipart"
)

func bad(file *multipart.FileHeader) ([]byte, error) {
	f1, err := file.Open()
	if err != nil { return nil, err }
	defer f1.Close()
	f2, err := file.Open()
	if err != nil { return nil, err }
	defer f2.Close()
	_ = f1
	return io.ReadAll(f2)
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalAdapterUseShape,
			wantContains: "Open() 2 times",
		},
		{
			// SPRINT-0052 task 2.6 (B-14): the param is captured by a goroutine
			// closure. Before following the FreeVar this passed (the direct
			// Open in the body satisfied the count and the MakeClosure was
			// ignored); now the capture is detected and refused.
			name: "goroutine capture of FileHeader refused",
			source: `
package main

import (
	"io"
	"mime/multipart"
)

func bad(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil { return nil, err }
	defer f.Close()
	go func() {
		_, _ = file.Open()
	}()
	return io.ReadAll(f)
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalAdapterUseShape,
			wantContains: "closure or goroutine",
		},
		{
			name: "Filename field access on FileHeader",
			source: `
package main

import "mime/multipart"

func bad(file *multipart.FileHeader) (string, error) {
	return file.Filename, nil
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalAdapterUseShape,
			wantContains: "field on *multipart.FileHeader",
		},
		{
			name: "io.Writer output parameter refused as live proxy",
			source: `
package main

import "io"

func bad(w io.Writer, data []byte) error {
	_, err := w.Write(data)
	return err
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalLiveProxyRequired,
			wantContains: "io.Writer",
		},
		{
			name: "channel parameter refused as live proxy",
			source: `
package main

func bad(ch chan int) error {
	ch <- 1
	return nil
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalLiveProxyRequired,
			wantContains: "channel",
		},
		{
			name: "*os.File refused as live proxy",
			source: `
package main

import "os"

func bad(f *os.File) error {
	return f.Close()
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalLiveProxyRequired,
			wantContains: "*os.File",
		},
		{
			name: "function-value parameter refused as adapter_impossible",
			source: `
package main

func bad(callback func() error) error {
	return callback()
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalAdapterImpossible,
			wantContains: "function-valued",
		},
		{
			name: "http.ResponseWriter refused as live proxy",
			source: `
package main

import "net/http"

func bad(w http.ResponseWriter) {
	w.WriteHeader(200)
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalLiveProxyRequired,
			wantContains: "http.ResponseWriter",
		},
		{
			name: "*bytes.Reader returned from non-NewReader producer refuses",
			source: `
package main

import "bytes"

var stash *bytes.Reader

func bad(b []byte) (*bytes.Reader, error) {
	if stash != nil {
		return stash, nil
	}
	stash = bytes.NewReader(b)
	return stash, nil
}

func main() { _ = bad }
`,
			funcName:     "bad",
			wantRefusal:  RefusalAdapterReturnRehydration,
			wantContains: "unrecognized producer",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			prog := loadAdapterSSA(t, tt.source)
			fn := findSSAFunction(t, prog, tt.funcName)
			plan, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: false})
			if plan != nil {
				t.Fatalf("expected refusal, got plan: %+v", plan)
			}
			if len(refusals) == 0 {
				t.Fatal("expected at least one refusal, got none")
			}
			found := false
			for _, r := range refusals {
				if r.Code == tt.wantRefusal {
					found = true
					if tt.wantContains != "" && !strings.Contains(r.Message, tt.wantContains) {
						t.Errorf("refusal message %q does not contain %q", r.Message, tt.wantContains)
					}
					break
				}
			}
			if !found {
				t.Fatalf("refusals=%+v, want code %s", refusals, tt.wantRefusal)
			}
		})
	}
}

// TestPatternMatches_Types confirms each pattern's type predicate matches
// only the intended shape.
func TestPatternMatches_Types(t *testing.T) {
	prog := loadAdapterSSA(t, `
package main

import (
	"bytes"
	"io"
	"mime/multipart"
)

type withFileHeader struct{ f *multipart.FileHeader }
type withReader struct{ r *bytes.Reader }

func probe(a *multipart.FileHeader, b *bytes.Reader, c io.Reader) {}
func main() { probe(nil, nil, nil) }
`)
	probe := findSSAFunction(t, prog, "probe")
	params := probe.Signature.Params()
	multipart := multipartFileReadAllPattern{}
	bytesReader := bytesReaderReturnPattern{}
	if !multipart.Matches(params.At(0).Type()) {
		t.Error("multipart_file_read_all should match *multipart.FileHeader")
	}
	if multipart.Matches(params.At(1).Type()) {
		t.Error("multipart_file_read_all should NOT match *bytes.Reader")
	}
	if !bytesReader.Matches(params.At(1).Type()) {
		t.Error("bytes_reader_return should match *bytes.Reader")
	}
	if bytesReader.Matches(params.At(2).Type()) {
		t.Error("bytes_reader_return should NOT match io.Reader")
	}
}

// TestTryAdapterPass_CallSiteObligation_ExportedHelper checks that an
// exported helper without a supplied call-site set is refused on the
// adapter_call_site obligation (cannot prove no function-value use).
func TestTryAdapterPass_CallSiteObligation_ExportedHelper(t *testing.T) {
	src := `
package main

import (
	"io"
	"mime/multipart"
)

func ProcessImage(file *multipart.FileHeader) ([]byte, error) {
	f, err := file.Open()
	if err != nil { return nil, err }
	defer f.Close()
	return io.ReadAll(f)
}

func main() { _ = ProcessImage }
`
	prog := loadAdapterSSA(t, src)
	fn := findSSAFunction(t, prog, "ProcessImage")
	_, refusals := TryAdapterPass(AdapterContext{Fn: fn, FunctionExported: true})
	if len(refusals) == 0 || refusals[0].Code != RefusalAdapterCallSite {
		t.Fatalf("expected adapter_call_site refusal for exported helper without call-site set, got %+v", refusals)
	}
}

func requireProofSatisfied(t *testing.T, proofs []AdapterProof, obligation string) {
	t.Helper()
	for _, p := range proofs {
		if p.Obligation == obligation {
			if !p.Satisfied {
				t.Errorf("obligation %s not satisfied: %s", obligation, p.Detail)
			}
			return
		}
	}
	t.Errorf("obligation %s missing from proof set %+v", obligation, proofs)
}
