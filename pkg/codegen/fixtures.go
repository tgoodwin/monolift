package codegen

import (
	"path/filepath"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type MVPContract struct {
	Name           string
	Target         string
	ModuleRoot     string
	ModulePath     string
	PackagePath    string
	PackageName    string
	FunctionName   string
	Line           int
	RequestFields  []Param
	ExcludedParams []Param
	Results        []Result
	ReturnCodec    ReturnCodec
}

type Fixture struct {
	Contract MVPContract
	Report   reportv2.Report
	Cut      activation.CutResult
}

func SanitizeHTMLFixture(repoRoot string) Fixture {
	moduleRoot := filepath.Join(repoRoot, "evaluation", "miniflux")
	targetFile := filepath.Join(moduleRoot, "internal", "reader", "sanitizer", "sanitizer.go")
	contract := MVPContract{
		Name:         "SanitizeHTML",
		Target:       targetFile + ":217",
		ModuleRoot:   moduleRoot,
		ModulePath:   "miniflux.app/v2",
		PackagePath:  "miniflux.app/v2/internal/reader/sanitizer",
		PackageName:  "sanitizer",
		FunctionName: "SanitizeHTML",
		Line:         217,
		RequestFields: []Param{
			{Name: "baseURL", JSONName: "base_url", GoType: "string", Codec: CodecPrimitive, Index: 0, Classification: activation.Trivial},
			{Name: "rawHTML", JSONName: "input", GoType: "string", Codec: CodecPrimitive, Index: 1, Classification: activation.Trivial},
			{Name: "sanitizerOptions", JSONName: "sanitizer_options", GoType: "*SanitizerOptions", Codec: CodecJSON, Index: 2, Classification: activation.Serializable},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "string", Codec: CodecPrimitive, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecPrimitive, GoType: "string"},
	}
	return Fixture{
		Contract: contract,
		Report:   fixtureReport(contract),
		Cut:      fixtureCut(contract, activation.Serializable, activation.Stateless),
	}
}

func RefreshFeedFixture(repoRoot string) Fixture {
	moduleRoot := filepath.Join(repoRoot, "evaluation", "miniflux")
	targetFile := filepath.Join(moduleRoot, "internal", "reader", "handler", "handler.go")
	contract := MVPContract{
		Name:         "RefreshFeed",
		Target:       targetFile + ":207",
		ModuleRoot:   moduleRoot,
		ModulePath:   "miniflux.app/v2",
		PackagePath:  "miniflux.app/v2/internal/reader/handler",
		PackageName:  "handler",
		FunctionName: "RefreshFeed",
		Line:         207,
		RequestFields: []Param{
			{Name: "userID", JSONName: "user_id", GoType: "int64", Codec: CodecPrimitive, Index: 1, Classification: activation.Trivial},
			{Name: "feedID", JSONName: "feed_id", GoType: "int64", Codec: CodecPrimitive, Index: 2, Classification: activation.Trivial},
			{Name: "forceRefresh", JSONName: "force_refresh", GoType: "bool", Codec: CodecPrimitive, Index: 3, Classification: activation.Trivial},
		},
		ExcludedParams: []Param{
			{Name: "store", JSONName: "store", GoType: "*storage.Storage", Codec: CodecJSON, Index: 0, Classification: activation.Reconstructible},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "*locale.LocalizedErrorWrapper", Codec: CodecLocalizedErrorWrapper, Index: 0},
		},
		ReturnCodec: ReturnCodec{Kind: CodecLocalizedErrorWrapper, Nullable: true, GoType: "*locale.LocalizedErrorWrapper"},
	}
	return Fixture{
		Contract: contract,
		Report:   fixtureReport(contract),
		Cut:      fixtureCut(contract, activation.Reconstructible, activation.ClientReconstructible),
	}
}

func fixtureReport(contract MVPContract) reportv2.Report {
	return reportv2.Report{
		SchemaVersion: reportv2.SchemaVersion,
		BuildConfig: reportv2.BuildConfig{
			ModuleRoot: contract.ModuleRoot,
		},
		Pragma: reportv2.Pragma{
			Name: contract.Name,
			Span: reportv2.SourceSpan{
				FileRelativePath: filepath.ToSlash(stringsTrimModule(contract.ModuleRoot, contract.TargetFile())),
				LineStart:        contract.Line,
				LineEnd:          contract.Line,
			},
		},
		Root: reportv2.Root{
			Identity: reportv2.SymbolIdentity{
				ModulePath:  contract.ModulePath,
				PackagePath: contract.PackagePath,
				ObjectName:  contract.FunctionName,
				Kind:        "function",
			},
		},
		Closure: reportv2.Closure{
			IncludedSymbols: []reportv2.SymbolEntry{},
			ExcludedSymbols: []reportv2.SymbolEntry{},
			WiringPaths:     []reportv2.WiringPath{},
		},
		State:        []reportv2.StateItem{},
		Adapters:     []reportv2.Adapter{},
		ExternalDeps: []reportv2.ExternalDep{},
	}
}

func fixtureCut(contract MVPContract, boundary activation.BoundaryDataClass, state activation.StateClass) activation.CutResult {
	candidate := activation.CutCandidate{
		Step: 1,
		NodeKey: activation.FunctionKey{
			PackagePath: contract.PackagePath,
			FuncName:    contract.FunctionName,
		},
		NodeName:     contract.FunctionName,
		Feasibility:  activation.Feasible,
		BoundaryData: boundary,
		Callbacks:    activation.ZeroConfirmed,
		State:        state,
		Surface:      activation.Minimal,
		ErrorSem:     activation.ErrorOK,
		EdgeAlign:    activation.Strong,
		Reason:       "fixture recommended cut",
	}
	return activation.CutResult{
		Recommended: &candidate,
		Candidates:  []activation.CutCandidate{candidate},
	}
}

func (contract MVPContract) TargetFile() string {
	if contract.Target == "" {
		return ""
	}
	for i := len(contract.Target) - 1; i >= 0; i-- {
		if contract.Target[i] == ':' {
			return contract.Target[:i]
		}
	}
	return contract.Target
}

func stringsTrimModule(moduleRoot, file string) string {
	rel, err := filepath.Rel(moduleRoot, file)
	if err != nil {
		return file
	}
	return rel
}
