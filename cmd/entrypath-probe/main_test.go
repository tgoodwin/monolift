package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/entrypath"
	"golang.org/x/tools/go/ssa"
)

func TestResolveRegionRootsExactMethodRoot(t *testing.T) {
	prog := loadProbeFixtureProgram(t)

	roots, stats, diagnostics, err := resolveRegionRoots(prog, []string{"example.com/rootresolution.(*Exact).Root"})
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || functionObjectName(roots[0]) != "(*Exact).Root" {
		t.Fatalf("roots = %v, want exact method root", roots)
	}
	if stats.MatchedSpecs != 1 || stats.FastPathHits != 1 || stats.FallbackHits != 0 {
		t.Fatalf("unexpected stats for exact root: %+v", stats)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics for exact root: %+v", diagnostics)
	}
}

func TestResolveRegionRootsBareMethodRootUsesFallback(t *testing.T) {
	prog := loadProbeFixtureProgram(t)

	roots, stats, diagnostics, err := resolveRegionRoots(prog, []string{"(*Bare).Root"})
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 || functionObjectName(roots[0]) != "(*Bare).Root" {
		t.Fatalf("roots = %v, want bare method root", roots)
	}
	if stats.MatchedSpecs != 1 || stats.FastPathHits != 0 || stats.FallbackHits != 1 {
		t.Fatalf("unexpected stats for bare root: %+v", stats)
	}
	if !hasProbeDiagnostic(diagnostics, "root_resolution_fallback_used") {
		t.Fatalf("missing fallback diagnostic: %+v", diagnostics)
	}
}

func TestResolveRegionRootsAmbiguousSuffixRoot(t *testing.T) {
	prog := loadProbeFixtureProgram(t)

	_, _, _, err := resolveRegionRoots(prog, []string{"(*Worker).Run"})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("err = %v, want ambiguous suffix error", err)
	}
}

func TestResolveRegionRootsMissingRoot(t *testing.T) {
	prog := loadProbeFixtureProgram(t)

	_, stats, diagnostics, err := resolveRegionRoots(prog, []string{"example.com/rootresolution.(*Missing).Root"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want missing root error", err)
	}
	if stats.MatchedSpecs != 0 {
		t.Fatalf("matched specs = %d, want 0", stats.MatchedSpecs)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none for missing root", diagnostics)
	}
}

func loadProbeFixtureProgram(t *testing.T) *ssa.Program {
	t.Helper()
	prog, _, err := loadSSA(filepath.Join("testdata", "root_resolution"), phaseLogger{})
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

func hasProbeDiagnostic(diagnostics []entrypath.Diagnostic, kind string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			return true
		}
	}
	return false
}
