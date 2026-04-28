package reportv2

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const propertyTransportHandlerBoundary = "transport.handler-boundary"

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

func TestParseRoundTripsSymbolProvenance(t *testing.T) {
	report := sampleReport("accept", nil)
	report.Closure.IncludedSymbols[0].Provenance = []string{"Hub", "WebConn"}
	data := mustReportJSON(t, report)

	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	provenance := got.Closure.IncludedSymbols[0].Provenance
	if len(provenance) != 2 || provenance[0] != "Hub" || provenance[1] != "WebConn" {
		t.Fatalf("provenance=%v, want [Hub WebConn]", provenance)
	}
}

func TestParseRoundTripsSeams(t *testing.T) {
	report := sampleReport("accept", nil)
	report.Seams = []SeamEntry{{
		Type:     "ChannelField",
		Field:    "WebConn.send",
		ElemType: "github.com/mattermost/mattermost/server/public/model.WebSocketMessage",
		Writers:  []string{"Hub"},
		Readers:  []string{"WebConn"},
		Span:     sampleSpan(),
		Evidence: "ssa field access in (*Hub).Broadcast",
	}}

	data := mustReportJSON(t, report)
	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(got.Seams) != 1 {
		t.Fatalf("seams=%v, want one seam", got.Seams)
	}
	if got.Seams[0].Field != "WebConn.send" || got.Seams[0].Writers[0] != "Hub" || got.Seams[0].Readers[0] != "WebConn" {
		t.Fatalf("seam=%+v, want WebConn.send Hub->WebConn", got.Seams[0])
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

func TestParseAllowsMissingOptionalArchetypeFields(t *testing.T) {
	var raw map[string]any
	if err := json.Unmarshal(mustReportJSON(t, sampleReport("accept", nil)), &raw); err != nil {
		t.Fatal(err)
	}
	root, ok := raw["root"].(map[string]any)
	if !ok {
		t.Fatalf("root=%T want object", raw["root"])
	}
	delete(root, "archetype_kind")
	delete(root, "primary")
	delete(root, "alternatives")
	delete(root, "pragma_provenance")
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	got, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Root.ArchetypeKind != "" || got.Root.Primary != nil || got.Root.Alternatives != nil || got.Root.PragmaProvenance != nil {
		t.Fatalf("optional archetype fields unexpectedly populated: %#v", got.Root)
	}
}

func TestParseAllowsOptionalSelectionAdmission(t *testing.T) {
	report := sampleReport("accept", nil)
	report.Selection = &Selection{
		Admission: &AdmissionRecord{
			Admitted: true,
			Reasons:  []string{"admitted by transport admission v0"},
		},
	}

	got, err := Parse(mustReportJSON(t, report))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Selection == nil || got.Selection.Admission == nil || !got.Selection.Admission.Admitted {
		t.Fatalf("selection admission=%#v", got.Selection)
	}
}

func TestParseAlternativeSetWithActorAdapter(t *testing.T) {
	report := sampleReport("accept", nil)
	report.Root.ArchetypeKind = "alternative_set"
	report.Root.Primary = &ArchetypeChoice{
		Archetype:               "serialized-actor",
		ContributingArchetypes:  []string{"serialized-actor"},
		Emittable:               true,
		RuntimeSelectable:       false,
		DynamicDelegateEligible: false,
		RationaleTier:           "[TOPOLOGY]",
		Rationale:               "native state topology preserves one serialized owner",
	}
	report.Root.Alternatives = []ArchetypeChoice{{
		Archetype:               "keyed-partitioned-state",
		ContributingArchetypes:  []string{"keyed-partitioned-state"},
		Verdict:                 "SUGGEST",
		Emittable:               false,
		RuntimeSelectable:       false,
		DynamicDelegateEligible: false,
		RationaleTier:           "[TOPOLOGY]",
		Rationale:               "runtime selection is not hosted yet",
	}}
	report.Adapters = append(report.Adapters, Adapter{
		Kind:                 "actor",
		ID:                   "serialized-actor",
		MatchedSymbols:       []SymbolIdentity{report.Root.Identity},
		CanonicalShapes:      []string{"http-handler"},
		StateEffects:         []string{"serialized-owner", "mutex-serialized-state"},
		TransportEffects:     []string{"rpc-command-mailbox"},
		SerializationEffects: []string{"command-envelope"},
	})

	got, err := Parse(mustReportJSON(t, report))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if got.Root.ArchetypeKind != "alternative_set" || got.Root.Primary == nil || len(got.Root.Alternatives) != 1 {
		t.Fatalf("archetype fields=%#v", got.Root)
	}
	if got.Adapters[len(got.Adapters)-1].Kind != "actor" {
		t.Fatalf("last adapter=%#v", got.Adapters[len(got.Adapters)-1])
	}
}

func TestPreSprintGoldensValidateAgainstAdditiveSchema(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "..", "test", "e2e", "targets", "caddy", "golden", "report.json"),
		filepath.Join("..", "..", "..", "test", "e2e", "targets", "miniflux", "golden", "report.json"),
		filepath.Join("..", "..", "..", "test", "e2e", "targets", "pocketbase", "golden", "report.json"),
	} {
		t.Run(filepath.Base(filepath.Dir(filepath.Dir(path))), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if err := Validate(data); err != nil {
				t.Fatalf("Validate(%s): %v", path, err)
			}
		})
	}
}

func TestBootSpecRoundTrip(t *testing.T) {
	report := sampleReport("accept", nil)
	report.Boot = &BootSpec{
		ConfigSources:     []BootConfigSource{{Kind: "env", Name: "MM_SQLSETTINGS_DATASOURCE", Required: true}},
		DependencyInits:   []BootDependencyInit{{Name: "app.New", Classification: "required"}},
		GoroutineLaunches: []BootGoroutineLaunch{{Callee: "(*Hub).Start"}},
		Refusals:          []BootPathRefusal{},
		EntryPath:         []string{"main.main"},
	}
	data := mustReportJSON(t, report)
	if err := Validate(data); err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Boot == nil || len(parsed.Boot.ConfigSources) != 1 || parsed.Boot.ConfigSources[0].Name != "MM_SQLSETTINGS_DATASOURCE" {
		t.Fatalf("boot round-trip failed: %+v", parsed.Boot)
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
			Admission:         "liftable",
			Properties:        []PropertyEvidence{{PropertyID: propertyTransportHandlerBoundary, Subject: "body", Verdict: "Hold", Source: "types", Detail: "unit-test"}},
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
