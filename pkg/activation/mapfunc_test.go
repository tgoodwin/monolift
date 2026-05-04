package activation

import "testing"

func TestMapFuncFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/direct")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if err := AugmentMapFuncValues(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "myFunc", MapFuncValue)
}

func TestMapFuncRegisterWrapperFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if err := AugmentMapFuncValues(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "newImpl", MapFuncValue)
}
