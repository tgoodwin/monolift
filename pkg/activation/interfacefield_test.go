package activation

import (
	"reflect"
	"sort"
	"testing"
)

func TestInterfaceFieldFromMapFactoryFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	graph := newTestGraph(findFunctionByName(t, program, "use"))

	mapIndex := buildMapFuncIndex(program)
	if err := AugmentInterfaceFields(graph, program, mapIndex); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "use", "Work", InterfaceDispatch)
}

func TestInterfaceFieldSharedMapIndexMatchesRebuild(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	rebuildGraph := newTestGraph(findFunctionByName(t, program, "use"))
	if err := AugmentInterfaceFields(rebuildGraph, program, nil); err != nil {
		t.Fatal(err)
	}

	sharedGraph := newTestGraph(findFunctionByName(t, program, "use"))
	mapIndex := buildMapFuncIndex(program)
	if err := AugmentInterfaceFields(sharedGraph, program, mapIndex); err != nil {
		t.Fatal(err)
	}

	if got, want := graphEdgeSignature(sharedGraph), graphEdgeSignature(rebuildGraph); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared map index edges mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func graphEdgeSignature(graph *Graph) []string {
	var sig []string
	for _, edge := range graph.Edges {
		from := graph.Nodes[edge.From]
		to := graph.Nodes[edge.To]
		sig = append(sig, string(edge.Kind)+":"+from.Key.String()+"->"+to.Key.String())
	}
	sort.Strings(sig)
	return sig
}
