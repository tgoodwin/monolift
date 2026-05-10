package activation

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/tools/go/ssa"
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

func TestUpdateStructFieldIndexMatchesFullRebuildAcrossThreeIterations(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/structfield/wrapper")

	full, err := AugmentStructField(nil, program)
	if err != nil {
		t.Fatal(err)
	}
	incremental := newStructFieldIndex()
	for _, chunk := range splitFunctionChunks(program.Functions(), 3) {
		UpdateStructFieldIndex(incremental, chunk)
	}

	if got, want := structFieldIndexSignature(incremental), structFieldIndexSignature(full); !reflect.DeepEqual(got, want) {
		t.Fatalf("incremental index mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func splitFunctionChunks(funcs []*ssa.Function, chunks int) [][]*ssa.Function {
	out := make([][]*ssa.Function, chunks)
	for i, fn := range funcs {
		out[i%chunks] = append(out[i%chunks], fn)
	}
	return out
}

func structFieldIndexSignature(index *StructFieldIndex) []string {
	var sig []string
	for _, key := range index.sortedKeys() {
		for _, store := range index.Stores[key] {
			sig = append(sig, "store:"+key.String()+":"+FunctionKeyForSSA(store.Func).String())
		}
		for _, read := range index.Reads[key] {
			sig = append(sig, "read:"+key.String()+":"+FunctionKeyForSSA(read.Caller).String())
		}
	}
	sort.Strings(sig)
	return sig
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
