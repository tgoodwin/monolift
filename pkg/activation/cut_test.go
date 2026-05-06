package activation

import (
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestAnalyzeCutEnumeratesNonEntryCandidates(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/simple")
	mainFn := findFunctionByName(t, program, "main")
	topFn := findFunctionByName(t, program, "top")
	helperFn := findFunctionByName(t, program, "helper")

	result := &Result{Path: &Path{Steps: []PathStep{
		{Node: cutTestNode(0, mainFn), Edge: nil},
		{Node: cutTestNode(1, topFn), Edge: &Edge{Kind: DirectCall}},
		{Node: cutTestNode(2, helperFn), Edge: &Edge{Kind: InterfaceDispatch}},
	}}}

	cut, err := AnalyzeCut(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(cut.Candidates), 2; got != want {
		t.Fatalf("len(Candidates) = %d, want %d", got, want)
	}
	stepOne := candidateByStep(t, cut.Candidates, 1)
	stepTwo := candidateByStep(t, cut.Candidates, 2)
	if got, want := stepOne.Step, 1; got != want {
		t.Fatalf("first candidate step = %d, want %d", got, want)
	}
	if got, want := stepTwo.Step, 2; got != want {
		t.Fatalf("second candidate step = %d, want %d", got, want)
	}
	if got, want := stepOne.IncomingEdge, DirectCall; got != want {
		t.Fatalf("first candidate edge = %s, want %s", got, want)
	}
	if got, want := stepTwo.IncomingEdge, InterfaceDispatch; got != want {
		t.Fatalf("second candidate edge = %s, want %s", got, want)
	}
	if got, want := stepOne.NodeName, "top"; got != want {
		t.Fatalf("first candidate node = %s, want %s", got, want)
	}
}

func TestAnalyzeCutRejectsMalformedPaths(t *testing.T) {
	tests := []struct {
		name    string
		result  *Result
		message string
	}{
		{
			name:    "nil result",
			result:  nil,
			message: "non-nil Result",
		},
		{
			name:    "nil path",
			result:  &Result{},
			message: "Result.Path",
		},
		{
			name:    "empty path",
			result:  &Result{Path: &Path{}},
			message: "at least one path step",
		},
		{
			name:    "nil node",
			result:  &Result{Path: &Path{Steps: []PathStep{{Node: nil}}}},
			message: "step 0 is nil",
		},
		{
			name: "missing func",
			result: &Result{Path: &Path{Steps: []PathStep{{
				Node: &Node{Key: FunctionKey{PackagePath: "p", FuncName: "entry"}, Name: "entry"},
			}}}},
			message: "nil Func",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cut, err := AnalyzeCut(tt.result, nil)
			if err == nil {
				t.Fatal("AnalyzeCut error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("AnalyzeCut error = %q, want substring %q", err.Error(), tt.message)
			}
			if cut == nil || len(cut.Diagnostics) != 1 {
				t.Fatalf("diagnostics = %+v, want one diagnostic", cut)
			}
			if !strings.Contains(cut.Diagnostics[0].Message, tt.message) {
				t.Fatalf("diagnostic = %q, want substring %q", cut.Diagnostics[0].Message, tt.message)
			}
		})
	}
}

func cutTestNode(id int, fn *ssa.Function) *Node {
	return &Node{
		ID:   id,
		Key:  FunctionKeyForSSA(fn),
		Name: fn.Name(),
		Func: fn,
	}
}

func candidateByStep(t *testing.T, candidates []CutCandidate, step int) CutCandidate {
	t.Helper()
	for _, candidate := range candidates {
		if candidate.Step == step {
			return candidate
		}
	}
	t.Fatalf("candidate step %d not found in %+v", step, candidates)
	return CutCandidate{}
}

func TestCutRankingDecisionDimensions(t *testing.T) {
	base := func(name string) CutCandidate {
		return CutCandidate{
			Step:         1,
			NodeKey:      FunctionKey{PackagePath: "p", FuncName: name},
			NodeName:     name,
			IncomingEdge: DirectCall,
			Feasibility:  Feasible,
			BoundaryData: Serializable,
			Callbacks:    ZeroConfirmed,
			State:        Stateless,
			Surface:      Minimal,
			ErrorSem:     ErrorOK,
			EdgeAlign:    Strong,
		}
	}

	tests := []struct {
		name   string
		winner CutCandidate
		loser  CutCandidate
	}{
		{
			name: "surface beats better lower dimensions",
			winner: func() CutCandidate {
				c := base("deep")
				c.Callbacks = Low
				c.State = SharedState
				c.ErrorSem = NeedsWrapper
				c.EdgeAlign = Anti
				return c
			}(),
			loser: func() CutCandidate {
				c := base("shallow")
				c.Surface = Small
				return c
			}(),
		},
		{
			name: "callbacks win after surface tie",
			winner: func() CutCandidate {
				c := base("zero")
				c.State = SharedState
				c.ErrorSem = NeedsWrapper
				c.EdgeAlign = Anti
				return c
			}(),
			loser: func() CutCandidate {
				c := base("nonzero")
				c.Callbacks = Low
				return c
			}(),
		},
		{
			name: "state wins after surface and callbacks tie",
			winner: func() CutCandidate {
				c := base("stateless")
				c.ErrorSem = NeedsWrapper
				c.EdgeAlign = Anti
				return c
			}(),
			loser: func() CutCandidate {
				c := base("stateful")
				c.State = ConfigOnly
				return c
			}(),
		},
		{
			name: "error semantics wins after surface state callbacks tie",
			winner: func() CutCandidate {
				c := base("error")
				c.EdgeAlign = Anti
				return c
			}(),
			loser: func() CutCandidate {
				c := base("bool")
				c.ErrorSem = NeedsWrapper
				return c
			}(),
		},
		{
			name: "edge alignment wins after error semantics ties",
			winner: func() CutCandidate {
				c := base("interface")
				c.Step = 1
				c.EdgeAlign = Strong
				return c
			}(),
			loser: func() CutCandidate {
				c := base("direct")
				c.Step = 9
				c.EdgeAlign = Anti
				return c
			}(),
		},
		{
			name: "deeper step wins exact tie",
			winner: func() CutCandidate {
				c := base("deep")
				c.Step = 3
				return c
			}(),
			loser: base("shallow"),
		},
		{
			name: "node key wins final tie",
			winner: func() CutCandidate {
				c := base("alpha")
				c.Step = 2
				return c
			}(),
			loser: func() CutCandidate {
				c := base("beta")
				c.Step = 2
				return c
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !betterCutCandidate(tt.winner, tt.loser) {
				t.Fatalf("winner was not ranked above loser\nwinner=%+v\nloser=%+v", tt.winner, tt.loser)
			}
			if betterCutCandidate(tt.loser, tt.winner) {
				t.Fatalf("loser ranked above winner\nwinner=%+v\nloser=%+v", tt.winner, tt.loser)
			}
		})
	}
}
