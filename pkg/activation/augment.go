package activation

import (
	"fmt"
	"strings"
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
func Augment(graph *Graph, program *Program, mode AugmentMode) error {
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
	for {
		snapshot := graph.FunctionSet()
		if err := runAugmentationPasses(graph, program, mode); err != nil {
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
		if err := ExploreCallees(graph, program, newFuncs); err != nil {
			return err
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

func runAugmentationPasses(graph *Graph, program *Program, mode AugmentMode) error {
	switch mode {
	case ModeAll:
		index, err := AugmentStructField(graph, program)
		if err != nil {
			return err
		}
		if err := ApplyPredicates(graph, program, index, DefaultFrameworkPredicates()); err != nil {
			return err
		}
		if err := AugmentGoroutine(graph, program); err != nil {
			return err
		}
		if err := AugmentPackageVars(graph, program); err != nil {
			return err
		}
		if err := AugmentFuncArgs(graph, program); err != nil {
			return err
		}
		if err := AugmentMapFuncValues(graph, program); err != nil {
			return err
		}
		return AugmentInterfaceFields(graph, program)
	case ModeStructField:
		_, err := AugmentStructField(graph, program)
		return err
	case ModePredicates:
		index, err := AugmentStructField(graph, program)
		if err != nil {
			return err
		}
		return ApplyPredicates(graph, program, index, DefaultFrameworkPredicates())
	case ModeGoroutine:
		return AugmentGoroutine(graph, program)
	default:
		return fmt.Errorf("unknown augmentation mode %q", mode)
	}
}
