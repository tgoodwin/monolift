package activation

import "testing"

func TestInterfaceFieldFromMapFactoryFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/mapfunc/wrapper")
	graph := newTestGraph(findFunctionByName(t, program, "use"))

	if err := AugmentInterfaceFields(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "use", "Work", InterfaceDispatch)
}
