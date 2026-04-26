package stateclass

import "github.com/tgoodwin/monolift/pkg/compiler/liftability"

// Subsume compares which properties an archetype demands. Construction has
// already checked the demanded verdicts for those properties.
type SubsumptionOutcome int

const (
	OutcomeEmpty SubsumptionOutcome = iota
	OutcomeSingle
	OutcomeSubsumed
	OutcomeIncomparable
)

func Subsume(set CandidateSet) (CandidateSet, SubsumptionOutcome) {
	switch len(set) {
	case 0:
		return nil, OutcomeEmpty
	case 1:
		return append(CandidateSet(nil), set...), OutcomeSingle
	}

	dropped := make([]bool, len(set))
	for i := range set {
		for j := range set {
			if i == j || dropped[j] {
				continue
			}
			if candidateStrictlySubsumes(set[i], set[j]) {
				dropped[j] = true
			}
		}
	}

	out := make(CandidateSet, 0, len(set))
	for i, candidate := range set {
		if !dropped[i] {
			out = append(out, candidate)
		}
	}
	if len(out) == 1 {
		return out, OutcomeSubsumed
	}
	return out, OutcomeIncomparable
}

func candidateStrictlySubsumes(a, b Candidate) bool {
	aKeys := requiredKeys(a.Archetype)
	bKeys := requiredKeys(b.Archetype)
	if len(aKeys) <= len(bKeys) {
		return false
	}
	for property := range bKeys {
		if !aKeys[property] {
			return false
		}
	}
	return true
}

func requiredKeys(id ArchetypeID) map[liftability.PropertyID]bool {
	archetype, ok := archetypeByID(id)
	if !ok {
		return nil
	}
	out := make(map[liftability.PropertyID]bool, len(archetype.Required))
	for property := range archetype.Required {
		out[property] = true
	}
	return out
}
