package stateclass

import "github.com/tgoodwin/monolift/pkg/compiler/liftability"

// This catalog backs the ADR-0022 candidate machinery. The legacy Class*
// inference rules in stateclass.go remain in place for existing call sites.
type ArchetypeID string

const (
	ArchetypeSerializedActor       ArchetypeID = "serialized-actor"
	ArchetypeKeyedPartitionedState ArchetypeID = "keyed-partitioned-state"
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
}

func archetypeByID(id ArchetypeID) (Archetype, bool) {
	archetype, ok := archetypes[id]
	return archetype, ok
}
