package compiler

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestSSASpikeBuildsCHACallgraphAndPropagatesEnv(t *testing.T) {
	t.Parallel()

	withTag, withTagGraph, withTagDuration := loadSpikeProgram(t, "1", []string{"monoliftspike"})
	withoutTag, withoutTagGraph, withoutTagDuration := loadSpikeProgram(t, "0", nil)

	t.Logf("ssaspike LoadAllSyntax+SSA+CHA durations: cgo=1 tags=monoliftspike -> %s; cgo=0 no-tags -> %s", withTagDuration, withoutTagDuration)

	assertCompiledFilePresent(t, withTag, "tagged_on.go")
	assertCompiledFileAbsent(t, withTag, "tagged_off.go")
	assertCompiledFilePresent(t, withTag, "cgo_on.go")
	assertCompiledFileAbsent(t, withTag, "cgo_off.go")
	assertNonEmptyCallgraph(t, withTagGraph)

	assertCompiledFilePresent(t, withoutTag, "tagged_off.go")
	assertCompiledFileAbsent(t, withoutTag, "tagged_on.go")
	assertCompiledFilePresent(t, withoutTag, "cgo_off.go")
	assertCompiledFileAbsent(t, withoutTag, "cgo_on.go")
	assertNonEmptyCallgraph(t, withoutTagGraph)
}

func loadSpikeProgram(t *testing.T, cgoEnabled string, buildTags []string) (*packages.Package, int, time.Duration) {
	t.Helper()

	start := time.Now()
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax | packages.NeedModule,
		Dir:  filepath.Join("testdata", "ssaspike"),
		Env: append(
			os.Environ(),
			"CGO_ENABLED="+cgoEnabled,
			"GOOS="+runtime.GOOS,
			"GOARCH="+runtime.GOARCH,
		),
	}
	if len(buildTags) > 0 {
		cfg.BuildFlags = []string{"-tags=" + buildTags[0]}
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if count := packages.PrintErrors(pkgs); count != 0 {
		t.Fatalf("packages.Load returned %d package errors", count)
	}
	if len(pkgs) != 1 {
		t.Fatalf("packages.Load returned %d packages, want 1", len(pkgs))
	}

	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	graph := cha.CallGraph(prog)

	return pkgs[0], len(graph.Nodes), time.Since(start)
}

func assertCompiledFilePresent(t *testing.T, pkg *packages.Package, base string) {
	t.Helper()
	if !compiledFilesContain(pkg, base) {
		t.Fatalf("compiled files missing %s: %v", base, baseNames(pkg.CompiledGoFiles))
	}
}

func assertCompiledFileAbsent(t *testing.T, pkg *packages.Package, base string) {
	t.Helper()
	if compiledFilesContain(pkg, base) {
		t.Fatalf("compiled files unexpectedly contain %s: %v", base, baseNames(pkg.CompiledGoFiles))
	}
}

func assertNonEmptyCallgraph(t *testing.T, nodeCount int) {
	t.Helper()
	if nodeCount == 0 {
		t.Fatal("CHA callgraph is empty")
	}
}

func compiledFilesContain(pkg *packages.Package, base string) bool {
	return slices.Contains(baseNames(pkg.CompiledGoFiles), base)
}

func baseNames(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		out = append(out, filepath.Base(path))
	}
	return out
}
