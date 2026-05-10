package activation

import (
	"reflect"
	"testing"
)

func TestFuncArgFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/funcarg/register")
	graph := newTestGraph(findFunctionByName(t, program, "main"))

	if _, err := AugmentFuncArgs(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "main", "myFunc", CallbackRegistration)
}

func TestFuncArgSharedCallsiteIndexMatchesRebuild(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/funcarg/register")
	rebuildGraph := newTestGraph(findFunctionByName(t, program, "main"))
	if _, err := AugmentFuncArgs(rebuildGraph, program); err != nil {
		t.Fatal(err)
	}

	sharedGraph := newTestGraph(findFunctionByName(t, program, "main"))
	index := buildCallbackCallsiteIndex(program)
	if _, err := AugmentFuncArgs(sharedGraph, program, index); err != nil {
		t.Fatal(err)
	}

	if got, want := graphEdgeSignature(sharedGraph), graphEdgeSignature(rebuildGraph); !reflect.DeepEqual(got, want) {
		t.Fatalf("shared callsite index edges mismatch\n got: %#v\nwant: %#v", got, want)
	}
}
