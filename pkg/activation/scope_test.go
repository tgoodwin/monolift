package activation

import (
	"path/filepath"
	"runtime"
	"testing"
)

func testdataDir(t *testing.T, name string) string {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata", "scope", name)
}

func TestReverseImportScope_Basic(t *testing.T) {
	dir := testdataDir(t, "basic")
	scoped, err := ReverseImportScope(dir, "target/target.go", nil)
	if err != nil {
		t.Fatalf("ReverseImportScope: %v", err)
	}

	want := map[string]bool{
		"example.com/basic/target":    true,
		"example.com/basic/importer1": true,
		"example.com/basic/importer2": true,
		"example.com/basic/cmd":       true,
	}
	excluded := "example.com/basic/unrelated"

	if len(scoped) != len(want) {
		t.Errorf("got %d packages, want %d: %v", len(scoped), len(want), scoped)
	}
	for _, pkg := range scoped {
		if !want[pkg] {
			t.Errorf("unexpected package in scope: %s", pkg)
		}
		if pkg == excluded {
			t.Errorf("unrelated package should not be in scope")
		}
	}
	for w := range want {
		found := false
		for _, pkg := range scoped {
			if pkg == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected package %s not in scope", w)
		}
	}
}

func TestReverseImportScope_Isolated(t *testing.T) {
	dir := testdataDir(t, "isolated")
	scoped, err := ReverseImportScope(dir, "target/target.go", nil)
	if err != nil {
		t.Fatalf("ReverseImportScope: %v", err)
	}

	if len(scoped) != 1 {
		t.Fatalf("got %d packages, want 1: %v", len(scoped), scoped)
	}
	if scoped[0] != "example.com/isolated/target" {
		t.Errorf("got %s, want example.com/isolated/target", scoped[0])
	}
}

func TestReverseImportScope_Diamond(t *testing.T) {
	dir := testdataDir(t, "diamond")
	scoped, err := ReverseImportScope(dir, "target/target.go", nil)
	if err != nil {
		t.Fatalf("ReverseImportScope: %v", err)
	}

	want := map[string]bool{
		"example.com/diamond/target": true,
		"example.com/diamond/mid1":   true,
		"example.com/diamond/mid2":   true,
		"example.com/diamond/top":    true,
		"example.com/diamond/cmd":    true,
	}

	if len(scoped) != len(want) {
		t.Errorf("got %d packages, want %d: %v", len(scoped), len(want), scoped)
	}
	for _, pkg := range scoped {
		if !want[pkg] {
			t.Errorf("unexpected package in scope: %s", pkg)
		}
	}
}
