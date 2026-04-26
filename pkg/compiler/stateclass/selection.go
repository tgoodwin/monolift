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
	}
	// SPRINT-0018: composite kind set here.
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
	return c.Archetype == ArchetypeSerializedActor
}

func RuntimeSelectable(Candidate) bool {
	return false
}

func DynamicDelegateEligible(Candidate) bool {
	return false
}
