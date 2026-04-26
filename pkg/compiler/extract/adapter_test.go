package extract

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestDeriveActorAdapterReadsClassificationPrimary(t *testing.T) {
	root := adapterTestRoot()
	shape := ShapeResult{Root: ShapeClassification{Shape: "http-handler"}}
	classification := &ArchetypeClassification{
		Primary: &ArchetypeChoice{
			Archetype: "serialized-actor",
			Emittable: true,
		},
		MatchedSymbols:  []reportv2.SymbolIdentity{root.Identity, adapterTestField()},
		CanonicalShapes: []string{"http-handler"},
	}

	adapters := deriveAdapters(root, shape, classification)
	got := findAdapter(adapters, "actor")
	if got == nil {
		t.Fatalf("actor adapter missing from %v", adapters)
	}
	if got.ID != "serialized-actor" || len(got.MatchedSymbols) != 2 {
		t.Fatalf("actor adapter=%#v", got)
	}

	classification.Primary.Archetype = "keyed-partitioned-state"
	adapters = deriveAdapters(root, shape, classification)
	if got := findAdapter(adapters, "actor"); got != nil {
		t.Fatalf("actor adapter emitted for non-actor primary: %#v", got)
	}
}

func TestDeriveActorAdapterRequiresSerializedActorPrimary(t *testing.T) {
	root := adapterTestRoot()
	shape := ShapeResult{Root: ShapeClassification{Shape: "http-handler"}}
	classification := &ArchetypeClassification{
		Primary: &ArchetypeChoice{
			Archetype: "keyed-partitioned-state",
			Emittable: false,
		},
		MatchedSymbols:  []reportv2.SymbolIdentity{root.Identity, adapterTestField()},
		CanonicalShapes: []string{"http-handler"},
	}

	adapters := deriveAdapters(root, shape, classification)
	if got := findAdapter(adapters, "actor"); got != nil {
		t.Fatalf("actor adapter emitted for keyed primary: %#v", got)
	}
}

func adapterTestRoot() reportv2.Root {
	return reportv2.Root{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  "example.com/app",
			PackagePath: "example.com/app",
			ObjectName:  "Handler",
			Kind:        "type",
		},
		Shape: "http-handler",
	}
}

func adapterTestField() reportv2.SymbolIdentity {
	return reportv2.SymbolIdentity{
		ModulePath:  "example.com/app",
		PackagePath: "example.com/app",
		ObjectName:  "Handler.connections",
		Kind:        "field",
	}
}

func findAdapter(adapters []reportv2.Adapter, kind string) *reportv2.Adapter {
	for i := range adapters {
		if adapters[i].Kind == kind {
			return &adapters[i]
		}
	}
	return nil
}
