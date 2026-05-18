package activation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type syntheticStep struct {
	name string
	edge EdgeKind
}

func TestAnalyzeCutRejectsInfeasibleFunctionBoundaryData(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func bad(callback func()) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "bad", edge: DirectCall},
	})

	candidate := candidateByStep(t, cut.Candidates, 1)
	if candidate.BoundaryData != BoundaryInfeasible || candidate.Feasibility != Infeasible {
		t.Fatalf("candidate = %+v, want infeasible boundary data", candidate)
	}
	if cut.Recommended != nil {
		t.Fatalf("Recommended = %+v, want nil", cut.Recommended)
	}
	if len(cut.Diagnostics) == 0 || !strings.Contains(cut.Diagnostics[0].Message, "function value") {
		t.Fatalf("diagnostics = %+v, want function-value diagnostic", cut.Diagnostics)
	}
}

func TestAnalyzeCutRejectsResponseWriterAsInfeasible(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

import "net/http"

func entry() {}
func handler(w http.ResponseWriter) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "handler", edge: HTTPHandlerRegistration},
	})

	if cut.Recommended != nil {
		t.Fatalf("expected no recommendation (ResponseWriter means cut too shallow per ADR-0028), got %+v", cut.Recommended)
	}
	candidate := candidateByStep(t, cut.Candidates, 1)
	if candidate.Feasibility != Infeasible {
		t.Fatalf("feasibility = %s, want Infeasible (ADR-0028)", candidate.Feasibility)
	}
}

func TestAnalyzeCutPrefersZeroCallbackCandidate(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func callbackNode() { entry() }
func primitiveNode(id int) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "callbackNode", edge: DirectCall},
		{name: "primitiveNode", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want primitiveNode")
	}
	if cut.Recommended.NodeName != "primitiveNode" {
		t.Fatalf("recommended = %s, want primitiveNode", cut.Recommended.NodeName)
	}
	if got := candidateByStep(t, cut.Candidates, 1).Callbacks; got != Low {
		t.Fatalf("callbackNode callbacks = %s, want %s", got, Low)
	}
	if got := candidateByStep(t, cut.Candidates, 2).Callbacks; got != ZeroConfirmed {
		t.Fatalf("primitiveNode callbacks = %s, want %s", got, ZeroConfirmed)
	}
}

func TestAnalyzeCutRanksDeepClientReconstructibleOverShallowStateless(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

import "database/sql"

type Repo struct {
	db *sql.DB
}

func entry() {}
func stateless(id int) {}
func (r *Repo) withDB(id int) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "stateless", edge: DirectCall},
		{name: "withDB", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want withDB (deeper cut wins on surface)")
	}
	if cut.Recommended.NodeName != "withDB" {
		t.Fatalf("recommended = %s, want withDB (deeper cut has smaller surface)", cut.Recommended.NodeName)
	}
	if got := candidateByStep(t, cut.Candidates, 1).State; got != Stateless {
		t.Fatalf("stateless state = %s, want %s", got, Stateless)
	}
	if got := candidateByStep(t, cut.Candidates, 2).State; got != ClientReconstructible {
		t.Fatalf("withDB state = %s, want %s", got, ClientReconstructible)
	}
}

func TestAnalyzeCutPrefersDeeperCutOnTies(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func a() {}
func b() {}
func c() {}
func shallow() {}
func deep() {}
`, []syntheticStep{
		{name: "entry"},
		{name: "a", edge: DirectCall},
		{name: "b", edge: DirectCall},
		{name: "c", edge: DirectCall},
		{name: "shallow", edge: DirectCall},
		{name: "deep", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want deep")
	}
	if cut.Recommended.NodeName != "deep" {
		t.Fatalf("recommended = %s, want deep", cut.Recommended.NodeName)
	}
}

func TestAnalyzeCutRanksErrorSemantics(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func a() {}
func b() {}
func c() {}
func returnsError() error { return nil }
func returnsBool() bool { return true }
`, []syntheticStep{
		{name: "entry"},
		{name: "a", edge: DirectCall},
		{name: "b", edge: DirectCall},
		{name: "c", edge: DirectCall},
		{name: "returnsError", edge: DirectCall},
		{name: "returnsBool", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want returnsError")
	}
	if cut.Recommended.NodeName != "returnsError" {
		t.Fatalf("recommended = %s, want returnsError", cut.Recommended.NodeName)
	}
	if cut.Recommended.ErrorSem != ErrorOK {
		t.Fatalf("error semantics = %s, want %s", cut.Recommended.ErrorSem, ErrorOK)
	}
}

func TestAnalyzeCutRanksEdgeAlignment(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func a() {}
func b() {}
func c() {}
func interfaceAligned() {}
func directAligned() {}
`, []syntheticStep{
		{name: "entry"},
		{name: "a", edge: DirectCall},
		{name: "b", edge: DirectCall},
		{name: "c", edge: DirectCall},
		{name: "interfaceAligned", edge: InterfaceDispatch},
		{name: "directAligned", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want interfaceAligned")
	}
	if cut.Recommended.NodeName != "interfaceAligned" {
		t.Fatalf("recommended = %s, want interfaceAligned", cut.Recommended.NodeName)
	}
	if cut.Recommended.EdgeAlign != Strong {
		t.Fatalf("edge alignment = %s, want %s", cut.Recommended.EdgeAlign, Strong)
	}
}

func TestAnalyzeCutAllInfeasiblePath(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
func badOne(callback func()) {}
func badTwo(callback func() error) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "badOne", edge: DirectCall},
		{name: "badTwo", edge: DirectCall},
	})

	if cut.Recommended != nil {
		t.Fatalf("Recommended = %+v, want nil", cut.Recommended)
	}
	if got, want := len(cut.Diagnostics), 2; got < want {
		t.Fatalf("diagnostics = %+v, want at least %d hard-gate diagnostics", cut.Diagnostics, want)
	}
}

func TestAnalyzeCutSingleStepPathHasNoRecommendation(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

func entry() {}
`, []syntheticStep{
		{name: "entry"},
	})

	if cut.Recommended != nil {
		t.Fatalf("Recommended = %+v, want nil", cut.Recommended)
	}
	if len(cut.Candidates) != 0 {
		t.Fatalf("Candidates = %+v, want none", cut.Candidates)
	}
}

func TestAnalyzeCutContextIsSerializable(t *testing.T) {
	cut := analyzeSyntheticCut(t, `
package main

import "context"

func entry() {}
func withContext(ctx context.Context, name string) {}
`, []syntheticStep{
		{name: "entry"},
		{name: "withContext", edge: DirectCall},
	})

	if cut.Recommended == nil {
		t.Fatal("Recommended = nil, want withContext")
	}
	if cut.Recommended.BoundaryData != Serializable {
		t.Fatalf("boundary data = %s, want %s", cut.Recommended.BoundaryData, Serializable)
	}
	if cut.Recommended.Feasibility != Feasible {
		t.Fatalf("feasibility = %s, want %s", cut.Recommended.Feasibility, Feasible)
	}
}

func TestBoundaryWalkerRefinements(t *testing.T) {
	tests := []struct {
		name         string
		source       string
		function     string
		wantBoundary BoundaryDataClass
		wantState    StateClass
	}{
		{
			name: "named interface with func method",
			source: `
package main

type CallbackSink interface {
	Register(func())
}

func entry() {}
func target(s CallbackSink) {}
`,
			function:     "target",
			wantBoundary: BoundaryInfeasible,
		},
		{
			name: "named interface with writer method",
			source: `
package main

type Stream interface {
	Write([]byte) (int, error)
}

func entry() {}
func target(s Stream) {}
`,
			function:     "target",
			wantBoundary: BoundaryInfeasible, // streaming interface at cut point per ADR-0028
		},
		{
			name: "pointer receiver with sync field",
			source: `
package main

import "sync"

type Locked struct {
	mu sync.Mutex
}

func entry() {}
func (l *Locked) target() {}
`,
			function:     "target",
			wantBoundary: Trivial, // receiver excluded from boundary data; sync.Mutex classified under state
			wantState:    SharedState,
		},
		{
			name: "variadic alias resolves element",
			source: `
package main

type Name = string

func entry() {}
func target(names ...Name) {}
`,
			function:     "target",
			wantBoundary: Trivial,
		},
		{
			name: "map slice worst element",
			source: `
package main

type Payload struct {
	ID string
}

func entry() {}
func target(values map[string][]Payload) {}
`,
			function:     "target",
			wantBoundary: Serializable,
		},
		{
			name: "known state overrides",
			source: `
package main

import (
	"database/sql"
	"log"
)

type Store struct {
	db *sql.DB
	logger *log.Logger
}

func entry() {}
func (s *Store) target() {}
`,
			function:     "target",
			wantBoundary: Trivial, // receiver excluded from boundary data; classified under state
			wantState:    ClientReconstructible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cut := analyzeSyntheticCut(t, tt.source, []syntheticStep{
				{name: "entry"},
				{name: tt.function, edge: DirectCall},
			})
			candidate := candidateByStep(t, cut.Candidates, 1)
			if candidate.BoundaryData != tt.wantBoundary {
				t.Fatalf("BoundaryData = %s, want %s (candidate=%+v)", candidate.BoundaryData, tt.wantBoundary, candidate)
			}
			if tt.wantState != "" && candidate.State != tt.wantState {
				t.Fatalf("State = %s, want %s (candidate=%+v)", candidate.State, tt.wantState, candidate)
			}
		})
	}
}

// TestAdapterClassSyntheticBoundary verifies that the default AdapterClass
// label propagation from defaultAdapterClass correctly classifies synthetic
// boundary types. This is Phase 1 label propagation — the adapter recovery
// pass in Phase 3 may refine AdapterUnknown to AdapterPossible,
// LiveProxyRequired, or AdapterImpossible based on pattern matching.
func TestAdapterClassSyntheticBoundary(t *testing.T) {
	tests := []struct {
		name             string
		source           string
		function         string
		wantBoundary     BoundaryDataClass
		wantAdapterClass AdapterClass
	}{
		{
			name: "multipart.FileHeader param is serializable, adapter direct",
			source: `
package main

import "mime/multipart"

func entry() {}
func target(file *multipart.FileHeader) {}
`,
			function:         "target",
			wantBoundary:     Serializable,
			wantAdapterClass: DirectBoundary,
		},
		{
			name: "bytes.Reader param is serializable, adapter direct",
			source: `
package main

import "bytes"

func entry() {}
func target(r *bytes.Reader) {}
`,
			function:         "target",
			wantBoundary:     Serializable,
			wantAdapterClass: DirectBoundary,
		},
		{
			name: "io.Writer param is boundary-infeasible, adapter unknown",
			source: `
package main

import "io"

func entry() {}
func target(w io.Writer) {}
`,
			function:         "target",
			wantBoundary:     BoundaryInfeasible,
			wantAdapterClass: AdapterUnknown,
		},
		{
			name: "http.ResponseWriter param is boundary-infeasible, adapter unknown",
			source: `
package main

import "net/http"

func entry() {}
func target(w http.ResponseWriter) {}
`,
			function:         "target",
			wantBoundary:     BoundaryInfeasible,
			wantAdapterClass: AdapterUnknown,
		},
		{
			name: "channel param is boundary-infeasible, adapter unknown",
			source: `
package main

func entry() {}
func target(ch chan int) {}
`,
			function:         "target",
			wantBoundary:     BoundaryInfeasible,
			wantAdapterClass: AdapterUnknown,
		},
		{
			name: "os.File param is boundary-infeasible, adapter unknown",
			source: `
package main

import "os"

func entry() {}
func target(f *os.File) {}
`,
			function:         "target",
			wantBoundary:     BoundaryInfeasible,
			wantAdapterClass: AdapterUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cut := analyzeSyntheticCut(t, tt.source, []syntheticStep{
				{name: "entry"},
				{name: tt.function, edge: DirectCall},
			})
			candidate := candidateByStep(t, cut.Candidates, 1)
			if candidate.BoundaryData != tt.wantBoundary {
				t.Errorf("BoundaryData = %s, want %s", candidate.BoundaryData, tt.wantBoundary)
			}
			if candidate.AdapterClass != tt.wantAdapterClass {
				t.Errorf("AdapterClass = %s, want %s", candidate.AdapterClass, tt.wantAdapterClass)
			}
		})
	}
}

func analyzeSyntheticCut(t *testing.T, source string, steps []syntheticStep) *CutResult {
	t.Helper()
	program := loadSyntheticProgram(t, source)
	path := &Path{Steps: make([]PathStep, 0, len(steps))}
	for i, step := range steps {
		fn := findFunctionByName(t, program, step.name)
		pathStep := PathStep{Node: cutTestNode(i, fn)}
		if i > 0 {
			pathStep.Edge = &Edge{Kind: step.edge}
		}
		path.Steps = append(path.Steps, pathStep)
	}

	cut, err := AnalyzeCut(&Result{Path: path}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return cut
}

func loadSyntheticProgram(t *testing.T, source string) *Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module cutsynthetic\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(strings.TrimSpace(source)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	program.BuildSSA()
	return program
}
