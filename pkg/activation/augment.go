package activation

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// AugmentMode controls which graph augmentation passes run after RTA.
type AugmentMode string

const (
	ModeRTAOnly     AugmentMode = "rta"
	ModeStructField AugmentMode = "structfield"
	ModePredicates  AugmentMode = "predicates"
	ModeGoroutine   AugmentMode = "goroutine"
	ModeAll         AugmentMode = "all"
)

const maxAugmentIterations = 10

// ParseAugmentMode validates a CLI augmentation mode string.
func ParseAugmentMode(raw string) (AugmentMode, error) {
	switch mode := AugmentMode(strings.TrimSpace(strings.ToLower(raw))); mode {
	case "", ModeRTAOnly, ModeStructField, ModePredicates, ModeGoroutine, ModeAll:
		if mode == "" {
			return ModeAll, nil
		}
		return mode, nil
	default:
		return "", fmt.Errorf("unknown augmentation mode %q", raw)
	}
}

// Augment runs the selected graph augmentation passes in deterministic order.
func Augment(graph *Graph, program *Program, mode AugmentMode, subTimings ...*[]PhaseTiming) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	if mode == "" {
		mode = ModeAll
	}
	graph.AugmentIterations = 0
	graph.AugmentLimitHit = false
	graph.AugmentDiagnostics = nil
	if mode == ModeRTAOnly {
		return nil
	}
	var timings *[]PhaseTiming
	if len(subTimings) > 0 {
		timings = subTimings[0]
	}
	state := &augmentState{}
	for {
		snapshot := graph.FunctionSet()
		if err := runAugmentationPasses(graph, program, mode, timings, state); err != nil {
			return err
		}
		newFuncs := graph.NewFunctionsSince(snapshot)
		if len(newFuncs) == 0 {
			recordAugmentInfo(graph, fmt.Sprintf("augmentation exploration converged after %d iteration(s)", graph.AugmentIterations))
			return nil
		}
		if graph.AugmentIterations >= maxAugmentIterations {
			graph.AugmentLimitHit = true
			graph.AugmentDiagnostics = append(graph.AugmentDiagnostics, Diagnostic{
				Severity: "warning",
				Phase:    "augment",
				Message:  fmt.Sprintf("augmentation exploration stopped after %d iterations with %d new root(s) remaining", maxAugmentIterations, len(newFuncs)),
			})
			return nil
		}
		graph.AugmentIterations++
		beforeExplore := graph.FunctionSet()
		rootsToExplore := unexploredRootsForState(state, newFuncs)
		_, err := timeSubPhase(timings, "ExploreCallees", func() (struct{}, error) {
			return struct{}{}, ExploreCallees(graph, program, rootsToExplore)
		})
		if err != nil {
			return err
		}
		if state.structFieldIndex != nil {
			UpdateStructFieldIndex(state.structFieldIndex, append(newFuncs, graph.NewFunctionsSince(beforeExplore)...))
		}
	}
}

func recordAugmentInfo(graph *Graph, message string) {
	if graph == nil {
		return
	}
	graph.AugmentDiagnostics = append(graph.AugmentDiagnostics, Diagnostic{
		Severity: "info",
		Phase:    "augment",
		Message:  message,
	})
}

type augmentState struct {
	structFieldIndex *StructFieldIndex
	funcArgCallsites *callbackCallsiteIndex
	exploredRoots    map[*ssa.Function]bool
}

func runAugmentationPasses(graph *Graph, program *Program, mode AugmentMode, timings *[]PhaseTiming, state *augmentState) error {
	switch mode {
	case ModeAll:
		index, err := timeSubPhase(timings, "AugmentStructField", func() (*StructFieldIndex, error) {
			return AugmentStructField(graph, program, structFieldIndexForState(state))
		})
		if err != nil {
			return err
		}
		setStructFieldIndexForState(state, index)
		_, err = timeSubPhase(timings, "ApplyPredicates", func() (struct{}, error) {
			return struct{}{}, ApplyPredicates(graph, program, index, DefaultFrameworkPredicates())
		})
		if err != nil {
			return err
		}
		_, err = timeSubPhase(timings, "AugmentGoroutine", func() (struct{}, error) {
			return struct{}{}, AugmentGoroutine(graph, program)
		})
		if err != nil {
			return err
		}
		_, err = timeSubPhase(timings, "AugmentPackageVars", func() (struct{}, error) {
			return struct{}{}, AugmentPackageVars(graph, program)
		})
		if err != nil {
			return err
		}
		funcArgCallsites, err := timeSubPhase(timings, "AugmentFuncArgs", func() (*callbackCallsiteIndex, error) {
			return AugmentFuncArgs(graph, program, funcArgCallsitesForState(state))
		})
		if err != nil {
			return err
		}
		setFuncArgCallsitesForState(state, funcArgCallsites)
		mapIndex, err := timeSubPhase(timings, "AugmentMapFuncValues", func() (*mapFuncIndex, error) {
			return AugmentMapFuncValues(graph, program)
		})
		if err != nil {
			return err
		}
		_, err = timeSubPhase(timings, "AugmentInterfaceFields", func() (struct{}, error) {
			return struct{}{}, AugmentInterfaceFields(graph, program, mapIndex)
		})
		return err
	case ModeStructField:
		_, err := timeSubPhase(timings, "AugmentStructField", func() (*StructFieldIndex, error) {
			index, err := AugmentStructField(graph, program, structFieldIndexForState(state))
			setStructFieldIndexForState(state, index)
			return index, err
		})
		return err
	case ModePredicates:
		index, err := timeSubPhase(timings, "AugmentStructField", func() (*StructFieldIndex, error) {
			return AugmentStructField(graph, program, structFieldIndexForState(state))
		})
		if err != nil {
			return err
		}
		setStructFieldIndexForState(state, index)
		_, err = timeSubPhase(timings, "ApplyPredicates", func() (struct{}, error) {
			return struct{}{}, ApplyPredicates(graph, program, index, DefaultFrameworkPredicates())
		})
		return err
	case ModeGoroutine:
		_, err := timeSubPhase(timings, "AugmentGoroutine", func() (struct{}, error) {
			return struct{}{}, AugmentGoroutine(graph, program)
		})
		return err
	default:
		return fmt.Errorf("unknown augmentation mode %q", mode)
	}
}

func structFieldIndexForState(state *augmentState) *StructFieldIndex {
	if state == nil {
		return nil
	}
	return state.structFieldIndex
}

func setStructFieldIndexForState(state *augmentState, index *StructFieldIndex) {
	if state != nil {
		state.structFieldIndex = index
	}
}

func funcArgCallsitesForState(state *augmentState) *callbackCallsiteIndex {
	if state == nil {
		return nil
	}
	return state.funcArgCallsites
}

func setFuncArgCallsitesForState(state *augmentState, index *callbackCallsiteIndex) {
	if state != nil {
		state.funcArgCallsites = index
	}
}

func unexploredRootsForState(state *augmentState, roots []*ssa.Function) []*ssa.Function {
	roots = sortedUniqueFunctions(roots)
	if state == nil {
		return roots
	}
	if state.exploredRoots == nil {
		state.exploredRoots = map[*ssa.Function]bool{}
	}
	for _, root := range roots {
		if root == nil {
			continue
		}
		state.exploredRoots[root] = true
	}
	return roots
}
