package extract

import (
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
)

type ShapeClassification struct {
	Operation        reportv2.SymbolIdentity
	Shape            string
	DefaultTransport string
	Evidence         []string
}

type LiftabilityClassification struct {
	Operation   reportv2.SymbolIdentity
	Admission   string
	Properties  []reportv2.PropertyEvidence
	RefusalCode string
}

type LiftabilityResult struct {
	Root         LiftabilityClassification
	PerOperation []LiftabilityClassification
	Diagnostics  []Diagnostic
}

type ShapeResult struct {
	Root         ShapeClassification
	PerOperation []ShapeClassification
	Diagnostics  []Diagnostic
}

type LiftabilityAnalyzer func(loaded *LoadedModule, program *ssa.Program, root reportv2.Root) (LiftabilityResult, error)
type ShapeClassifier func(loaded *LoadedModule, program *ssa.Program, root reportv2.Root, liftability LiftabilityResult) (ShapeResult, error)
type ShapeValidator func(loaded *LoadedModule, root reportv2.Root, liftability LiftabilityResult, result ShapeResult) []Diagnostic
type StateResult struct {
	Items             []reportv2.StateItem
	Diagnostics       []Diagnostic
	PrecisionTriggers []string
	Classification    *ArchetypeClassification
}

type StateInferer func(loaded *LoadedModule, program *ssa.Program, reachable []*ssa.Function, root reportv2.Root, parsed *Pragma) (StateResult, error)

type ArchetypeClassification struct {
	ArchetypeKind   string
	Primary         *ArchetypeChoice
	Alternatives    []ArchetypeChoice
	RationaleTier   string
	RationaleProse  string
	MatchedSymbols  []reportv2.SymbolIdentity
	CanonicalShapes []string
}

type ArchetypeChoice struct {
	Archetype               string
	ContributingArchetypes  []string
	Alias                   string
	Emittable               bool
	RuntimeSelectable       bool
	DynamicDelegateEligible bool
	RationaleTier           string
	Rationale               string
}

var (
	registeredLiftabilityAnalyzer LiftabilityAnalyzer
	registeredShapeClassifier     ShapeClassifier
	registeredShapeValidator      ShapeValidator
	registeredStateInferer        StateInferer
)

func RegisterLiftabilityAnalyzer(analyzer LiftabilityAnalyzer) {
	registeredLiftabilityAnalyzer = analyzer
}

func RegisterShapeClassifier(classifier ShapeClassifier) {
	registeredShapeClassifier = classifier
}

func RegisterShapeValidator(validator ShapeValidator) {
	registeredShapeValidator = validator
}

func RegisterStateInferer(inferer StateInferer) {
	registeredStateInferer = inferer
}
