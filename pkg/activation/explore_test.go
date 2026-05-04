package activation

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestExploreCalleesAddsDirectCallTree(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/explore/direct")
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
	b := findFunctionByName(t, program, "B")
	c := findFunctionByName(t, program, "C")
	if graph.nodeByFunction(c) != nil {
		t.Fatal("C is present before re-rooted exploration")
	}
	if err := ExploreCallees(graph, program, []*ssa.Function{b}); err != nil {
		t.Fatal(err)
	}
	if graph.nodeByFunction(c) == nil {
		t.Fatal("C is absent after re-rooted exploration")
	}
	assertEdge(t, graph, "B", "C", DirectCall)
}

func TestExploreCalleesAddsInterfaceDispatch(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/explore/interface")
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
	b := findFunctionByName(t, program, "B")
	work := findFunctionByName(t, program, "Work")
	if graph.nodeByFunction(work) != nil {
		t.Fatal("D.Work is present before re-rooted exploration")
	}
	if err := ExploreCallees(graph, program, []*ssa.Function{b}); err != nil {
		t.Fatal(err)
	}
	if graph.nodeByFunction(work) == nil {
		t.Fatal("D.Work is absent after re-rooted exploration")
	}
	assertEdge(t, graph, "B", "Work", InterfaceDispatch)
}

func findFunctionByName(t *testing.T, program *Program, name string) *ssa.Function {
	t.Helper()
	program.BuildSSA()
	for _, fn := range sortedFunctions(program.SSAProgram) {
		if fn.Name() == name {
			return fn
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}
