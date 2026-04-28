package stateclass

import (
	"sort"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

type Composite struct {
	Components     []ArchetypeID
	Alias          string
	CoherenceCheck func([]Candidate, []liftability.Evidence) bool
}

var registeredComposites = []Composite{
	{
		Components: sortedArchetypeIDs([]ArchetypeID{
			ArchetypeFanoutPublisher,
			ArchetypeKeyedPartitionedState,
			ArchetypeSessionAffinityState,
		}),
		Alias:          "connection-hub-buffer",
		CoherenceCheck: connectionHubBufferCoherent,
	},
}

func ExtendWithComposites(set CandidateSet, evidence []liftability.Evidence) CandidateSet {
	out := append(CandidateSet(nil), set...)
	for _, composite := range registeredComposites {
		components := matchingComponents(set, composite.Components)
		if len(components) != len(composite.Components) {
			continue
		}
		if composite.CoherenceCheck != nil && !composite.CoherenceCheck(components, evidence) {
			continue
		}
		contributing := sortedArchetypeIDs(composite.Components)
		satisfied := map[liftability.PropertyID]liftability.Verdict{}
		for _, component := range components {
			for property, verdict := range component.SatisfiedProperties {
				satisfied[property] = verdict
			}
		}
		out = append(out, Candidate{
			Archetype:              ArchetypeID(composite.Alias),
			ContributingArchetypes: contributing,
			Alias:                  composite.Alias,
			SatisfiedProperties:    satisfied,
		})
	}
	return out
}

func matchingComponents(set CandidateSet, want []ArchetypeID) []Candidate {
	wantSet := map[ArchetypeID]bool{}
	for _, id := range want {
		wantSet[id] = true
	}
	var out []Candidate
	for _, candidate := range set {
		if wantSet[candidate.Archetype] {
			out = append(out, candidate)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Archetype < out[j].Archetype
	})
	return out
}

func sortedArchetypeIDs(ids []ArchetypeID) []ArchetypeID {
	out := append([]ArchetypeID(nil), ids...)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func isCompositeCandidate(candidate Candidate) bool {
	return candidate.Alias != "" && len(candidate.ContributingArchetypes) > 1
}
