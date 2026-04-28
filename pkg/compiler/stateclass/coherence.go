package stateclass

import (
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

type refinementAxis string

const (
	axisOwnership refinementAxis = "ownership"
	axisRouting   refinementAxis = "routing"
	axisDelivery  refinementAxis = "delivery"
)

var archetypeAxes = map[ArchetypeID]refinementAxis{
	ArchetypeFanoutPublisher:       axisDelivery,
	ArchetypeKeyedPartitionedState: axisOwnership,
	ArchetypeSessionAffinityState:  axisRouting,
}

func connectionHubBufferCoherent(components []Candidate, evidence []liftability.Evidence) bool {
	if !componentsRefineDisjointAxes(components) {
		return false
	}
	return keyingDimensionAgrees(evidence)
}

func componentsRefineDisjointAxes(components []Candidate) bool {
	seen := map[refinementAxis]bool{}
	for _, component := range components {
		axis := archetypeAxes[component.Archetype]
		if axis == "" || seen[axis] {
			return false
		}
		seen[axis] = true
	}
	return len(seen) == len(components)
}

func keyingDimensionAgrees(evidence []liftability.Evidence) bool {
	keys := map[string]bool{}
	hasKeyEvidence := false
	for _, item := range evidence {
		if item.PropertyID != liftability.PropertyStateKeyedAccessInvariant || item.Verdict != liftability.VerdictHold {
			continue
		}
		hasKeyEvidence = true
		key := keyDimension(item)
		if key != "" {
			keys[key] = true
		}
	}
	if !hasKeyEvidence {
		return false
	}
	return len(keys) <= 1
}

func keyDimension(item liftability.Evidence) string {
	for _, part := range strings.FieldsFunc(item.Detail, func(r rune) bool {
		return r == ' ' || r == ';' || r == ','
	}) {
		if strings.HasPrefix(part, "key=") {
			return strings.TrimPrefix(part, "key=")
		}
	}
	return ""
}
