package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type Report struct{}

type normativeReport struct {
	SchemaVersion       string                  `json:"schemaVersion"`
	AnalysisAlgorithm   string                  `json:"analysisAlgorithm"`
	Root                reportv2.SymbolIdentity `json:"root"`
	PragmaVerdict       string                  `json:"pragmaVerdict"`
	BoundedPruning      bool                    `json:"boundedPruning"`
	StateDispositions   []string                `json:"stateDispositions"`
	AdapterKinds        []string                `json:"adapterKinds"`
	ExternalAccessPaths []string                `json:"externalAccessPaths"`
	DiagnosticCodes     []string                `json:"diagnosticCodes"`
}

func (Report) CompareNormativeSubset(golden, got *reportv2.Report) error {
	goldenSubset := normativeSubset(golden)
	gotSubset := normativeSubset(got)
	if reflect.DeepEqual(goldenSubset, gotSubset) {
		return nil
	}
	goldenJSON, _ := json.MarshalIndent(goldenSubset, "", "  ")
	gotJSON, _ := json.MarshalIndent(gotSubset, "", "  ")
	return fmt.Errorf("closure report mismatch\nexpected normative subset:\n%s\ngot normative subset:\n%s", goldenJSON, gotJSON)
}

func LoadGolden(path string) (*reportv2.Report, error) {
	data, err := os.ReadFile(FromRepoRoot(path))
	if err != nil {
		return nil, err
	}
	return reportv2.Parse(data)
}

func WriteGolden(path string, report *reportv2.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	fullPath := FromRepoRoot(path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, data, 0o644)
}

func normativeSubset(report *reportv2.Report) normativeReport {
	subset := normativeReport{
		SchemaVersion:       report.SchemaVersion,
		AnalysisAlgorithm:   report.Analysis.Algorithm,
		Root:                report.Root.Identity,
		PragmaVerdict:       report.Pragma.Options["verdict"],
		BoundedPruning:      report.Pruning.Bounded,
		StateDispositions:   make([]string, 0, len(report.State)),
		AdapterKinds:        make([]string, 0, len(report.Adapters)),
		ExternalAccessPaths: make([]string, 0, len(report.ExternalDeps)),
		DiagnosticCodes:     make([]string, 0, len(report.Diagnostics)),
	}
	for _, state := range report.State {
		subset.StateDispositions = append(subset.StateDispositions, state.Symbol.ObjectName+"="+state.Disposition)
	}
	for _, adapter := range report.Adapters {
		subset.AdapterKinds = append(subset.AdapterKinds, adapter.Kind)
	}
	for _, dep := range report.ExternalDeps {
		subset.ExternalAccessPaths = append(subset.ExternalAccessPaths, dep.AccessPath)
	}
	for _, diagnostic := range report.Diagnostics {
		subset.DiagnosticCodes = append(subset.DiagnosticCodes, diagnostic.Code)
	}
	sort.Strings(subset.StateDispositions)
	sort.Strings(subset.AdapterKinds)
	sort.Strings(subset.ExternalAccessPaths)
	sort.Strings(subset.DiagnosticCodes)
	return subset
}
