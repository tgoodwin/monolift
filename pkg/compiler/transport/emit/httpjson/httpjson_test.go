package httpjson

import (
	"bytes"
	"errors"
	"flag"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/transport"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
)

var updateGolden = flag.Bool("update-golden", false, "update httpjson golden files")

func TestRenderImportsRealSymbol(t *testing.T) {
	t.Parallel()

	mainGo := renderedMain(t)
	file := parseGo(t, mainGo)
	if !hasImport(file, "github.com/caddyserver/caddy/v2/modules/caddyhttp") {
		t.Fatal("rendered main.go does not import caddyhttp")
	}
	if !hasCleanPathCall(file) {
		t.Fatal("rendered main.go does not call caddyhttp.CleanPath")
	}
}

func TestRenderRejectsSyntheticBody(t *testing.T) {
	t.Parallel()

	err := ValidateNoSyntheticBody([]byte(`package main
func CleanPath(p string, collapseSlashes bool) string { return cleanPath(p) }
`))
	if err == nil {
		t.Fatal("synthetic CleanPath body was accepted")
	}
}

func TestCounterIncrementsBeforeRealCall(t *testing.T) {
	t.Parallel()

	file := parseGo(t, renderedMain(t))
	var sawOrdering bool
	ast.Inspect(file, func(node ast.Node) bool {
		fn, ok := node.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "handleInvoke" {
			return true
		}
		addIndex := -1
		callIndex := -1
		for i, stmt := range fn.Body.List {
			ast.Inspect(stmt, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isSelector(call.Fun, "atomic", "AddInt64") {
					addIndex = i
				}
				if isSelector(call.Fun, "caddyhttp", "CleanPath") {
					callIndex = i
				}
				return true
			})
		}
		sawOrdering = addIndex >= 0 && callIndex >= 0 && addIndex < callIndex
		return false
	})
	if !sawOrdering {
		t.Fatal("atomic.AddInt64 does not precede caddyhttp.CleanPath in handleInvoke")
	}
}

func TestRenderProducesGofmtClean(t *testing.T) {
	t.Parallel()

	artifact, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range artifact.Files {
		if filepath.Ext(path) != ".go" {
			continue
		}
		formatted, err := format.Source(data)
		if err != nil {
			t.Fatalf("format %s: %v", path, err)
		}
		if !bytes.Equal(data, formatted) {
			t.Fatalf("%s is not gofmt clean", path)
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	t.Parallel()

	first, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	second, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Files) != len(second.Files) {
		t.Fatalf("file count mismatch")
	}
	for path, firstData := range first.Files {
		if !bytes.Equal(firstData, second.Files[path]) {
			t.Fatalf("%s differs across renders", path)
		}
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	t.Parallel()

	_, err := emit.Emit(transport.Selection{Template: transport.TemplateHandler}, cleanPathContext())
	if !errors.Is(err, emit.ErrTemplateUnsupported) {
		t.Fatalf("err=%v want ErrTemplateUnsupported", err)
	}
}

func TestRenderGoBuild(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	artifact, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	for path, data := range artifact.Files {
		out := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(filepath.Join(root, "upstream"), os.DirFS(filepath.Join(repoRoot(t), "evaluation", "caddy"))); err != nil {
		t.Fatalf("CopyFS upstream: %v", err)
	}
	cmd := exec.Command("go", "build", "-mod=mod", "./...")
	cmd.Dir = filepath.Join(root, "extracted-cleanpath")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, output)
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
		golden := filepath.Join("testdata", "cleanpath", filepath.FromSlash(path))
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
			t.Fatalf("%s differs from golden; run go test ./pkg/compiler/transport/emit/httpjson -run TestRenderMatchesGoldens -update-golden", path)
		}
	}
}

func renderedMain(t *testing.T) []byte {
	t.Helper()
	artifact, err := Render(cleanPathContext())
	if err != nil {
		t.Fatal(err)
	}
	return artifact.Files["extracted-cleanpath/main.go"]
}

func cleanPathContext() emit.Context {
	return emit.Context{
		SymbolImportPath:   "github.com/caddyserver/caddy/v2/modules/caddyhttp",
		ObjectName:         "CleanPath",
		ParamFields:        []emit.FieldSpec{{Name: "P", JSONName: "p", GoType: "string"}, {Name: "CollapseSlashes", JSONName: "collapse_slashes", GoType: "bool"}},
		ResultFields:       []emit.FieldSpec{{Name: "Result", JSONName: "result", GoType: "string"}},
		UpstreamModulePath: "github.com/caddyserver/caddy/v2",
		UpstreamLocalPath:  "../upstream",
		ServiceName:        "monolift-extracted-cleanpath",
		EnvVarPrefix:       "MONOLIFT_LIFT_CLEANPATH",
	}
}

func parseGo(t *testing.T, src []byte) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", src, 0)
	if err != nil {
		t.Fatalf("ParseFile: %v\n%s", err, src)
	}
	return file
}

func hasImport(file *ast.File, path string) bool {
	for _, spec := range file.Imports {
		if spec.Path.Value == `"`+path+`"` {
			return true
		}
	}
	return false
}

func hasCleanPathCall(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && isSelector(call.Fun, "caddyhttp", "CleanPath") {
			found = true
			return false
		}
		return true
	})
	return found
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repo root not found")
		}
		dir = parent
	}
}
