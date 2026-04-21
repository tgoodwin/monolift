package reportv2

import (
	"encoding/json"
	"testing"
)

func TestParseValidAcceptReport(t *testing.T) {
	data := mustReportJSON(t, sampleReport("accept", nil))

	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Pragma.Options["verdict"] != "accept" {
		t.Fatalf("verdict=%q want accept", got.Pragma.Options["verdict"])
	}
	if got.Root.Shape != "http-handler" {
		t.Fatalf("root.shape=%q want http-handler", got.Root.Shape)
	}
	if got.Root.DefaultTransport != "handler" {
		t.Fatalf("root.defaultTransport=%q want handler", got.Root.DefaultTransport)
	}
}

func TestParseValidRefuseReport(t *testing.T) {
	diagnostics := []Diagnostic{
		{
			Code:     "MLV2_EMBEDDED_DB_APP_ROOT",
			Severity: "error",
			Span:     sampleSpan(),
			RuleIDs:  []string{"EC-PRUNE-3"},
			Message:  "embedded DB app root refused",
		},
	}
	data := mustReportJSON(t, sampleReport("refuse-blocking", diagnostics))

	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(got.Diagnostics) != 1 || got.Diagnostics[0].Code != "MLV2_EMBEDDED_DB_APP_ROOT" {
		t.Fatalf("diagnostics=%v", got.Diagnostics)
	}
}

func TestParseInvalidMissingRequiredField(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(mustReportJSON(t, sampleReport("accept", nil)), &raw); err != nil {
		t.Fatal(err)
	}
	delete(raw, "schemaVersion")
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Parse(data); err == nil {
		t.Fatal("Parse succeeded for report missing schemaVersion")
	}
}

func TestParseAllowsMissingOptionalRootShapeFields(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(mustReportJSON(t, sampleReport("accept", nil)), &raw); err != nil {
		t.Fatal(err)
	}
	root, ok := raw["root"].(map[string]any)
	if !ok {
		t.Fatalf("root=%T want object", raw["root"])
	}
	delete(root, "shape")
	delete(root, "defaultTransport")
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Root.Shape != "" {
		t.Fatalf("root.shape=%q want empty", got.Root.Shape)
	}
	if got.Root.DefaultTransport != "" {
		t.Fatalf("root.defaultTransport=%q want empty", got.Root.DefaultTransport)
	}
}

func mustReportJSON(t *testing.T, report Report) []byte {
	t.Helper()

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func sampleReport(verdict string, diagnostics []Diagnostic) Report {
	root := sampleIdentity("github.com/example/app", "github.com/example/app/root", "Root", "type")
	return Report{
		SchemaVersion: SchemaVersion,
		BuildConfig: BuildConfig{
			GOOS:               "linux",
			GOARCH:             "amd64",
			CGOEnabled:         false,
			BuildTags:          []string{},
			ModuleRoot:         "/src",
			WorkspaceMode:      "single-module",
			Tests:              false,
			DependencyManifest: []Dependency{},
		},
		Analysis: Analysis{
			Algorithm:         "ssa-rta-stub",
			PrecisionTriggers: []string{},
			Deterministic:     true,
		},
		Pragma: Pragma{
			Name:    "root",
			Surface: "struct",
			Span:    sampleSpan(),
			Options: map[string]string{"verdict": verdict},
		},
		Root: Root{
			Identity:          root,
			RegistryKey:       nil,
			Shape:             "http-handler",
			DefaultTransport:  "handler",
			ExposedOperations: []SymbolIdentity{root},
		},
		Closure: Closure{
			IncludedSymbols: []SymbolEntry{{Identity: root, Span: sampleSpan(), RuleIDs: []string{"EC-REPORT-1"}}},
			ExcludedSymbols: []SymbolEntry{},
			WiringPaths:     []WiringPath{},
		},
		State: []StateItem{
			{
				Symbol:            root,
				Classes:           []string{"stateless"},
				Disposition:       "replicated",
				Evidence:          []string{"unit-test"},
				DeveloperDeclared: false,
			},
		},
		Adapters: []Adapter{
			{
				Kind:                 "handler",
				ID:                   "http-json",
				MatchedSymbols:       []SymbolIdentity{root},
				CanonicalShapes:      []string{"http-handler"},
				StateEffects:         []string{},
				TransportEffects:     []string{"http"},
				SerializationEffects: []string{"json"},
			},
		},
		ExternalDeps: []ExternalDep{},
		Pruning: Pruning{
			Bounded:  verdict == "accept",
			Frontier: []SymbolEntry{},
		},
		Diagnostics: diagnostics,
	}
}

func sampleIdentity(modulePath, packagePath, objectName, kind string) SymbolIdentity {
	return SymbolIdentity{
		ModulePath:  modulePath,
		PackagePath: packagePath,
		ObjectName:  objectName,
		Kind:        kind,
	}
}

func sampleSpan() SourceSpan {
	return SourceSpan{
		FileRelativePath: "root.go",
		ByteOffsetStart:  0,
		ByteOffsetEnd:    10,
		LineStart:        1,
		LineEnd:          1,
	}
}
