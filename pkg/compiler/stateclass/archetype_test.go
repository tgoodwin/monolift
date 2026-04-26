package stateclass

import (
	"reflect"
	"sort"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

func TestArchetypeCatalogStableIDs(t *testing.T) {
	if string(ArchetypeSerializedActor) != "serialized-actor" {
		t.Fatalf("serialized actor ID=%q", ArchetypeSerializedActor)
	}
	if string(ArchetypeKeyedPartitionedState) != "keyed-partitioned-state" {
		t.Fatalf("keyed partitioned state ID=%q", ArchetypeKeyedPartitionedState)
	}
}

func TestArchetypeCatalogRequiredProperties(t *testing.T) {
	tests := []struct {
		name string
		id   ArchetypeID
		want []liftability.PropertyID
	}{
		{
			name: "serialized actor",
			id:   ArchetypeSerializedActor,
			want: []liftability.PropertyID{
				liftability.PropertyEffectsNoGlobalWrites,
				liftability.PropertyEffectsNoParamEscape,
				liftability.PropertyEffectsNoParamHeapMutation,
				liftability.PropertyStateMutexEnclosesStoreInvariant,
				liftability.PropertyStateReceiverOwnedState,
			},
		},
		{
			name: "keyed partitioned state",
			id:   ArchetypeKeyedPartitionedState,
			want: []liftability.PropertyID{
				liftability.PropertyEffectsNoGlobalWrites,
				liftability.PropertyStateKeyedAccessInvariant,
				liftability.PropertyStateReceiverOwnedState,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			archetype, ok := archetypeByID(tc.id)
			if !ok {
				t.Fatalf("missing archetype %q", tc.id)
			}
			got := sortedRequiredKeys(archetype.Required)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("required keys=%v want %v", got, tc.want)
			}
			for property, verdict := range archetype.Required {
				if verdict != liftability.VerdictHold {
					t.Fatalf("%s verdict=%s want Hold", property, verdict)
				}
			}
		})
	}
}

func sortedRequiredKeys(required map[liftability.PropertyID]liftability.Verdict) []liftability.PropertyID {
	out := make([]liftability.PropertyID, 0, len(required))
	for property := range required {
		out = append(out, property)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}
