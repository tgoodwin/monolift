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

type ShapeResult struct {
	Root         ShapeClassification
	PerOperation []ShapeClassification
	Diagnostics  []Diagnostic
}

type ShapeClassifier func(loaded *LoadedModule, program *ssa.Program, root reportv2.Root) (ShapeResult, error)
type ShapeValidator func(loaded *LoadedModule, root reportv2.Root, result ShapeResult) []Diagnostic
type StateResult struct {
	Items             []reportv2.StateItem
	Diagnostics       []Diagnostic
	PrecisionTriggers []string
}

type StateInferer func(loaded *LoadedModule, program *ssa.Program, reachable []*ssa.Function, root reportv2.Root, parsed *Pragma) (StateResult, error)

var (
	registeredShapeClassifier ShapeClassifier
	registeredShapeValidator  ShapeValidator
	registeredStateInferer    StateInferer
)

func RegisterShapeClassifier(classifier ShapeClassifier) {
	registeredShapeClassifier = classifier
}

func RegisterShapeValidator(validator ShapeValidator) {
	registeredShapeValidator = validator
}

func RegisterStateInferer(inferer StateInferer) {
	registeredStateInferer = inferer
}
