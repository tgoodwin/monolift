package stateclass

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestCandidateConstructionAgainstStateFixtures(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		seedName string
		want     []ArchetypeID
		wantKind string
		wantAltN int
	}{
		{
			name:     "mutex keyed map",
			dir:      "mutex-keyed-map",
			seedName: "Handler.connections",
			want:     []ArchetypeID{ArchetypeKeyedPartitionedState, ArchetypeSerializedActor},
			wantKind: "alternative_set",
			wantAltN: 1,
		},
		{
			name:     "mutex only store",
			dir:      "mutex-only-store",
			seedName: "Handler.value",
			want:     []ArchetypeID{ArchetypeSerializedActor},
			wantKind: "single",
		},
		{
			name:     "keyed no mutex",
			dir:      "keyed-no-mutex",
			seedName: "Handler.values",
			want:     []ArchetypeID{ArchetypeKeyedPartitionedState},
			wantKind: "single",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			props := fixtureRegionEvidence(t, tc.dir, tc.seedName)
			candidates := ConstructCandidates(props)
			got := candidateIDs(candidates)
			sort.Slice(tc.want, func(i, j int) bool { return tc.want[i] < tc.want[j] })
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("candidates=%v want %v props=%v", got, tc.want, props)
			}

			classification := ClassifyRegion(props)
			if classification.ArchetypeKind != tc.wantKind {
				t.Fatalf("kind=%q want %q", classification.ArchetypeKind, tc.wantKind)
			}
			if len(classification.Alternatives) != tc.wantAltN {
				t.Fatalf("alternatives=%d want %d", len(classification.Alternatives), tc.wantAltN)
			}
		})
	}
}

func fixtureRegionEvidence(t *testing.T, fixtureDir, seedName string) []liftability.Evidence {
	t.Helper()

	dir := filepath.Join("testdata", "fixtures", fixtureDir)
	req := extract.Request{
		Sources: []string{dir},
		Pragmas: []extract.Pragma{{
			Name:     fixtureDir,
			Surface:  extract.SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options:  map[string]string{"methods": "ServeHTTP"},
			Span: extract.Span{
				Filename: filepath.Join(dir, "root.go"),
				Line:     5,
				EndLine:  5,
			},
		}},
	}
	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	program, err := extract.BuildProgram(loaded)
	if err != nil {
		t.Fatalf("BuildProgram: %v", err)
	}
	root := extract.ResolveRoot(loaded)
	env := &sharedFixtureEnv{
		loaded:    loaded,
		program:   program,
		functions: ssautil.AllFunctions(program),
		callGraph: extract.CallGraphForProgram(program),
	}
	reachable := reachableFunctionsForRoot(loaded, env, root)
	seeds := harvestSeeds(loaded, root, reachable)
	for _, seed := range seeds {
		if seed.identity.ObjectName == seedName {
			return regionEvidence(baseCandidateProperties(), seed)
		}
	}
	t.Fatalf("seed %q not found in %#v", seedName, seeds)
	return nil
}

func baseCandidateProperties() []reportv2.PropertyEvidence {
	return []reportv2.PropertyEvidence{
		reportProperty(liftability.PropertyEffectsNoGlobalWrites),
		reportProperty(liftability.PropertyEffectsNoParamEscape),
		reportProperty(liftability.PropertyEffectsNoParamHeapMutation),
	}
}

func reportProperty(id liftability.PropertyID) reportv2.PropertyEvidence {
	return reportv2.PropertyEvidence{
		PropertyID: string(id),
		Subject:    liftability.SubjectBody,
		Verdict:    string(liftability.VerdictHold),
		Source:     string(liftability.SourceSSA),
		Detail:     "unit-test",
	}
}

func candidateIDs(candidates CandidateSet) []ArchetypeID {
	out := make([]ArchetypeID, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.Archetype)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
