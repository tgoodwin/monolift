package bootpath

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/surface"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestWalkToySources(t *testing.T) {
	prog, mainPkg, functions := loadBootSSA(t, map[string]string{"main.go": `package main
import (
	"flag"
	"os"
)

func main() {
	_ = os.Getenv("APP_ENV")
	_ = os.Getenv("MM_SQLSETTINGS_DATASOURCE")
	_ = flag.String("config", "config.json", "")
	_, _ = os.ReadFile("config.json")
	_, _ = os.Open("/etc/host-only-state")
	go regionLoop()
	regionRoot()
}

func regionLoop() {}
func regionRoot() {}
`})
	spec, err := Walk(prog, mainPkg, "toy", surface.RegionSurface{Category: surface.SurfaceCall}, functions)
	if err != nil {
		t.Fatal(err)
	}
	if countSources[EnvSource](spec.ConfigSources) != 2 {
		t.Fatalf("env sources=%+v", spec.ConfigSources)
	}
	if countSources[FlagSource](spec.ConfigSources) != 1 {
		t.Fatalf("flag sources=%+v", spec.ConfigSources)
	}
	if countSources[FileSource](spec.ConfigSources) != 2 {
		t.Fatalf("file sources=%+v", spec.ConfigSources)
	}
	if len(spec.GoroutineLaunches) != 1 {
		t.Fatalf("goroutines=%+v", spec.GoroutineLaunches)
	}
	if len(spec.Refusals) != 1 || spec.Refusals[0].Kind != RefusalUnportableLiteralPath {
		t.Fatalf("refusals=%+v", spec.Refusals)
	}
}

func TestWalkIgnoresGoroutineOutsideUnion(t *testing.T) {
	prog, mainPkg, functions := loadBootSSA(t, map[string]string{"main.go": `package main
func main() { go outside(); regionRoot() }
func outside() {}
func regionRoot() {}
`})
	var union []*ssa.Function
	for _, fn := range functions {
		if fn.Name() == "regionRoot" {
			union = append(union, fn)
		}
	}
	spec, err := Walk(prog, mainPkg, "toy", surface.RegionSurface{Category: surface.SurfaceCall}, union)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.GoroutineLaunches) != 0 {
		t.Fatalf("goroutines=%+v", spec.GoroutineLaunches)
	}
}

func TestWalkDeterministicOrdering(t *testing.T) {
	prog, mainPkg, functions := loadBootSSA(t, map[string]string{"main.go": `package main
import "os"
func main() { _ = os.Getenv("Z"); _ = os.Getenv("A"); regionRoot() }
func regionRoot() {}
`})
	first, err := Walk(prog, mainPkg, "toy", surface.RegionSurface{Category: surface.SurfaceCall}, functions)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Walk(prog, mainPkg, "toy", surface.RegionSurface{Category: surface.SurfaceCall}, functions)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfigSources[0].Identifier() != second.ConfigSources[0].Identifier() || first.ConfigSources[0].Identifier() != "A" {
		t.Fatalf("ordering first=%+v second=%+v", first.ConfigSources, second.ConfigSources)
	}
}

func TestWalkCaddyMainSmoke(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo, "evaluation", "caddy")
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./cmd/caddy")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("load failed")
	}
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	if len(ssaPkgs) == 0 {
		t.Fatal("no caddy cmd SSA package")
	}
	mainFn, _ := ssaPkgs[0].Members["main"].(*ssa.Function)
	spec, err := Walk(prog, ssaPkgs[0], "caddy-smoke", surface.RegionSurface{Category: surface.SurfaceCall}, []*ssa.Function{mainFn})
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.EntryPath) == 0 {
		t.Fatal("empty caddy boot entry path")
	}
}

func countSources[T ConfigSource](sources []ConfigSource) int {
	count := 0
	for _, source := range sources {
		if _, ok := source.(T); ok {
			count++
		}
	}
	return count
}

func loadBootSSA(t *testing.T, files map[string]string) (*ssa.Program, *ssa.Package, []*ssa.Function) {
	t.Helper()
	dir := t.TempDir()
	writeBootFile(t, dir, "go.mod", "module example.com/bootfixture\n\ngo 1.22\n")
	for name, src := range files {
		writeBootFile(t, dir, name, src)
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatal("load failed")
	}
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	var mainPkg *ssa.Package
	if len(ssaPkgs) > 0 {
		mainPkg = ssaPkgs[0]
	}
	var functions []*ssa.Function
	for fn := range ssautil.AllFunctions(prog) {
		if fn.Package() == mainPkg {
			functions = append(functions, fn)
		}
	}
	return prog, mainPkg, functions
}

func writeBootFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
