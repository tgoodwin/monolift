package activation

import "testing"

func TestFuncArgFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/funcarg/register")
	graph := newTestGraph(findFunctionByName(t, program, "main"))

	if err := AugmentFuncArgs(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "main", "myFunc", CallbackRegistration)
}
