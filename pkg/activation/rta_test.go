package activation

import (
	"path/filepath"
	"testing"
)

func TestRTAFixtureEdgeKinds(t *testing.T) {
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
	graph, err := BuildRTAGraph(program, entrypoints)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[EdgeKind]bool{}
	for _, edge := range graph.Edges {
		seen[edge.Kind] = true
	}
	for _, kind := range []EdgeKind{DirectCall, ConcreteMethodCall, InterfaceDispatch} {
		if !seen[kind] {
			t.Fatalf("RTA graph missing edge kind %s; saw %#v", kind, seen)
		}
	}
}

func TestShortestPathFixture(t *testing.T) {
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
	line := markerLine(t, filepath.Join(dir, "main.go"), "activation-target")
	target, err := cfg.ResolveTarget(program, "main.go", line)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildRTAGraph(program, entrypoints)
	if err != nil {
		t.Fatal(err)
	}
	path, found := ShortestPath(graph, entrypoints, target)
	if !found {
		t.Fatal("path not found")
	}
	if len(path.Steps) < 2 {
		t.Fatalf("path too short: %d", len(path.Steps))
	}
	if got := path.Steps[len(path.Steps)-1].Node.Key.FuncName; got != "target" {
		t.Fatalf("last step = %s, want target", got)
	}
}
