package stateclass

import (
	"reflect"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

func TestConnectionHubBufferCompositePositive(t *testing.T) {
	props := compositeEvidence("userID")
	classification := ClassifyRegion(props)
	if classification.Primary == nil {
		t.Fatal("primary is nil")
	}
	if classification.ArchetypeKind != "composite" {
		t.Fatalf("kind=%q want composite", classification.ArchetypeKind)
	}
	if classification.Primary.Alias != "connection-hub-buffer" {
		t.Fatalf("primary=%+v want connection-hub-buffer alias", classification.Primary)
	}
	want := []ArchetypeID{ArchetypeFanoutPublisher, ArchetypeKeyedPartitionedState, ArchetypeSessionAffinityState}
	if !reflect.DeepEqual(classification.Primary.ContributingArchetypes, want) {
		t.Fatalf("contributing=%v want %v", classification.Primary.ContributingArchetypes, want)
	}
	if len(classification.Alternatives) == 0 {
		t.Fatal("components were not retained as alternatives")
	}
	if DynamicDelegateEligible(*classification.Primary) {
		t.Fatal("connection-hub-buffer should not be dynamic-delegate eligible")
	}
}

func TestConnectionHubBufferCompositeNegativeTwoOfThree(t *testing.T) {
	props := []liftability.Evidence{
		evidence(liftability.PropertyEffectsNoGlobalWrites, "unit-test"),
		evidence(liftability.PropertyEffectsNoParamEscape, "unit-test"),
		evidence(liftability.PropertyStateReceiverOwnedState, "unit-test"),
		evidence(liftability.PropertyStateKeyedAccessInvariant, "key=userID"),
	}
	set := ExtendWithComposites(ConstructCandidates(props), props)
	for _, candidate := range set {
		if candidate.Alias == "connection-hub-buffer" {
			t.Fatalf("unexpected composite in %v", set)
		}
	}
}

func TestConnectionHubBufferCompositeNegativeMismatchedKeyDimension(t *testing.T) {
	props := compositeEvidence("userID")
	props = append(props, evidence(liftability.PropertyStateKeyedAccessInvariant, "key=connectionID"))
	set := ExtendWithComposites(ConstructCandidates(props), props)
	for _, candidate := range set {
		if candidate.Alias == "connection-hub-buffer" {
			t.Fatalf("unexpected composite with mismatched keys in %v", set)
		}
	}
}

func TestConnectionHubBufferCompositeNegativeSameAxisTwice(t *testing.T) {
	components := []Candidate{
		{Archetype: ArchetypeFanoutPublisher},
		{Archetype: ArchetypeFanoutPublisher},
		{Archetype: ArchetypeKeyedPartitionedState},
	}
	if componentsRefineDisjointAxes(components) {
		t.Fatal("same delivery axis claimed twice")
	}
}

func TestCaddyStyleActorKeyedDoesNotProduceConnectionHubBuffer(t *testing.T) {
	props := []liftability.Evidence{
		evidence(liftability.PropertyEffectsNoGlobalWrites, "unit-test"),
		evidence(liftability.PropertyEffectsNoParamEscape, "unit-test"),
		evidence(liftability.PropertyEffectsNoParamHeapMutation, "unit-test"),
		evidence(liftability.PropertyStateReceiverOwnedState, "unit-test"),
		evidence(liftability.PropertyStateMutexEnclosesStoreInvariant, "unit-test"),
		evidence(liftability.PropertyStateKeyedAccessInvariant, "key=host"),
	}
	set := ExtendWithComposites(ConstructCandidates(props), props)
	for _, candidate := range set {
		if candidate.Alias == "connection-hub-buffer" {
			t.Fatalf("unexpected composite in %v", set)
		}
	}
}

func compositeEvidence(key string) []liftability.Evidence {
	return []liftability.Evidence{
		evidence(liftability.PropertyEffectsNoGlobalWrites, "unit-test"),
		evidence(liftability.PropertyEffectsNoParamEscape, "fanout recipient iteration"),
		evidence(liftability.PropertyEffectsNoParamHeapMutation, "session-affinity replay state"),
		evidence(liftability.PropertyStateReceiverOwnedState, "unit-test"),
		evidence(liftability.PropertyStateKeyedAccessInvariant, "key="+key),
	}
}

func evidence(id liftability.PropertyID, detail string) liftability.Evidence {
	return liftability.Evidence{
		PropertyID: id,
		Subject:    liftability.SubjectBody,
		Verdict:    liftability.VerdictHold,
		Source:     liftability.SourceSSA,
		Detail:     detail,
	}
}
