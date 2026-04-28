package stateclass

import (
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

type Candidate struct {
	Archetype              ArchetypeID
	ContributingArchetypes []ArchetypeID
	Alias                  string
	SatisfiedProperties    map[liftability.PropertyID]liftability.Verdict
}

type CandidateSet []Candidate

func ConstructCandidates(props []liftability.Evidence) CandidateSet {
	evidence := map[liftability.PropertyID]liftability.Verdict{}
	for _, prop := range props {
		if prop.Verdict == liftability.VerdictUnknown {
			continue
		}
		evidence[prop.PropertyID] = prop.Verdict
	}

	var out CandidateSet
	for _, archetype := range archetypesInOrder() {
		if !requiredPropertiesSatisfied(archetype.Required, evidence) {
			continue
		}
		if !archetypeEvidenceMatched(archetype.ID, props) {
			continue
		}
		satisfied := make(map[liftability.PropertyID]liftability.Verdict, len(archetype.Required))
		for property, verdict := range archetype.Required {
			satisfied[property] = verdict
		}
		out = append(out, Candidate{
			Archetype:              archetype.ID,
			ContributingArchetypes: []ArchetypeID{archetype.ID},
			SatisfiedProperties:    satisfied,
		})
	}
	return out
}

func archetypeEvidenceMatched(id ArchetypeID, props []liftability.Evidence) bool {
	switch id {
	case ArchetypeFanoutPublisher:
		return evidenceDetailContains(props, "fanout")
	case ArchetypeSessionAffinityState:
		return evidenceDetailContains(props, "session-affinity")
	default:
		return true
	}
}

func evidenceDetailContains(props []liftability.Evidence, needle string) bool {
	for _, prop := range props {
		if strings.Contains(prop.Detail, needle) {
			return true
		}
	}
	return false
}

func requiredPropertiesSatisfied(required map[liftability.PropertyID]liftability.Verdict, evidence map[liftability.PropertyID]liftability.Verdict) bool {
	for property, want := range required {
		if got, ok := evidence[property]; !ok || got != want {
			return false
		}
	}
	return true
}

func archetypesInOrder() []Archetype {
	ids := make([]string, 0, len(archetypes))
	for id := range archetypes {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]Archetype, 0, len(ids))
	for _, id := range ids {
		out = append(out, archetypes[ArchetypeID(id)])
	}
	return out
}
