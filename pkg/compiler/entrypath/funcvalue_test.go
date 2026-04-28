package entrypath

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/tools/go/ssa"
)

func TestFunctionRefIndexRecordsCallArgs(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "function_value_arg"))
	handler := mainPkg.Func("handler")
	if handler == nil {
		t.Fatal("handler function not found")
	}
	index := BuildFunctionRefIndex(prog)
	refs := index.Uses[handler]
	if len(refs) == 0 {
		t.Fatal("handler has no indexed references")
	}
	if !hasRefKind(refs, "call_arg") {
		t.Fatalf("handler refs missing call_arg: %+v", refs)
	}
}

func TestFunctionRefIndexMaxFunctionsProducesDeterministicPartialStats(t *testing.T) {
	prog, _ := loadFixtureProgram(t, filepath.Join("testdata", "function_value_arg"))
	totalFunctions := len(sortedProgramFunctions(prog))
	const maxFunctions = 3
	if totalFunctions <= maxFunctions {
		t.Fatalf("fixture has %d functions, need more than %d for partial sampling", totalFunctions, maxFunctions)
	}

	first := BuildFunctionRefIndexWithOptions(prog, FunctionRefIndexOptions{MaxFunctions: maxFunctions})
	second := BuildFunctionRefIndexWithOptions(prog, FunctionRefIndexOptions{MaxFunctions: maxFunctions})

	if first.Stats.ScannedFunctions != maxFunctions {
		t.Fatalf("scanned functions = %d, want %d", first.Stats.ScannedFunctions, maxFunctions)
	}
	if first.Stats.SkippedFunctions != totalFunctions-maxFunctions {
		t.Fatalf("skipped functions = %d, want %d", first.Stats.SkippedFunctions, totalFunctions-maxFunctions)
	}
	if !reflect.DeepEqual(stableFunctionRefStats(first.Stats), stableFunctionRefStats(second.Stats)) {
		t.Fatalf("max-function stats are not deterministic:\nfirst:  %+v\nsecond: %+v", first.Stats, second.Stats)
	}
}

func TestFunctionRefIndexBudgetProducesPartialStatsAndDiagnostic(t *testing.T) {
	prog, _ := loadFixtureProgram(t, filepath.Join("testdata", "function_value_arg"))
	totalFunctions := len(sortedProgramFunctions(prog))
	if totalFunctions == 0 {
		t.Fatal("fixture has no functions")
	}

	index := BuildFunctionRefIndexWithOptions(prog, FunctionRefIndexOptions{Budget: time.Nanosecond})

	if !hasDiagnostic(index.Diagnostics, "function_ref_index_budget_exceeded") {
		t.Fatalf("missing budget diagnostic: %+v", index.Diagnostics)
	}
	if index.Stats.ScannedFunctions != 0 {
		t.Fatalf("scanned functions = %d, want 0 for immediately exhausted budget", index.Stats.ScannedFunctions)
	}
	if index.Stats.SkippedFunctions != totalFunctions {
		t.Fatalf("skipped functions = %d, want %d", index.Stats.SkippedFunctions, totalFunctions)
	}
}

func TestFunctionRefIndexDefaultOrderingRemainsSorted(t *testing.T) {
	prog, _ := loadFixtureProgram(t, filepath.Join("testdata", "function_value_arg"))
	functions := sortedProgramFunctions(prog)
	if len(functions) < 3 {
		t.Fatalf("fixture has %d functions, need at least 3", len(functions))
	}

	allIndex := BuildFunctionRefIndexWithOptions(prog, FunctionRefIndexOptions{MaxFunctions: 1})
	if !allIndex.ScannedOwners[functions[0]] {
		t.Fatalf("all-function index scanned different first owner, want %s", functionString(functions[0]))
	}

	seeds := NewFunctionIndexSeedSet()
	seeds.Add(functions[2], FunctionIndexSeedReasonReversePath)
	seeds.Add(functions[0], FunctionIndexSeedReasonBoundary)
	seeds.Add(functions[1], FunctionIndexSeedReasonOnDemandExpansion)
	seedOwners := seeds.Owners()
	seedIndex := BuildFunctionRefIndexForSeeds(seeds, FunctionRefIndexOptions{MaxFunctions: 1})
	if !seedIndex.ScannedOwners[seedOwners[0]] {
		t.Fatalf("seeded index scanned different first owner, want %s", functionString(seedOwners[0]))
	}
}

func TestReversePathSeededIndexFindsReverseReachableHandler(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "reverse_path_seed"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	rootOnly := NewFunctionIndexSeedSet()
	rootOnly.Add(root, FunctionIndexSeedReasonReversePath)
	rootOnlyFlow := analyzeFunctionValueFlow(prog, BuildFunctionRefIndexForSeeds(rootOnly, FunctionRefIndexOptions{}), []*ssa.Function{root})
	if hasExternalSurface(rootOnlyFlow.ExternalSurfaces, "handler") {
		t.Fatalf("roots-only seeded index unexpectedly found handler: %+v", rootOnlyFlow.ExternalSurfaces)
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode: FunctionIndexModeReversePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasExternalSurface(result.ExternalSurfaces, "handler") {
		t.Fatalf("reverse-path mode missed handler: %+v", result.ExternalSurfaces)
	}
}

func TestHTTPSinkSeedSetIncludesOnlyHTTPShapedOwner(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "http_sink_seed"))
	seeds := httpSinkSeedSet(prog)

	if !seedSetContains(seeds, mainPkg.Func("httpOwner")) {
		t.Fatalf("httpOwner was not seeded")
	}
	if seedSetContains(seeds, mainPkg.Func("callbackOwner")) {
		t.Fatalf("callbackOwner was unexpectedly seeded")
	}
}

func TestTargetedModeFindsWrapperPathMissedBySeedModes(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "targeted_wrapper"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	caller := mainPkg.Func("caller")
	wrapper := mainPkg.Func("wrapper")
	if caller == nil || wrapper == nil {
		t.Fatalf("fixture functions not found: caller=%v wrapper=%v", caller, wrapper)
	}

	reverseOnly := NewFunctionIndexSeedSet()
	reverseOnly.Add(caller, FunctionIndexSeedReasonReversePath)
	reverseOnlyFlow := analyzeFunctionValueFlow(prog, BuildFunctionRefIndexForSeeds(reverseOnly, FunctionRefIndexOptions{}), []*ssa.Function{root})
	if hasExternalSurface(reverseOnlyFlow.ExternalSurfaces, "external") {
		t.Fatalf("reverse-only seeds unexpectedly found external: %+v", reverseOnlyFlow.ExternalSurfaces)
	}

	httpOnly := NewFunctionIndexSeedSet()
	httpOnly.Add(wrapper, FunctionIndexSeedReasonHTTPSink)
	httpOnlyFlow := analyzeFunctionValueFlow(prog, BuildFunctionRefIndexForSeeds(httpOnly, FunctionRefIndexOptions{}), []*ssa.Function{root})
	if hasExternalSurface(httpOnlyFlow.ExternalSurfaces, "external") {
		t.Fatalf("http-only seeds unexpectedly found external: %+v", httpOnlyFlow.ExternalSurfaces)
	}

	targetedSeeds := NewFunctionIndexSeedSet()
	targetedSeeds.Merge(reverseOnly)
	targetedSeeds.Merge(httpOnly)
	targeted := analyzeFunctionValueFlow(prog, BuildFunctionRefIndexForSeeds(targetedSeeds, FunctionRefIndexOptions{}), []*ssa.Function{root})
	if !hasExternalSurface(targeted.ExternalSurfaces, "external") {
		t.Fatalf("targeted union seeds missed external: %+v", targeted.ExternalSurfaces)
	}
	if !hasRegistrationOwner(targeted.RegistrationSites, "wrapper") {
		t.Fatalf("targeted union seeds did not recover wrapper registration: %+v", targeted.RegistrationSites)
	}
}

func TestFunctionValueFlowRecordsStoredField(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "struct_field_handler"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	result, err := Probe(prog, mainPkg, []*ssa.Function{root})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRegistrationEdge(result.RegistrationSites, EdgeFunctionValueStoredField) {
		t.Fatalf("missing stored-field registration: %+v", result.RegistrationSites)
	}
	if !hasExternalSurface(result.ExternalSurfaces, "external") {
		t.Fatalf("missing external surface: %+v", result.ExternalSurfaces)
	}
}

func TestFunctionValueFlowRecordsWrapperChain(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "wrapper_callback"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	result, err := Probe(prog, mainPkg, []*ssa.Function{root})
	if err != nil {
		t.Fatal(err)
	}
	if !hasWrapperTarget(result.WrapperChains, "middleware") {
		t.Fatalf("missing middleware wrapper chain: %+v", result.WrapperChains)
	}
}

func hasRefKind(refs []FunctionRef, kind string) bool {
	for _, ref := range refs {
		if ref.Kind == kind {
			return true
		}
	}
	return false
}

func hasWrapperTarget(chains []WrapperChain, objectName string) bool {
	for _, chain := range chains {
		for _, link := range chain.Links {
			if link.To.Identity.ObjectName == objectName {
				return true
			}
		}
	}
	return false
}

func hasRegistrationEdge(sites []RegistrationSite, edge string) bool {
	for _, site := range sites {
		if site.EdgeKind == edge {
			return true
		}
	}
	return false
}

func hasRegistrationOwner(sites []RegistrationSite, objectName string) bool {
	for _, site := range sites {
		if site.Node.Identity.ObjectName == objectName {
			return true
		}
	}
	return false
}

func hasExternalSurface(surfaces []ExternalSurface, objectName string) bool {
	for _, surface := range surfaces {
		if surface.Node.Identity.ObjectName == objectName {
			return true
		}
	}
	return false
}

func seedSetContains(seeds *FunctionIndexSeedSet, owner *ssa.Function) bool {
	for _, seeded := range seeds.Owners() {
		if seeded == owner {
			return true
		}
	}
	return false
}

type comparableFunctionRefStats struct {
	ScannedFunctions          int
	ScannedBlocks             int
	ScannedInstructions       int
	DiscoveredFunctionSources int
	ClosureSources            int
	OperandRefs               int
	CallArgRefs               int
	StoreRefs                 int
	ReturnRefs                int
	SkippedFunctions          int
}

func stableFunctionRefStats(stats FunctionRefIndexStats) comparableFunctionRefStats {
	return comparableFunctionRefStats{
		ScannedFunctions:          stats.ScannedFunctions,
		ScannedBlocks:             stats.ScannedBlocks,
		ScannedInstructions:       stats.ScannedInstructions,
		DiscoveredFunctionSources: stats.DiscoveredFunctionSources,
		ClosureSources:            stats.ClosureSources,
		OperandRefs:               stats.OperandRefs,
		CallArgRefs:               stats.CallArgRefs,
		StoreRefs:                 stats.StoreRefs,
		ReturnRefs:                stats.ReturnRefs,
		SkippedFunctions:          stats.SkippedFunctions,
	}
}
