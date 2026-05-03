package activation

import (
	"path/filepath"
	"testing"
)

func TestGoroutineFixtures(t *testing.T) {
	cases := []struct {
		name       string
		targetFunc string
		anonymous  bool
	}{
		{name: "direct", targetFunc: "target"},
		{name: "method", targetFunc: "Run"},
		{name: "closure", anonymous: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/goroutine", tc.name)
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
			if err := AugmentGoroutine(graph, program); err != nil {
				t.Fatal(err)
			}
			assertGoroutineEdge(t, graph, tc.targetFunc, tc.anonymous)
		})
	}
}

func assertGoroutineEdge(t *testing.T, graph *Graph, targetFunc string, anonymous bool) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.Kind != GoroutineLaunch {
			continue
		}
		from := graph.Nodes[edge.From]
		to := graph.Nodes[edge.To]
		if from.Key.FuncName != "main" {
			continue
		}
		targetOK := to.Key.FuncName == targetFunc
		if anonymous {
			targetOK = to.Func != nil && to.Func.Parent() != nil
		}
		if targetOK {
			if edge.Position.File == "" || edge.Position.Line == 0 {
				t.Fatalf("goroutine edge has empty position: %+v", edge)
			}
			return
		}
	}
	t.Fatalf("missing goroutine edge from main to %q", targetFunc)
}
