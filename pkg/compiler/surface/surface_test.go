package surface

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestDeriveCallSessionAndAsync(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/surfacetest\n\ngo 1.22\n")
	writeFile(t, dir, "root.go", `package surfacetest

import "net/http"

func CleanPath(p string, collapseSlashes bool) string { return p }

func EstimateReadingTime(content string, a, b int) int { return len(content) / a }

func PocketBaseHook(name string) string { return name }

func HijackHandler(w http.ResponseWriter, r *http.Request) {
	h, ok := w.(http.Hijacker)
	if !ok { return }
	conn, _, err := h.Hijack()
	if err != nil { return }
	defer conn.Close()
}

func Async(in <-chan string) {}
`)
	prog := loadSSA(t, dir)
	all := allFunctions(prog)
	for _, name := range []string{"CleanPath", "EstimateReadingTime", "PocketBaseHook"} {
		surface, err := Derive(root("example.com/surfacetest", name), all)
		if err != nil {
			t.Fatalf("Derive(%s): %v", name, err)
		}
		if surface.Category != SurfaceCall || surface.WireProtocol != WireProtocolHTTPJSON {
			t.Fatalf("%s category=%s protocol=%s", name, surface.Category, surface.WireProtocol)
		}
	}
	session, err := Derive(root("example.com/surfacetest", "HijackHandler"), all)
	if err != nil {
		t.Fatalf("Derive(HijackHandler): %v", err)
	}
	if session.Category != SurfaceSession || session.WireProtocol != WireProtocolStreamProxy {
		t.Fatalf("session category=%s protocol=%s", session.Category, session.WireProtocol)
	}
	async, err := Derive(root("example.com/surfacetest", "Async"), all)
	if err != nil {
		t.Fatalf("Derive(Async): %v", err)
	}
	if len(async.Refusals) != 1 || async.Refusals[0].Code != DiagnosticAsyncUnsupported {
		t.Fatalf("async refusals=%+v", async.Refusals)
	}
}

func root(pkgPath, object string) reportv2.Root {
	return reportv2.Root{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  pkgPath,
			PackagePath: pkgPath,
			ObjectName:  object,
			Kind:        "function",
		},
	}
}

func loadSSA(t *testing.T, dir string) *ssa.Program {
	t.Helper()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("package load failed")
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	return prog
}

func allFunctions(prog *ssa.Program) []*ssa.Function {
	var out []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		out = append(out, fn)
	}
	return out
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
