package stateclass

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
)

func TestCandidateSelectionMatrix(t *testing.T) {
	withTemporaryArchetypes(t, map[ArchetypeID]Archetype{
		"test-base": {
			ID:   "test-base",
			Name: "test base",
			Required: map[liftability.PropertyID]liftability.Verdict{
				"test.base": liftability.VerdictHold,
			},
		},
		"test-superset": {
			ID:   "test-superset",
			Name: "test superset",
			Required: map[liftability.PropertyID]liftability.Verdict{
				"test.base":  liftability.VerdictHold,
				"test.extra": liftability.VerdictHold,
			},
		},
		"test-left": {
			ID:   "test-left",
			Name: "test left",
			Required: map[liftability.PropertyID]liftability.Verdict{
				"test.left": liftability.VerdictHold,
			},
		},
		"test-right": {
			ID:   "test-right",
			Name: "test right",
			Required: map[liftability.PropertyID]liftability.Verdict{
				"test.right": liftability.VerdictHold,
			},
		},
	})

	tests := []struct {
		fixture     string
		wantOutcome SubsumptionOutcome
		wantKind    string
		wantPrimary ArchetypeID
		wantTier    RationaleTier
		wantAltN    int
	}{
		{fixture: "single-archetype-hold", wantOutcome: OutcomeSingle, wantKind: "single", wantPrimary: ArchetypeKeyedPartitionedState, wantTier: TierPLOSEL},
		{fixture: "subsumed", wantOutcome: OutcomeSubsumed, wantKind: "single", wantPrimary: "test-superset", wantTier: TierPLOSEL},
		{fixture: "incomparable-tier-2", wantOutcome: OutcomeIncomparable, wantKind: "alternative_set", wantPrimary: ArchetypeSerializedActor, wantTier: TierTopology, wantAltN: 1},
		{fixture: "incomparable-tier-fallthrough", wantOutcome: OutcomeIncomparable, wantKind: "alternative_set", wantPrimary: "test-left", wantTier: TierStability, wantAltN: 1},
		{fixture: "empty", wantOutcome: OutcomeEmpty, wantKind: "", wantPrimary: "", wantTier: ""},
	}

	for _, tc := range tests {
		t.Run(tc.fixture, func(t *testing.T) {
			props := readCandidateFixture(t, tc.fixture)
			candidates := ConstructCandidates(props)
			extended := ExtendWithComposites(candidates, props)
			reduced, outcome := Subsume(extended)
			if outcome != tc.wantOutcome {
				t.Fatalf("outcome=%v want %v", outcome, tc.wantOutcome)
			}
			classification := ClassifyRegion(props)
			if classification.ArchetypeKind != tc.wantKind {
				t.Fatalf("kind=%q want %q", classification.ArchetypeKind, tc.wantKind)
			}
			if tc.wantPrimary == "" {
				if classification.Primary != nil {
					t.Fatalf("primary=%v want nil", classification.Primary)
				}
			} else if classification.Primary == nil || classification.Primary.Archetype != tc.wantPrimary {
				t.Fatalf("primary=%v want %s; reduced=%v", classification.Primary, tc.wantPrimary, reduced)
			}
			if classification.RationaleTier != tc.wantTier {
				t.Fatalf("tier=%q want %q", classification.RationaleTier, tc.wantTier)
			}
			if len(classification.Alternatives) != tc.wantAltN {
				t.Fatalf("alternatives=%d want %d", len(classification.Alternatives), tc.wantAltN)
			}
		})
	}
}

func TestExtendWithCompositesIsNoOp(t *testing.T) {
	for _, fixture := range []string{"single-archetype-hold", "subsumed", "incomparable-tier-2", "incomparable-tier-fallthrough", "empty"} {
		t.Run(fixture, func(t *testing.T) {
			props := readCandidateFixture(t, fixture)
			candidates := ConstructCandidates(props)
			extended := ExtendWithComposites(candidates, props)
			if !reflect.DeepEqual(extended, candidates) {
				t.Fatalf("extended=%v want identity %v", extended, candidates)
			}
		})
	}
}

func TestNoLegacyTerminology(t *testing.T) {
	for _, dir := range []string{".", filepath.Join("..", "liftability")} {
		entries, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("glob: %v", err)
		}
		for _, path := range entries {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			for _, term := range []string{"dom" + "inance", "dom" + "inate", "mono" + "tone"} {
				if strings.Contains(string(data), term) {
					t.Fatalf("%s contains legacy term %q", path, term)
				}
			}
		}
	}
}

func TestTierTableEnumeration(t *testing.T) {
	got := map[ArchetypeID]int{}
	for id, priority := range topologyTierPriority {
		got[id] = priority
	}
	want := map[ArchetypeID]int{
		ArchetypeSerializedActor:       100,
		ArchetypeKeyedPartitionedState: 50,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("topology tier table=%v want %v", got, want)
	}
}

func TestNoSingleCandidateLengthBranch(t *testing.T) {
	for _, path := range []string{"selection.go", "tiers.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		source := string(data)
		for _, forbidden := range []string{"len(candidates) == 1", "len(set) == 1"} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains forbidden single-candidate branch %q", path, forbidden)
			}
		}
	}
}

func readCandidateFixture(t *testing.T, name string) []liftability.Evidence {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "candidates", name, "properties.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var out []liftability.Evidence
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			t.Fatalf("invalid fixture line %q", line)
		}
		out = append(out, liftability.Evidence{
			PropertyID: liftability.PropertyID(parts[0]),
			Subject:    liftability.SubjectBody,
			Verdict:    liftability.Verdict(parts[1]),
			Source:     liftability.SourceSSA,
			Detail:     "fixture",
		})
	}
	return out
}

func withTemporaryArchetypes(t *testing.T, entries map[ArchetypeID]Archetype) {
	t.Helper()

	original := map[ArchetypeID]Archetype{}
	for id, archetype := range archetypes {
		original[id] = archetype
	}
	for id, archetype := range entries {
		archetypes[id] = archetype
	}
	t.Cleanup(func() {
		archetypes = original
	})
}

func TestArchetypesInOrderDeterministic(t *testing.T) {
	got := archetypesInOrder()
	ids := make([]string, 0, len(got))
	for _, archetype := range got {
		ids = append(ids, string(archetype.ID))
	}
	want := append([]string(nil), ids...)
	sort.Strings(want)
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("archetype order=%v want sorted %v", ids, want)
	}
}
