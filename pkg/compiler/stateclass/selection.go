package stateclass

import "github.com/tgoodwin/monolift/pkg/compiler/liftability"

type Classification struct {
	ArchetypeKind  string
	Primary        *Candidate
	Alternatives   []Candidate
	RationaleTier  RationaleTier
	RationaleProse string
}

func ClassifyRegion(props []liftability.Evidence) Classification {
	constructed := ConstructCandidates(props)
	extended := ExtendWithComposites(constructed, props)
	reduced, outcome := Subsume(extended)
	primary, alternatives, tier, prose := SelectPrimary(reduced)

	classification := Classification{
		ArchetypeKind:  archetypeKindForOutcome(outcome),
		Alternatives:   alternatives,
		RationaleTier:  tier,
		RationaleProse: prose,
	}
	if outcome != OutcomeEmpty {
		classification.Primary = &primary
		if isCompositeCandidate(primary) {
			classification.ArchetypeKind = "composite"
		}
	}
	return classification
}

func archetypeKindForOutcome(outcome SubsumptionOutcome) string {
	switch outcome {
	case OutcomeSingle, OutcomeSubsumed:
		return "single"
	case OutcomeIncomparable:
		return "alternative_set"
	default:
		return ""
	}
}

func Emittable(c Candidate) bool {
	if isCompositeCandidate(c) {
		return allContributing(c, Emittable)
	}
	return c.Archetype == ArchetypeSerializedActor
}

func RuntimeSelectable(c Candidate) bool {
	if isCompositeCandidate(c) {
		return allContributing(c, RuntimeSelectable)
	}
	return false
}

func DynamicDelegateEligible(c Candidate) bool {
	if isCompositeCandidate(c) {
		return allContributing(c, DynamicDelegateEligible)
	}
	return false
}

func allContributing(c Candidate, predicate func(Candidate) bool) bool {
	for _, id := range c.ContributingArchetypes {
		if !predicate(Candidate{Archetype: id, ContributingArchetypes: []ArchetypeID{id}}) {
			return false
		}
	}
	return len(c.ContributingArchetypes) > 0
}
