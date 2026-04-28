package stateclass

import "github.com/tgoodwin/monolift/pkg/compiler/liftability"

// This catalog backs the ADR-0022 candidate machinery. The legacy Class*
// inference rules in stateclass.go remain in place for existing call sites.
type ArchetypeID string

const (
	ArchetypeSerializedActor       ArchetypeID = "serialized-actor"
	ArchetypeKeyedPartitionedState ArchetypeID = "keyed-partitioned-state"
	ArchetypeFanoutPublisher       ArchetypeID = "fanout-publisher"
	ArchetypeSessionAffinityState  ArchetypeID = "session-affinity-state"
)

type Archetype struct {
	ID       ArchetypeID
	Name     string
	Required map[liftability.PropertyID]liftability.Verdict
}

var archetypes = map[ArchetypeID]Archetype{
	ArchetypeSerializedActor: {
		ID:   ArchetypeSerializedActor,
		Name: "Serialized actor",
		Required: map[liftability.PropertyID]liftability.Verdict{
			liftability.PropertyEffectsNoParamHeapMutation:       liftability.VerdictHold,
			liftability.PropertyEffectsNoParamEscape:             liftability.VerdictHold,
			liftability.PropertyEffectsNoGlobalWrites:            liftability.VerdictHold,
			liftability.PropertyStateMutexEnclosesStoreInvariant: liftability.VerdictHold,
			liftability.PropertyStateReceiverOwnedState:          liftability.VerdictHold,
		},
	},
	ArchetypeKeyedPartitionedState: {
		ID:   ArchetypeKeyedPartitionedState,
		Name: "Keyed partitioned state",
		Required: map[liftability.PropertyID]liftability.Verdict{
			liftability.PropertyEffectsNoGlobalWrites:     liftability.VerdictHold,
			liftability.PropertyStateReceiverOwnedState:   liftability.VerdictHold,
			liftability.PropertyStateKeyedAccessInvariant: liftability.VerdictHold,
		},
	},
	ArchetypeFanoutPublisher: {
		ID:   ArchetypeFanoutPublisher,
		Name: "Fanout publisher",
		Required: map[liftability.PropertyID]liftability.Verdict{
			// Fanout evidence reuses ADR-0018 facts: root-owned state plus no
			// global writes means recipient iteration stays inside the region.
			liftability.PropertyEffectsNoGlobalWrites:   liftability.VerdictHold,
			liftability.PropertyEffectsNoParamEscape:    liftability.VerdictHold,
			liftability.PropertyStateReceiverOwnedState: liftability.VerdictHold,
		},
	},
	ArchetypeSessionAffinityState: {
		ID:   ArchetypeSessionAffinityState,
		Name: "Session affinity state",
		Required: map[liftability.PropertyID]liftability.Verdict{
			// Existing ADR-0018 properties capture the required shape: state is
			// receiver-owned, keyed, and not mutated through boundary params.
			liftability.PropertyEffectsNoGlobalWrites:      liftability.VerdictHold,
			liftability.PropertyEffectsNoParamHeapMutation: liftability.VerdictHold,
			liftability.PropertyStateReceiverOwnedState:    liftability.VerdictHold,
			liftability.PropertyStateKeyedAccessInvariant:  liftability.VerdictHold,
		},
	},
}

func archetypeByID(id ArchetypeID) (Archetype, bool) {
	archetype, ok := archetypes[id]
	return archetype, ok
}
