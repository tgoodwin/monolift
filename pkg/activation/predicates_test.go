package activation

import (
	"path/filepath"
	"testing"
)

func TestFrameworkPredicateFixture(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/predicates")
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
	predicate := FrameworkPredicate{
		ImportPath: "github.com/tgoodwin/monolift/pkg/activation/testdata/predicates",
		TypeName:   "Handler",
		FieldName:  "Run",
		DispatchFn: "(*Handler).execute",
	}
	if err := ApplyPredicates(graph, program, index, []FrameworkPredicate{predicate}); err != nil {
		t.Fatal(err)
	}
	assertEdge(t, graph, "execute", "target", StructFieldFuncValue)
}
