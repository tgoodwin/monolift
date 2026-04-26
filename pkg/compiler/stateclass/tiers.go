package stateclass

type RationaleTier string

const (
	TierPLOSEL    RationaleTier = "[PLOS-EL]"
	TierTopology  RationaleTier = "[TOPOLOGY]"
	TierOpsCost   RationaleTier = "[OPS-COST]"
	TierStability RationaleTier = "[STABILITY]"
)

var topologyTierPriority = map[ArchetypeID]int{
	// ADR-0022 Decision 1: preserves the worked example's single-owner topology.
	ArchetypeSerializedActor: 100,
	// ADR-0022 Decision 1: sharding is useful but changes the native state topology.
	ArchetypeKeyedPartitionedState: 50,
}

func SelectPrimary(set CandidateSet) (primary Candidate, alternatives []Candidate, tier RationaleTier, prose string) {
	if len(set) == 0 {
		return Candidate{}, nil, "", ""
	}
	best := 0
	bestScore := topologyScore(set[0])
	tied := false
	for i := 1; i < len(set); i++ {
		score := topologyScore(set[i])
		switch {
		case score > bestScore:
			best = i
			bestScore = score
			tied = false
		case score == bestScore:
			tied = true
		}
	}
	primary = set[best]
	for i, candidate := range set {
		if i != best {
			alternatives = append(alternatives, candidate)
		}
	}
	if len(alternatives) == 0 {
		return primary, alternatives, TierPLOSEL, "single satisfied archetype selected"
	}
	if !tied {
		return primary, alternatives, TierTopology, "native state topology preserves one serialized owner"
	}
	return primary, alternatives, TierStability, "stable catalog order selected after earlier tiers tied"
}

func topologyScore(candidate Candidate) int {
	return topologyTierPriority[candidate.Archetype]
}
