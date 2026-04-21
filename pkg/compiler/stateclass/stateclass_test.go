package stateclass

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
)

func TestInferExternalClientType(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("SQLRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 8))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassExternalizedDurable {
		t.Fatalf("items=%v want externalized-durable", result.Items)
	}
}

func TestInferSyncGuardedGlobal(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("SyncStore", extract.SurfaceFunction, nil, 15))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassSharedMutableAcross {
		t.Fatalf("items=%v want shared-mutable-across-callers", result.Items)
	}
}

func TestInferChannelLoop(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("Worker", extract.SurfaceStruct, map[string]string{"methods": "Start"}, 20))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassSingletonMutable {
		t.Fatalf("items=%v want singleton-mutable", result.Items)
	}
}

func TestInferMutationFreeCapturedConfig(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("ConfigRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 27))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassImmutableCapturedConfig {
		t.Fatalf("items=%v want immutable-captured-config", result.Items)
	}
}

func TestInferUnknownState(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("MutatingField", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 32))
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_STATE_UNKNOWN" {
		t.Fatalf("diagnostics=%v want MLV2_STATE_UNKNOWN", result.Diagnostics)
	}
}

func TestInferDeveloperDeclaredNarrowing(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("NoState", extract.SurfaceFunction, map[string]string{"state": "singleton"}, 35))
	if len(result.Items) != 1 || !result.Items[0].DeveloperDeclared {
		t.Fatalf("items=%v want developer-declared state", result.Items)
	}
}

func TestInferDeveloperDeclaredConflict(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("GlobalStore", extract.SurfaceFunction, map[string]string{"state": "stateless"}, 12))
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_STATE_DECL_CONFLICT" {
		t.Fatalf("diagnostics=%v want MLV2_STATE_DECL_CONFLICT", result.Diagnostics)
	}
}

func TestInferDeterministic(t *testing.T) {
	t.Parallel()

	req := fixturePragma("ConfigRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 27)
	first := inferFixture(t, req)
	second := inferFixture(t, req)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("infer result differs between runs\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestInferCompositeEmbeddedDBRule(t *testing.T) {
	t.Parallel()

	previous := embeddedDBAppRootMethodThreshold
	embeddedDBAppRootMethodThreshold = 2
	defer func() { embeddedDBAppRootMethodThreshold = previous }()

	result := inferFixture(t, fixturePragma("DBApp", extract.SurfaceStruct, nil, 37))
	gotCodes := []string{}
	for _, diagnostic := range result.Diagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	if !reflect.DeepEqual(gotCodes, []string{"MLV2_CLOSURE_TOO_LARGE", "MLV2_EMBEDDED_DB_APP_ROOT"}) && !reflect.DeepEqual(gotCodes, []string{"MLV2_EMBEDDED_DB_APP_ROOT", "MLV2_CLOSURE_TOO_LARGE"}) {
		t.Fatalf("diagnostics=%v want embedded-db composite codes", gotCodes)
	}
	if len(result.Items) != 1 || result.Items[0].Disposition != "refused" {
		t.Fatalf("items=%v want refused externalized-durable row", result.Items)
	}
}

func inferFixture(t *testing.T, req extract.Request) Result {
	t.Helper()

	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	analyzed, err := extract.Analyze(req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	program, reachable, err := extract.ReachableFunctions(loaded, analyzed.Report.Root)
	if err != nil {
		t.Fatalf("ReachableFunctions: %v", err)
	}
	result, err := Infer(loaded, program, reachable, analyzed.Report.Root, &loaded.RootPragma)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	return result
}

func fixturePragma(decl string, surface extract.Surface, options map[string]string, line int) extract.Request {
	rootFile := filepath.Join("testdata", "fixtures", "root.go")
	return extract.Request{
		Sources: []string{filepath.Join("testdata", "fixtures")},
		Pragmas: []extract.Pragma{{
			Name:     decl,
			Surface:  surface,
			DeclName: decl,
			DeclKind: string(surface),
			Options:  options,
			Span: extract.Span{
				Filename: rootFile,
				Line:     line,
				EndLine:  line,
			},
		}},
	}
}
