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
	switch mode {
	case "", ModeAll:
		index, err := AugmentStructField(graph, program)
		if err != nil {
			return err
		}
		if err := ApplyPredicates(graph, program, index, DefaultFrameworkPredicates()); err != nil {
			return err
		}
		return AugmentGoroutine(graph, program)
	case ModeRTAOnly:
		return nil
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
