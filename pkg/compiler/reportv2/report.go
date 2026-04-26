// Package reportv2 contains the shared Monolift v2 closure-report contract.
//
// This is the normative v1.0 schema per docs/specs/monolift-v2-contract.md
// §EC-REPORT. Additions are backwards-compatible; renames require
// schemaVersion bump.
package reportv2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const SchemaVersion = "1.0"

type Report struct {
	SchemaVersion string        `json:"schemaVersion"`
	BuildConfig   BuildConfig   `json:"buildConfig"`
	Analysis      Analysis      `json:"analysis"`
	Pragma        Pragma        `json:"pragma"`
	Root          Root          `json:"root"`
	Selection     *Selection    `json:"selection,omitempty"`
	Closure       Closure       `json:"closure"`
	State         []StateItem   `json:"state"`
	Adapters      []Adapter     `json:"adapters"`
	ExternalDeps  []ExternalDep `json:"externalDependencies"`
	Pruning       Pruning       `json:"pruning"`
	Diagnostics   []Diagnostic  `json:"diagnostics"`
}

type BuildConfig struct {
	GOOS               string       `json:"GOOS"`
	GOARCH             string       `json:"GOARCH"`
	CGOEnabled         bool         `json:"CGO_ENABLED"`
	BuildTags          []string     `json:"buildTags"`
	ModuleRoot         string       `json:"moduleRoot"`
	WorkspaceMode      string       `json:"workspaceMode"`
	Tests              bool         `json:"tests"`
	DependencyManifest []Dependency `json:"dependencyManifest"`
}

type Dependency struct {
	ModulePath string `json:"module_path"`
	Version    string `json:"version"`
	Sum        string `json:"sum"`
}

type Analysis struct {
	Algorithm         string   `json:"algorithm"`
	PrecisionTriggers []string `json:"precisionTriggers"`
	Deterministic     bool     `json:"deterministic"`
}

type Pragma struct {
	Name    string            `json:"name"`
	Surface string            `json:"surface"`
	Span    SourceSpan        `json:"span"`
	Options map[string]string `json:"options"`
}

type Root struct {
	Identity          SymbolIdentity     `json:"identity"`
	RegistryKey       *string            `json:"registryKey"`
	Admission         string             `json:"admission"`
	Properties        []PropertyEvidence `json:"properties"`
	Shape             string             `json:"shape"`
	DefaultTransport  string             `json:"defaultTransport"`
	ExposedOperations []SymbolIdentity   `json:"exposedOperations"`
	ArchetypeKind     string             `json:"archetype_kind,omitempty"`
	Primary           *ArchetypeChoice   `json:"primary,omitempty"`
	Alternatives      []ArchetypeChoice  `json:"alternatives,omitempty"`
	PragmaProvenance  *PragmaProvenance  `json:"pragma_provenance,omitempty"`
}

type PropertyEvidence struct {
	PropertyID string `json:"propertyId"`
	Subject    string `json:"subject"`
	Verdict    string `json:"verdict"`
	Source     string `json:"source"`
	Detail     string `json:"detail"`
}

type Selection struct {
	Admission *AdmissionRecord `json:"admission,omitempty"`
}

type AdmissionRecord struct {
	Admitted bool     `json:"admitted"`
	Reasons  []string `json:"reasons"`
}

type ArchetypeChoice struct {
	Archetype               string   `json:"archetype"`
	ContributingArchetypes  []string `json:"contributing_archetypes"`
	Alias                   string   `json:"alias,omitempty"`
	Verdict                 string   `json:"verdict,omitempty"`
	Emittable               bool     `json:"emittable"`
	RuntimeSelectable       bool     `json:"runtime_selectable"`
	DynamicDelegateEligible bool     `json:"dynamic_delegate_eligible"`
	RationaleTier           string   `json:"rationale_tier,omitempty"`
	Rationale               string   `json:"rationale,omitempty"`
}

type PragmaProvenance struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type Closure struct {
	IncludedSymbols []SymbolEntry `json:"includedSymbols"`
	ExcludedSymbols []SymbolEntry `json:"excludedSymbols"`
	WiringPaths     []WiringPath  `json:"wiringPaths"`
}

type StateItem struct {
	Symbol            SymbolIdentity `json:"symbol"`
	Classes           []string       `json:"classes"`
	Disposition       string         `json:"disposition"`
	Evidence          []string       `json:"evidence"`
	DeveloperDeclared bool           `json:"developerDeclared"`
}

type Adapter struct {
	Kind                 string           `json:"kind"`
	ID                   string           `json:"id"`
	MatchedSymbols       []SymbolIdentity `json:"matchedSymbols"`
	CanonicalShapes      []string         `json:"canonicalShapes"`
	StateEffects         []string         `json:"stateEffects"`
	TransportEffects     []string         `json:"transportEffects"`
	SerializationEffects []string         `json:"serializationEffects"`
}

type ExternalDep struct {
	Identity            SymbolIdentity `json:"identity"`
	AccessPath          string         `json:"accessPath"`
	ConfigurationSource string         `json:"configurationSource"`
	StateEffectSummary  []string       `json:"stateEffectSummary"`
}

type Pruning struct {
	Bounded  bool          `json:"bounded"`
	Frontier []SymbolEntry `json:"frontier"`
}

type Diagnostic struct {
	Code        string     `json:"code"`
	Severity    string     `json:"severity"`
	Span        SourceSpan `json:"span"`
	RuleIDs     []string   `json:"ruleIds"`
	Message     string     `json:"message"`
	Remediation *string    `json:"remediation"`
}

type SymbolIdentity struct {
	ModulePath    string    `json:"module_path"`
	PackagePath   string    `json:"package_path"`
	ObjectName    string    `json:"object_name"`
	Kind          string    `json:"kind"`
	Instantiation *[]string `json:"instantiation,omitempty"`
}

type SourceSpan struct {
	FileRelativePath string `json:"file_relative_path"`
	ByteOffsetStart  int    `json:"byte_offset_start"`
	ByteOffsetEnd    int    `json:"byte_offset_end"`
	LineStart        int    `json:"line_start"`
	LineEnd          int    `json:"line_end"`
}

type SymbolEntry struct {
	Identity SymbolIdentity `json:"identity"`
	Span     SourceSpan     `json:"span"`
	RuleIDs  []string       `json:"ruleIds"`
}

type WiringPath struct {
	Target SymbolIdentity `json:"target"`
	Steps  []SymbolEntry  `json:"steps"`
}

func Parse(data []byte) (*Report, error) {
	if err := Validate(data); err != nil {
		return nil, err
	}

	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}

func Validate(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("reportv2: invalid JSON: %w", err)
	}
	for _, field := range requiredReportFields {
		if _, ok := raw[field]; !ok {
			return fmt.Errorf("reportv2: missing required field %q", field)
		}
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var report Report
	if err := dec.Decode(&report); err != nil {
		return fmt.Errorf("reportv2: schema decode failed: %w", err)
	}
	if err := ensureEOF(dec); err != nil {
		return err
	}
	return validateReport(&report)
}

var requiredReportFields = []string{
	"schemaVersion",
	"buildConfig",
	"analysis",
	"pragma",
	"root",
	"closure",
	"state",
	"adapters",
	"externalDependencies",
	"pruning",
	"diagnostics",
}

func ensureEOF(dec *json.Decoder) error {
	if err := dec.Decode(&struct{}{}); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("reportv2: trailing JSON decode failed: %w", err)
	}
	return errors.New("reportv2: trailing JSON value")
}

func validateReport(report *Report) error {
	if report.SchemaVersion != SchemaVersion {
		return fmt.Errorf("reportv2: schemaVersion=%q want %q", report.SchemaVersion, SchemaVersion)
	}
	if report.Analysis.Algorithm == "" {
		return errors.New("reportv2: analysis.algorithm is required")
	}
	if !report.Analysis.Deterministic {
		return errors.New("reportv2: analysis.deterministic must be true")
	}
	if !oneOf(report.Pragma.Surface, "interface", "function", "method", "struct") {
		return fmt.Errorf("reportv2: unsupported pragma.surface %q", report.Pragma.Surface)
	}
	if err := validateIdentity("root.identity", report.Root.Identity); err != nil {
		return err
	}
	for i, property := range report.Root.Properties {
		if property.PropertyID == "" || property.Subject == "" || property.Verdict == "" || property.Source == "" {
			return fmt.Errorf("reportv2: root.properties[%d] requires propertyId, subject, verdict, and source", i)
		}
		if !oneOf(property.Verdict, "Hold", "Violate", "Unknown") {
			return fmt.Errorf("reportv2: unsupported root.properties[%d].verdict %q", i, property.Verdict)
		}
		if !oneOf(property.Source, "types", "ssa", "callgraph") {
			return fmt.Errorf("reportv2: unsupported root.properties[%d].source %q", i, property.Source)
		}
	}
	if report.Root.ArchetypeKind != "" && !oneOf(report.Root.ArchetypeKind, "single", "alternative_set", "composite") {
		return fmt.Errorf("reportv2: unsupported root.archetype_kind %q", report.Root.ArchetypeKind)
	}
	if report.Root.Primary != nil {
		if err := validateArchetypeChoice("root.primary", *report.Root.Primary); err != nil {
			return err
		}
	}
	for i, alternative := range report.Root.Alternatives {
		if err := validateArchetypeChoice(fmt.Sprintf("root.alternatives[%d]", i), alternative); err != nil {
			return err
		}
	}
	for i, state := range report.State {
		if err := validateIdentity(fmt.Sprintf("state[%d].symbol", i), state.Symbol); err != nil {
			return err
		}
		if len(state.Classes) == 0 {
			return fmt.Errorf("reportv2: state[%d].classes must not be empty", i)
		}
		if !oneOf(state.Disposition, "replicated", "singleton", "affinity-routed", "externalize-required", "refused") {
			return fmt.Errorf("reportv2: unsupported state[%d].disposition %q", i, state.Disposition)
		}
	}
	for i, adapter := range report.Adapters {
		if !oneOf(adapter.Kind, "handler", "registry", "serialization", "context-value", "cgo", "reflection", "generic-substitution", "actor") {
			return fmt.Errorf("reportv2: unsupported adapters[%d].kind %q", i, adapter.Kind)
		}
	}
	for i, dep := range report.ExternalDeps {
		if err := validateIdentity(fmt.Sprintf("externalDependencies[%d].identity", i), dep.Identity); err != nil {
			return err
		}
	}
	for i, diag := range report.Diagnostics {
		if diag.Code == "" {
			return fmt.Errorf("reportv2: diagnostics[%d].code is required", i)
		}
		if !oneOf(diag.Severity, "error", "warning") {
			return fmt.Errorf("reportv2: unsupported diagnostics[%d].severity %q", i, diag.Severity)
		}
	}
	return nil
}

func validateArchetypeChoice(path string, choice ArchetypeChoice) error {
	if choice.Archetype == "" {
		return fmt.Errorf("reportv2: %s.archetype is required", path)
	}
	if len(choice.ContributingArchetypes) == 0 {
		return fmt.Errorf("reportv2: %s.contributing_archetypes must not be empty", path)
	}
	if choice.Verdict != "" && !oneOf(choice.Verdict, "AUTO", "SUGGEST") {
		return fmt.Errorf("reportv2: unsupported %s.verdict %q", path, choice.Verdict)
	}
	if choice.RationaleTier != "" && !oneOf(choice.RationaleTier, "[PLOS-EL]", "[TOPOLOGY]", "[OPS-COST]", "[STABILITY]") {
		return fmt.Errorf("reportv2: unsupported %s.rationale_tier %q", path, choice.RationaleTier)
	}
	return nil
}

func validateIdentity(path string, identity SymbolIdentity) error {
	if identity.ModulePath == "" || identity.PackagePath == "" || identity.ObjectName == "" || identity.Kind == "" {
		return fmt.Errorf("reportv2: %s requires module_path, package_path, object_name, and kind", path)
	}
	if !oneOf(identity.Kind, "function", "method", "type", "interface", "field", "variable", "constant", "registry-entry", "package", "adapter") {
		return fmt.Errorf("reportv2: unsupported %s.kind %q", path, identity.Kind)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
