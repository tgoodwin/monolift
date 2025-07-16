package profiling

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tgoodwin/monolift/pkg/profiling/internal/tree"

	pprofProfile "github.com/google/pprof/profile"
)

// Helper to create a minimal profile for testing
func createTestProfile(t *testing.T) *pprofProfile.Profile {
	t.Helper()
	// Minimal profile with one sample and one location
	prof := &pprofProfile.Profile{
		SampleType: []*pprofProfile.ValueType{
			{Type: "samples", Unit: "count"},
		},
		Sample: []*pprofProfile.Sample{
			{
				Value:    []int64{100},
				Location: []*pprofProfile.Location{{ID: 1}},
			},
		},
		Location: []*pprofProfile.Location{
			{
				ID: 1,
				Line: []pprofProfile.Line{
					{
						Function: &pprofProfile.Function{
							ID:   1,
							Name: "root",
						},
					},
				},
			},
		},
		Function: []*pprofProfile.Function{
			{ID: 1, Name: "root"},
		},
	}
	return prof
}

// Helper to create a simple flamegraph node tree
func createTestFlameGraph() *tree.FlameGraphNode {
	return &tree.FlameGraphNode{
		Name:  "root",
		Value: 100,
		Children: []*tree.FlameGraphNode{
			{
				Name:  "foo",
				Value: 60,
				Children: []*tree.FlameGraphNode{
					{Name: "bar", Value: 30},
				},
			},
			{Name: "baz", Value: 20},
		},
	}
}

func TestRemoveString(t *testing.T) {
	s := []string{"a", "b", "c"}
	got := remove_string(s, "b")
	want := []string{"a", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("remove_string() = %v, want %v", got, want)
	}
}

func TestRemoveChildrenCosts(t *testing.T) {
	node := &tree.FlameGraphNode{
		Value: 10,
		Children: []*tree.FlameGraphNode{
			{Value: 3},
			{Value: 2},
		},
	}
	got := removeChildrenCosts(node)
	want := int64(5)
	if got != want {
		t.Errorf("removeChildrenCosts() = %v, want %v", got, want)
	}
}

func TestBuildTree(t *testing.T) {
	prof := createTestProfile(t)
	_, err := BuildTree(prof)
	if err != nil {
		t.Errorf("BuildTree() error = %v", err)
	}
}

// TODO: Use testdata folder
func TestInspectPprofFiles(t *testing.T) {
	// Write a temp profile file
	tmpDir := t.TempDir()
	prof := createTestProfile(t)
	path := filepath.Join(tmpDir, "test.pb.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := prof.Write(f); err != nil {
		t.Fatal(err)
	}
	f.Close()

	inspector := &ProfileInspector{}
	inspector.InspectPprofFiles([]string{path})
	if len(inspector.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(inspector.Profiles))
	}
}

// TODO: Use testdata folder
func TestMergeProfiles(t *testing.T) {
	// Write two temp profile files
	tmpDir := t.TempDir()
	prof := createTestProfile(t)
	path1 := filepath.Join(tmpDir, "test1.pb.gz")
	path2 := filepath.Join(tmpDir, "test2.pb.gz")
	f1, _ := os.Create(path1)
	f2, _ := os.Create(path2)
	prof.Write(f1)
	prof.Write(f2)
	f1.Close()
	f2.Close()

	inspector := &ProfileInspector{}
	inspector.InspectPprofFiles([]string{path1, path2})
	merged := inspector.MergeProfiles([]string{path1, path2})
	if merged.Profile == nil {
		t.Errorf("expected merged profile, got nil")
	}
}

func TestGetProfileFunctionSubset(t *testing.T) {
	profile := &ProfileUnit{
		FlamegraphSourceRoot: createTestFlameGraph(),
	}
	functions := []string{"foo", "bar"}
	root := profile.GetProfileFunctionSubset(functions)
	if root == nil || root.Name != "root" {
		t.Errorf("expected root node, got %v", root)
	}
}

func TestGetProfileSubsetCountSortedList(t *testing.T) {
	node := &FunctionNode{
		Name:       "root",
		TotalValue: 100,
		SelfValue:  50,
		Children: []*FunctionNode{
			{Name: "foo", TotalValue: 60, SelfValue: 30},
			{Name: "bar", TotalValue: 20, SelfValue: 20},
		},
	}
	profile := &ProfileUnit{}
	list := profile.GetProfileSubsetCountSortedList(node, false)
	if len(list) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(list))
	}
	if list[0].TotalValue < list[1].TotalValue {
		t.Errorf("expected sorted by TotalValue")
	}
	list2 := profile.GetProfileSubsetCountSortedList(node, true)
	if list2[0].SelfValue < list2[1].SelfValue {
		t.Errorf("expected sorted by SelfValue")
	}
}

func TestProportionOfRootFromName(t *testing.T) {
	profile := &ProfileUnit{
		FlamegraphSourceRoot: createTestFlameGraph(),
	}
	got := profile.ProportionOfRootFromName("foo")
	want := float64(60) / float64(100)
	if got != want {
		t.Errorf("ProportionOfRootFromName() = %v, want %v", got, want)
	}
}

func TestFunctionCostWithoutChildren(t *testing.T) {
	profile := &ProfileUnit{
		FlamegraphSourceRoot: createTestFlameGraph(),
	}
	got := profile.FunctionCostWithoutChildren("foo")
	want := int64(30) // 60 - 30 (bar)
	if got != want {
		t.Errorf("FunctionCostWithoutChildren() = %v, want %v", got, want)
	}
}

func TestFindTopNFunction(t *testing.T) {
	profile := &ProfileUnit{
		FlamegraphSourceRoot: createTestFlameGraph(),
	}
	top := profile.FindTopNFunction(2, false)
	if len(top) != 2 {
		t.Errorf("expected 2 top functions, got %d", len(top))
	}
	top2 := profile.FindTopNFunction(2, true)
	if len(top2) != 2 {
		t.Errorf("expected 2 top functions, got %d", len(top2))
	}
}

func TestProfileInspector_GetProfileFunctionSubset(t *testing.T) {
	inspector := ProfileInspector{
		Profiles: map[string]ProfileUnit{
			"test": {
				FlamegraphSourceRoot: createTestFlameGraph(),
			},
		},
	}
	node := inspector.GetProfileFunctionSubset("test", []string{"foo"})
	if node == nil {
		t.Errorf("expected non-nil node")
	}
}
