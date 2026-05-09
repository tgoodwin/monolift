package activation

import (
	"context"
	"go/token"
	"time"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// Analyzer runs activation-path analysis for one package pattern and target.
type Analyzer struct {
	Config Config
}

// Config describes one activation-path analysis run.
type Config struct {
	Dir           string
	Packages      []string
	Target        string
	Format        string
	Verbose       bool
	Timeout       time.Duration
	Context       context.Context `json:"-"`
	BuildFlags    []string
	Env           []string
	Augment       AugmentMode
	ScopePackages bool
}

// Program is the loaded package/type/SSA state used by the analyzer.
type Program struct {
	Fset        *token.FileSet
	Packages    []*packages.Package
	SSAProgram  *ssa.Program
	SSAPackages []*ssa.Package
}

// Graph is a deterministic call graph projection used for path search.
type Graph struct {
	Nodes              []*Node         `json:"nodes"`
	Edges              []*Edge         `json:"edges"`
	Out                map[int][]*Edge `json:"-"`
	In                 map[int][]*Edge `json:"-"`
	AugmentIterations  int             `json:"-"`
	AugmentLimitHit    bool            `json:"-"`
	AugmentDiagnostics []Diagnostic    `json:"-"`
}

// Node identifies one SSA function in the activation graph.
type Node struct {
	ID       int           `json:"id"`
	Key      FunctionKey   `json:"key"`
	Name     string        `json:"name"`
	Package  string        `json:"package"`
	Position Position      `json:"position"`
	Func     *ssa.Function `json:"-"`
}

// Edge is one typed activation edge between graph nodes.
type Edge struct {
	ID          int      `json:"id"`
	From        int      `json:"from"`
	To          int      `json:"to"`
	Kind        EdgeKind `json:"kind"`
	Position    Position `json:"position"`
	Description string   `json:"description,omitempty"`
}

// Path is the shortest static path found by the analyzer.
type Path struct {
	Steps []PathStep `json:"steps"`
}

// PathStep is a node plus the incoming edge used to reach it.
type PathStep struct {
	Node *Node `json:"node"`
	Edge *Edge `json:"edge,omitempty"`
}

// Result is the stable analyzer result emitted by the CLI and evaluator.
type Result struct {
	Found       bool          `json:"found"`
	Category    MissCategory  `json:"category,omitempty"`
	Target      *Node         `json:"target,omitempty"`
	Entrypoints []*Node       `json:"entrypoints,omitempty"`
	Path        *Path         `json:"path,omitempty"`
	Cut         *CutResult    `json:"cut,omitempty"`
	PartialPath *PartialPath  `json:"partial_path,omitempty"`
	Diagnostics []Diagnostic  `json:"diagnostics,omitempty"`
	Timings     []PhaseTiming `json:"timings,omitempty"`
	Stats       GraphStats    `json:"stats"`
}

// Diagnostic describes a non-fatal analysis detail or a structured miss cause.
type Diagnostic struct {
	Severity string   `json:"severity"`
	Phase    string   `json:"phase"`
	Message  string   `json:"message"`
	Position Position `json:"position,omitempty"`
}

// PhaseTiming records wall time for a major analysis phase.
type PhaseTiming struct {
	Phase    string        `json:"phase"`
	Duration time.Duration `json:"duration"`
}

// GraphStats is a compact graph size summary.
type GraphStats struct {
	Nodes int `json:"nodes"`
	Edges int `json:"edges"`
}

// Position records a source location when available.
type Position struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// MissCategory classifies why no activation path was returned.
type MissCategory string

const (
	MissNone                MissCategory = ""
	MissTargetUnreachable   MissCategory = "target-unreachable"
	MissUnsupportedEdgeKind MissCategory = "unsupported-edge-kind"
	MissPackageLoadFailure  MissCategory = "package-load-failure"
	MissTimeout             MissCategory = "timeout"
	MissTargetNotFound      MissCategory = "target-not-found"
)
