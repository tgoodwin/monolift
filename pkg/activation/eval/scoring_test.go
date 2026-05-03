package eval

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/activation"
)

func TestScoreTier2ExactAndFuzzy(t *testing.T) {
	expected := []activation.FunctionKey{
		{PackagePath: "p", FuncName: "main"},
		{PackagePath: "p", Receiver: "*T", FuncName: "Run"},
		{PackagePath: "p", FuncName: "target"},
	}
	actual := []activation.FunctionKey{
		{PackagePath: "p", FuncName: "main"},
		{PackagePath: "p", Receiver: "T", FuncName: "Run$bound"},
		{PackagePath: "p", FuncName: "target"},
	}
	exact, fuzzy := ScoreTier2(expected, actual)
	if exact != 0 {
		t.Fatalf("exact = %v, want 0", exact)
	}
	if fuzzy != 1 {
		t.Fatalf("fuzzy = %v, want 1", fuzzy)
	}
}

func TestScoreTier3FileLine(t *testing.T) {
	trace := Trace{
		Steps: []TraceStep{
			{To: "server/cmd/mattermost/main.go:19", Func: strPtr("main")},
			{To: "server/platform/services/docextractor/docextractor.go:21", Func: strPtr("Extract")},
		},
	}
	target := Target{
		Name:       "mattermost",
		ProjectDir: "/repo/evaluation/mattermost",
		WorkDir:    "/repo/evaluation/mattermost/server",
	}
	path := &activation.Path{Steps: []activation.PathStep{
		{Node: &activation.Node{Position: activation.Position{File: "/repo/evaluation/mattermost/server/cmd/mattermost/main.go", Line: 19}}},
		{Node: &activation.Node{Position: activation.Position{File: "/repo/evaluation/mattermost/server/platform/services/docextractor/docextractor.go", Line: 21}}},
	}}
	if got := ScoreTier3(trace, target, path); got != 1 {
		t.Fatalf("Tier3 = %v, want 1", got)
	}
}

func TestFirstUnsupportedEdge(t *testing.T) {
	trace := Trace{Steps: []TraceStep{
		{Step: 0, EdgeType: "entrypoint"},
		{Step: 1, EdgeType: "direct-function-call"},
		{Step: 2, EdgeType: "function-value-in-struct-field", To: "cmd/root.go:20", Func: strPtr("run")},
		{Step: 3, EdgeType: "channel-send-receive"},
	}}
	blocker := FirstUnsupportedEdge(trace)
	if blocker == nil {
		t.Fatal("blocker is nil")
	}
	if blocker.Step != 3 || blocker.Kind != activation.ChannelFlow {
		t.Fatalf("blocker = %+v", blocker)
	}
}

func TestClassifyMiss(t *testing.T) {
	trace := Trace{Steps: []TraceStep{
		{Step: 0, EdgeType: "entrypoint"},
		{Step: 1, EdgeType: "channel-send-receive"},
	}}
	category, blocker := ClassifyMiss(false, activation.MissTargetUnreachable, trace)
	if category != activation.MissUnsupportedEdgeKind {
		t.Fatalf("category = %s", category)
	}
	if blocker == nil || blocker.Kind != activation.ChannelFlow {
		t.Fatalf("blocker = %+v", blocker)
	}
	category, blocker = ClassifyMiss(false, activation.MissTimeout, trace)
	if category != activation.MissTimeout || blocker != nil {
		t.Fatalf("timeout classification = %s, %+v", category, blocker)
	}
}

func strPtr(s string) *string {
	return &s
}
