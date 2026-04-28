package entrypath

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestVTAFallbackTrigger(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "interface_dispatch"))
	result, err := Probe(prog, mainPkg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.CallgraphAlgorithm != "rta+vta" {
		t.Fatalf("algorithm = %q, want rta+vta", result.Stats.CallgraphAlgorithm)
	}
	if !hasDiagnostic(result.Diagnostics, "vta_fallback_used") {
		t.Fatalf("missing vta_fallback_used diagnostic: %+v", result.Diagnostics)
	}
}

func TestReverseBFSFindsCallers(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "reverse_bfs"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	result, err := Probe(prog, mainPkg, []*ssa.Function{root})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FunctionCount == 0 || result.Stats.StaticEdgeCount == 0 || result.Stats.PeakRSSBytes == 0 {
		t.Fatalf("stats not populated: %+v", result.Stats)
	}
	if !hasTouchpoint(result.RegionTouchpoints, "caller") {
		t.Fatalf("missing caller touchpoint: %+v", result.RegionTouchpoints)
	}
	if !hasTouchpoint(result.RegionTouchpoints, "main") {
		t.Fatalf("missing main touchpoint: %+v", result.RegionTouchpoints)
	}
}

func loadFixtureProgram(t *testing.T, dir string) (*ssa.Program, *ssa.Package) {
	t.Helper()
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Dir:   dir,
		Tests: false,
		Env:   append(os.Environ(), "GOTOOLCHAIN=local"),
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("packages.Load errors for %s", dir)
	}
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	for _, pkg := range ssaPkgs {
		if pkg != nil && pkg.Pkg != nil && pkg.Pkg.Name() == "main" {
			return prog, pkg
		}
	}
	t.Fatalf("main SSA package not found for %s", dir)
	return nil, nil
}

func hasDiagnostic(diags []Diagnostic, kind string) bool {
	for _, diag := range diags {
		if diag.Kind == kind {
			return true
		}
	}
	return false
}

func hasTouchpoint(touchpoints []RegionTouchpoint, objectName string) bool {
	for _, touchpoint := range touchpoints {
		if touchpoint.Touchpoint.Identity.ObjectName == objectName {
			return true
		}
	}
	return false
}
