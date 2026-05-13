package activation

import (
	"reflect"
	"testing"
)

func TestMapFuncFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/direct")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if _, err := AugmentMapFuncValues(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "myFunc", MapFuncValue)
}

func TestMapFuncRegisterWrapperFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if _, err := AugmentMapFuncValues(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "newImpl", MapFuncValue)
}

func TestMapFuncSharedIndexMatchesRebuild(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	rebuildGraph := newTestGraph(findFunctionByName(t, program, "dispatch"))
	if _, err := AugmentMapFuncValues(rebuildGraph, program); err != nil {
		t.Fatal(err)
	}

	sharedGraph := newTestGraph(findFunctionByName(t, program, "dispatch"))
	index := buildMapFuncIndex(program)
	if _, err := AugmentMapFuncValues(sharedGraph, program, index); err != nil {
		t.Fatal(err)
	}

	if got, want := graphEdgeSignature(sharedGraph), graphEdgeSignature(rebuildGraph); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared map index edges mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
