package activation

import (
	"path/filepath"
	"testing"
)

func TestStructFieldFixtures(t *testing.T) {
	cases := []struct {
		name       string
		targetFunc string
		kind       EdgeKind
	}{
		{name: "direct", targetFunc: "myFunc", kind: StructFieldFuncValue},
		{name: "literal", targetFunc: "myFunc", kind: StructLiteralFieldAssignment},
		{name: "methodvalue", targetFunc: "Method", kind: StructFieldFuncValue},
		{name: "wrapper", targetFunc: "inner", kind: StructFieldFuncValue},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/structfield", tc.name)
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
			index, err := AugmentStructField(graph, program)
			if err != nil {
				t.Fatal(err)
			}
			if len(index.Stores) == 0 {
				t.Fatal("struct-field store index is empty")
			}
			assertEdge(t, graph, "dispatch", tc.targetFunc, tc.kind)
		})
	}
}

func assertEdge(t *testing.T, graph *Graph, fromFunc, toFunc string, kind EdgeKind) {
	t.Helper()
	for _, edge := range graph.Edges {
		from := graph.Nodes[edge.From]
		to := graph.Nodes[edge.To]
		if from.Key.FuncName == fromFunc && to.Key.FuncName == toFunc && edge.Kind == kind {
			return
		}
	}
	t.Fatalf("missing %s edge %s -> %s", kind, fromFunc, toFunc)
}
