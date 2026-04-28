package liftpatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
)

var updateGolden = flag.Bool("update-golden", false, "update liftpatch golden files")

func TestPatchInjectsPrelude(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(s string) string { return s }\n"})
	result, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	if err != nil {
		t.Fatal(err)
	}
	if result.AlreadyApplied {
		t.Fatal("first patch reported AlreadyApplied")
	}
	fn := parseFunc(t, result.PatchedFile, "Foo")
	stmt, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("first statement is %T, want *ast.IfStmt", fn.Body.List[0])
	}
	ident, ok := stmt.Cond.(*ast.Ident)
	if !ok || ident.Name != "monoliftLiftEnabled" {
		t.Fatalf("condition = %#v, want monoliftLiftEnabled ident", stmt.Cond)
	}
}

func TestPatchIdempotentStructural(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(s string) string { return s }\n"})
	req := patchRequest(dir, "Foo", "func(string) string")
	first, err := PatchSymbolBody(req)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(first.PatchedFile)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PatchSymbolBody(req)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(first.PatchedFile)
	if err != nil {
		t.Fatal(err)
	}
	if !second.AlreadyApplied {
		t.Fatal("second patch did not report AlreadyApplied")
	}
	if !bytes.Equal(before, after) {
		t.Fatal("second patch changed file bytes")
	}
}

func TestPatchPreservesOriginalBody(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(s string) string {\ns = s + \"!\"\nreturn s\n}\n"})
	result, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	if err != nil {
		t.Fatal(err)
	}
	fn := parseFunc(t, result.PatchedFile, "Foo")
	if len(fn.Body.List) != 3 {
		t.Fatalf("body len=%d want 3", len(fn.Body.List))
	}
	if _, ok := fn.Body.List[1].(*ast.AssignStmt); !ok {
		t.Fatalf("stmt 1 = %T, want assign", fn.Body.List[1])
	}
	if _, ok := fn.Body.List[2].(*ast.ReturnStmt); !ok {
		t.Fatalf("stmt 2 = %T, want return", fn.Body.List[2])
	}
}

func TestPatchSignatureMismatch(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(i int) string { return \"\" }\n"})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticSignatureMismatch)
}

func TestPatchRefusesGenerics(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo[T any](s string) string { return s }\n"})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticGenericFunction)
}

func TestPatchRefusesReceiver(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\ntype T struct{}\nfunc (T) Foo(s string) string { return s }\n"})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticMethodReceiver)
}

func TestPatchRefusesNamedNakedReturn(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(s string) (out string) { out = s; return }\n"})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticNamedNakedReturn)
}

func TestPatchRefusesBuildTagDuplicate(t *testing.T) {
	dir := writePackage(t, map[string]string{
		"foo.go":        "package p\nfunc Foo(s string) string { return s }\n",
		"foo_tagged.go": "//go:build linux\n\npackage p\nfunc Foo(s string) string { return s }\n",
	})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticAmbiguousTarget)
}

func TestPatchMultiFilePackage(t *testing.T) {
	dir := writePackage(t, map[string]string{
		"bar.go": "package p\nfunc Bar() {}\n",
		"foo.go": "package p\nfunc Foo(s string) string { return s }\n",
	})
	if _, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string")); err != nil {
		t.Fatal(err)
	}

	dup := writePackage(t, map[string]string{
		"foo.go":  "package p\nfunc Foo(s string) string { return s }\n",
		"foo2.go": "package p\nfunc Foo(s string) string { return s }\n",
	})
	_, err := PatchSymbolBody(patchRequest(dup, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticAmbiguousTarget)
}

func TestPatchTargetNotFound(t *testing.T) {
	dir := writePackage(t, map[string]string{"bar.go": "package p\nfunc Bar() {}\n"})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticTargetNotFound)
}

func TestPatchScansForCollisions(t *testing.T) {
	dir := writePackage(t, map[string]string{
		"foo.go":       "package p\nfunc Foo(s string) string { return s }\n",
		"collision.go": "package p\nvar monoliftLiftFoo = true\n",
	})
	_, err := PatchSymbolBody(patchRequest(dir, "Foo", "func(string) string"))
	assertDiagnostic(t, err, DiagnosticIdentifierCollision)
}

func TestPatchEmitsLIFTPATCHJson(t *testing.T) {
	dir := writePackage(t, map[string]string{"foo.go": "package p\nfunc Foo(s string) string { return s }\n"})
	req := patchRequest(dir, "Foo", "func(string) string")
	result, err := PatchSymbolBody(req)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "LIFTPATCH.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest LiftPatchManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.PackageImportPath != req.PackageImportPath || manifest.FunctionName != "Foo" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.OriginalSHA256 == "" || manifest.PatchedSHA256 == "" || result.OriginalSHA256 == "" || result.PatchedSHA256 == "" {
		t.Fatalf("missing hashes: manifest=%+v result=%+v", manifest, result)
	}
	if len(manifest.GeneratedFiles) != 1 || !strings.HasSuffix(manifest.GeneratedFiles[0], "monolift_lift_foo.go") {
		t.Fatalf("unexpected generated files: %+v", manifest.GeneratedFiles)
	}
}

func TestPatchRegionMultiSymbolMultiReceiver(t *testing.T) {
	dir := writePackage(t, map[string]string{"root.go": `package p
type Alpha struct{}
type Beta struct{}
func (Alpha) Serve(s string) string { return "a:" + s }
func (Beta) Serve(s string) string { return "b:" + s }
`})
	shared := filepath.Join(dir, "monolift_region_client.go")
	result, err := PatchRegion(RegionPatchRequest{
		RegionName: "toy",
		Symbols: []PatchSymbolRequest{
			regionSymbol(dir, "Alpha", "Serve"),
			regionSymbol(dir, "Beta", "Serve"),
		},
		SharedGeneratedFiles: []GeneratedFile{{Path: shared, Content: []byte("package p\nfunc monoliftRegionClient(s string) (string, bool) { return s, true }\n")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Refused != nil {
		t.Fatalf("refused: %+v", result.Refused)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files len=%d want 2", len(result.Files))
	}
	if len(result.GeneratedFiles) != 1 || result.GeneratedFiles[0].Path != shared {
		t.Fatalf("generated=%+v", result.GeneratedFiles)
	}
	for _, receiver := range []string{"Alpha", "Beta"} {
		fn := parseFunc(t, filepath.Join(dir, "root.go"), "Serve")
		if fn == nil {
			t.Fatalf("%s Serve not patched", receiver)
		}
	}
}

func TestPatchRegionMultiPackageSentinelUniqueness(t *testing.T) {
	dirA := writePackage(t, map[string]string{"a.go": "package a\ntype Alpha struct{}\nfunc (Alpha) Serve(s string) string { return s }\n"})
	dirB := writePackage(t, map[string]string{"b.go": "package b\ntype Beta struct{}\nfunc (Beta) Serve(s string) string { return s }\n"})
	req := RegionPatchRequest{
		RegionName: "toy",
		Symbols: []PatchSymbolRequest{
			regionSymbolWithImport(dirA, "example.com/a", "Alpha", "Serve"),
			regionSymbolWithImport(dirB, "example.com/b", "Beta", "Serve"),
		},
	}
	if req.Symbols[0].SentinelIdent == req.Symbols[1].SentinelIdent {
		t.Fatal("sentinels should differ across package import paths")
	}
	result, err := PatchRegion(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Refused != nil {
		t.Fatalf("refused: %+v", result.Refused)
	}
	if len(result.Files) != 2 {
		t.Fatalf("files len=%d want 2", len(result.Files))
	}
}

func TestPatchRegionNegativeCases(t *testing.T) {
	t.Run("duplicate symbol identities", func(t *testing.T) {
		dir := writePackage(t, map[string]string{"root.go": "package p\ntype Alpha struct{}\nfunc (Alpha) Serve(s string) string { return s }\n"})
		req := RegionPatchRequest{RegionName: "toy", Symbols: []PatchSymbolRequest{regionSymbol(dir, "Alpha", "Serve"), regionSymbol(dir, "Alpha", "Serve")}}
		assertRegionRefusal(t, req, DiagnosticAmbiguousTarget)
	})
	t.Run("signature mismatch", func(t *testing.T) {
		dir := writePackage(t, map[string]string{"root.go": "package p\ntype Alpha struct{}\nfunc (Alpha) Serve(i int) string { return \"\" }\n"})
		assertRegionRefusal(t, RegionPatchRequest{RegionName: "toy", Symbols: []PatchSymbolRequest{regionSymbol(dir, "Alpha", "Serve")}}, DiagnosticSignatureMismatch)
	})
	t.Run("generated-file collision", func(t *testing.T) {
		dir := writePackage(t, map[string]string{"root.go": "package p\ntype Alpha struct{}\nfunc (Alpha) Serve(s string) string { return s }\n"})
		path := filepath.Join(dir, "generated.go")
		req := RegionPatchRequest{
			RegionName: "toy",
			Symbols: []PatchSymbolRequest{{
				PackageImportPath: "example.com/p",
				PackageDir:        dir,
				FuncName:          "Serve",
				ReceiverType:      "Alpha",
				ExpectedSignature: "func(string) string",
				Prelude:           PreludeSpec{GoSource: "return s"},
				GeneratedFiles:    []GeneratedFile{{Path: path, Content: []byte("package p\n")}},
			}},
			SharedGeneratedFiles: []GeneratedFile{{Path: path, Content: []byte("package p\nvar X = 1\n")}},
		}
		assertRegionRefusal(t, req, DiagnosticIdentifierCollision)
	})
	t.Run("receiver mismatch", func(t *testing.T) {
		dir := writePackage(t, map[string]string{"root.go": "package p\ntype Alpha struct{}\nfunc (Alpha) Serve(s string) string { return s }\n"})
		symbol := regionSymbol(dir, "Beta", "Serve")
		assertRegionRefusal(t, RegionPatchRequest{RegionName: "toy", Symbols: []PatchSymbolRequest{symbol}}, DiagnosticTargetNotFound)
	})
}

func TestRenderLiftClient(t *testing.T) {
	artifact, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	data := artifact.Files["monolift_lift_cleanpath.go"]
	formatted, err := format.Source(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, formatted) {
		t.Fatal("rendered lift client is not gofmt clean")
	}
	file, err := parser.ParseFile(token.NewFileSet(), "monolift_lift_cleanpath.go", data, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !hasValue(file, "monoliftLiftClient") || !hasValue(file, "monoliftLiftEnabled") || !hasConst(file, "monoliftLiftFailureSentinel") {
		t.Fatalf("rendered lift client missing required package-level declarations")
	}
	text := string(data)
	for _, needle := range []string{"&http.Client", "os.Getenv", "MONOLIFT_LIFT_CLEANPATH", "func monoliftLiftCleanPath(p string, collapseSlashes bool) (string, bool)"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("rendered lift client missing %q", needle)
		}
	}
	if len(artifact.HostPatchOps) != 1 {
		t.Fatalf("HostPatchOps len=%d want 1", len(artifact.HostPatchOps))
	}
	if artifact.HostPatchOps[0].ExpectedSignature != "func(string, bool) string" {
		t.Fatalf("signature=%q", artifact.HostPatchOps[0].ExpectedSignature)
	}
}

func TestRenderLiftClientUsesIntFailureSentinel(t *testing.T) {
	artifact, err := Render(estimateReadingTimeContext())
	if err != nil {
		t.Fatal(err)
	}
	data := artifact.Files["monolift_lift_estimatereadingtime.go"]
	if _, err := format.Source(data); err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, needle := range []string{
		"const monoliftLiftFailureSentinel = -1",
		"func monoliftLiftEstimateReadingTime(content string, defaultReadingSpeed int, cjkReadingSpeed int) (int, bool)",
	} {
		if !strings.Contains(text, needle) {
			t.Fatalf("rendered int lift client missing %q\n%s", needle, data)
		}
	}
	if strings.Contains(text, `"\x00MONOLIFT_LIFT_FAILED\x00"`) {
		t.Fatalf("rendered int lift client contains string sentinel\n%s", data)
	}
	if artifact.HostPatchOps[0].ExpectedSignature != "func(string, int, int) int" {
		t.Fatalf("signature=%q", artifact.HostPatchOps[0].ExpectedSignature)
	}
	if !strings.Contains(artifact.HostPatchOps[0].PreludeSource, "return monoliftLiftFailureSentinel") {
		t.Fatalf("prelude missing failure sentinel return: %s", artifact.HostPatchOps[0].PreludeSource)
	}
}

func TestRenderMatchesGoldens(t *testing.T) {
	artifact, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 0, len(artifact.Files))
	for path := range artifact.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		data := artifact.Files[path]
		golden := filepath.Join("testdata", "caddyhttp", filepath.FromSlash(path))
		if *updateGolden {
			if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(golden, data, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, want) {
			t.Fatalf("%s differs from golden; run go test ./pkg/compiler/transport/emit/liftpatch -run TestRenderMatchesGoldens -update-golden", path)
		}
	}
}

func writePackage(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func patchRequest(dir, name, signature string) PatchRequest {
	generated := filepath.Join(dir, "monolift_lift_"+strings.ToLower(name)+".go")
	return PatchRequest{
		ModuleRoot:        filepath.Dir(dir),
		PackageImportPath: "example.com/p",
		PackageDir:        dir,
		FuncName:          name,
		ExpectedSignature: signature,
		PreludeSpec: PreludeSpec{GoSource: `if monoliftLiftEnabled {
	if result, ok := monoliftLiftFoo(s); ok {
		return result
	}
	if !monoliftLiftFailOpen {
		return monoliftLiftFailureSentinel
	}
}`},
		GeneratedFiles: []GeneratedFile{{Path: generated, Content: []byte("package p\nfunc monoliftLiftFoo(s string) (string, bool) { return s, true }\n")}},
		SentinelIdent:  "monoliftLiftEnabled",
	}
}

func regionSymbol(dir, receiver, name string) PatchSymbolRequest {
	return regionSymbolWithImport(dir, "example.com/p", receiver, name)
}

func regionSymbolWithImport(dir, importPath, receiver, name string) PatchSymbolRequest {
	sentinel := regionSentinel("toy", importPath)
	return PatchSymbolRequest{
		PackageImportPath: importPath,
		PackageDir:        dir,
		FuncName:          name,
		ReceiverType:      receiver,
		ExpectedSignature: "func(string) string",
		Prelude: PreludeSpec{GoSource: `if ` + sentinel + ` {
	if result, ok := monoliftRegionClient(s); ok {
		return result
	}
}`},
		SentinelIdent: sentinel,
	}
}

func parseFunc(t *testing.T, path, name string) *ast.FuncDecl {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("%s not found", name)
	return nil
}

func assertRegionRefusal(t *testing.T, req RegionPatchRequest, want DiagnosticKind) {
	t.Helper()
	result, err := PatchRegion(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Refused == nil {
		t.Fatalf("refused=nil want %s", want)
	}
	if result.Refused.Kind != want {
		t.Fatalf("refusal=%s want %s (%s)", result.Refused.Kind, want, result.Refused.Message)
	}
}

func assertDiagnostic(t *testing.T, err error, want DiagnosticKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("err=nil want %s", want)
	}
	var diagnostic *DiagnosticError
	if !errors.As(err, &diagnostic) {
		t.Fatalf("err=%T %v, want DiagnosticError", err, err)
	}
	if diagnostic.Kind != want {
		t.Fatalf("diagnostic=%s want %s", diagnostic.Kind, want)
	}
}

func hasValue(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for _, ident := range value.Names {
				if ident.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func hasConst(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for _, ident := range value.Names {
				if ident.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func cleanPathContext() emit.Context {
	return emit.Context{
		SymbolImportPath: "github.com/caddyserver/caddy/v2/modules/caddyhttp",
		ObjectName:       "CleanPath",
		ParamFields: []emit.FieldSpec{
			{Name: "P", JSONName: "p", GoType: "string"},
			{Name: "CollapseSlashes", JSONName: "collapse_slashes", GoType: "bool"},
		},
		ResultFields: []emit.FieldSpec{
			{Name: "Result", JSONName: "result", GoType: "string"},
		},
		ServiceName:  "monolift-extracted-cleanpath",
		EnvVarPrefix: "MONOLIFT_LIFT_CLEANPATH",
	}
}

func estimateReadingTimeContext() emit.Context {
	return emit.Context{
		SymbolImportPath: "miniflux.app/v2/internal/reader/readingtime",
		ObjectName:       "EstimateReadingTime",
		ParamFields: []emit.FieldSpec{
			{Name: "Content", JSONName: "content", GoType: "string"},
			{Name: "DefaultReadingSpeed", JSONName: "default_reading_speed", GoType: "int"},
			{Name: "CjkReadingSpeed", JSONName: "cjk_reading_speed", GoType: "int"},
		},
		ResultFields: []emit.FieldSpec{
			{Name: "ReadingTime", JSONName: "reading_time", GoType: "int"},
		},
		ServiceName:  "monolift-extracted-estimatereadingtime",
		EnvVarPrefix: "MONOLIFT_LIFT_ESTIMATEREADINGTIME",
	}
}
