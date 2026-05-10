package activation

import (
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestAugmentRTAOnlyBypassesAugmentation(t *testing.T) {
	graph, program := buildFixtureGraph(t, "pkg/activation/testdata/structfield/direct")
	nodes, edges := len(graph.Nodes), len(graph.Edges)

	if err := Augment(graph, program, ModeRTAOnly); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != nodes || len(graph.Edges) != edges {
		t.Fatalf("RTA-only changed graph size from %d/%d to %d/%d", nodes, edges, len(graph.Nodes), len(graph.Edges))
	}
	if graph.AugmentIterations != 0 {
		t.Fatalf("RTA-only iterations = %d, want 0", graph.AugmentIterations)
	}
	if len(graph.AugmentDiagnostics) != 0 {
		t.Fatalf("RTA-only diagnostics = %d, want 0", len(graph.AugmentDiagnostics))
	}
	assertNoEdge(t, graph, "dispatch", "myFunc", StructFieldFuncValue)
}

func TestAugmentModeStructFieldExploresNewRoots(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/modes/structfield")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))
	if err := Augment(graph, program, ModeStructField); err != nil {
		t.Fatal(err)
	}
	assertEdge(t, graph, "dispatch", "handler", StructFieldFuncValue)
	assertEdge(t, graph, "handler", "target", DirectCall)
	if graph.AugmentIterations == 0 {
		t.Fatal("ModeStructField did not explore newly added roots")
	}
}

func TestAugmentModePredicatesExploresNewRoots(t *testing.T) {
	graph, program := buildFixtureGraph(t, "pkg/activation/testdata/modes/cobra")
	if err := Augment(graph, program, ModePredicates); err != nil {
		t.Fatal(err)
	}
	assertEdge(t, graph, "execute", "run", StructFieldFuncValue)
	assertEdge(t, graph, "run", "target", DirectCall)
}

func TestAugmentModeAllExploresGoroutineRoots(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/modes/goroutine")
	graph := newTestGraph(findFunctionByName(t, program, "main"))
	if err := Augment(graph, program, ModeAll); err != nil {
		t.Fatal(err)
	}
	assertEdge(t, graph, "main", "worker", GoroutineLaunch)
	assertEdge(t, graph, "worker", "target", DirectCall)
	if graph.AugmentIterations == 0 {
		t.Fatal("ModeAll did not explore newly added goroutine roots")
	}
}

func TestAugmentIsIdempotent(t *testing.T) {
	graph, program := buildFixtureGraph(t, "pkg/activation/testdata/modes/cobra")
	if err := Augment(graph, program, ModeAll); err != nil {
		t.Fatal(err)
	}
	nodes, edges := len(graph.Nodes), len(graph.Edges)
	if err := Augment(graph, program, ModeAll); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != nodes || len(graph.Edges) != edges {
		t.Fatalf("second Augment changed graph size from %d/%d to %d/%d", nodes, edges, len(graph.Nodes), len(graph.Edges))
	}
}

func TestAugmentTwiceKeepsStableGraphAndNoDuplicateEdges(t *testing.T) {
	graph, program := buildFixtureGraph(t, "pkg/activation/testdata/mapfunc/direct")
	if err := Augment(graph, program, ModeAll); err != nil {
		t.Fatal(err)
	}
	nodes := len(graph.Nodes)
	edges := len(graph.Edges)
	signature := graphEdgeSignature(graph)

	if err := Augment(graph, program, ModeAll); err != nil {
		t.Fatal(err)
	}
	if len(graph.Nodes) != nodes || len(graph.Edges) != edges {
		t.Fatalf("graph size changed after second augment: nodes %d->%d edges %d->%d", nodes, len(graph.Nodes), edges, len(graph.Edges))
	}
	if got := graphEdgeSignature(graph); !reflect.DeepEqual(got, signature) {
		t.Fatalf("edge signature changed after second augment\n got: %#v\nwant: %#v", got, signature)
	}
	seen := map[string]bool{}
	for _, edge := range graphEdgeSignature(graph) {
		if seen[edge] {
			t.Fatalf("duplicate edge after second augment: %s", edge)
		}
		seen[edge] = true
	}
}

func buildFixtureGraph(t *testing.T, fixture string) (*Graph, *Program) {
	t.Helper()
	program := loadFixtureProgram(t, fixture)
	cfg := Config{Dir: filepath.Join(repoRoot(t), fixture), Packages: []string{"."}}
	entrypoints, err := cfg.FindEntrypoints(program)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := BuildRTAGraph(program, entrypoints)
	if err != nil {
		t.Fatal(err)
	}
	return graph, program
}

func loadFixtureProgram(t *testing.T, fixture string) *Program {
	t.Helper()
	dir := filepath.Join(repoRoot(t), fixture)
	cfg := Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func newTestGraph(fns ...*ssa.Function) *Graph {
	graph := &Graph{Out: map[int][]*Edge{}, In: map[int][]*Edge{}}
	for _, fn := range fns {
		graph.AddNode(FunctionKeyForSSA(fn), fn)
	}
	return graph
}

func assertNoEdge(t *testing.T, graph *Graph, fromFunc, toFunc string, kind EdgeKind) {
	t.Helper()
	for _, edge := range graph.Edges {
		from := graph.Nodes[edge.From]
		to := graph.Nodes[edge.To]
		if from.Key.FuncName == fromFunc && to.Key.FuncName == toFunc && edge.Kind == kind {
			t.Fatalf("unexpected %s edge %s -> %s", kind, fromFunc, toFunc)
		}
	}
}
