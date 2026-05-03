package activation

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveTargetFixture(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/simple")
	cfg := Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	line := markerLine(t, filepath.Join(dir, "main.go"), "activation-target")
	fn, err := cfg.ResolveTarget(program, "main.go", line)
	if err != nil {
		t.Fatal(err)
	}
	if got := fn.Name(); got != "target" {
		t.Fatalf("resolved function = %s, want target", got)
	}
}

func TestFindEntrypointsFixture(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/simple")
	cfg := Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	entrypoints, err := cfg.FindEntrypoints(program)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(entrypoints), 1; got != want {
		t.Fatalf("len(entrypoints) = %d, want %d", got, want)
	}
	if got := entrypoints[0].Name(); got != "main" {
		t.Fatalf("entrypoint = %s, want main", got)
	}
}

func markerLine(t *testing.T, path, marker string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, marker) {
			return i + 1
		}
	}
	t.Fatalf("marker %q not found in %s", marker, path)
	return 0
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}
