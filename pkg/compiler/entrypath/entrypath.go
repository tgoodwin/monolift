package entrypath

import (
	"errors"
	"go/token"
	"go/types"
	"runtime"
	"sort"
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func Probe(prog *ssa.Program, mainPkg *ssa.Package, regionRoots []*ssa.Function) (ProbeResult, error) {
	return ProbeWithOptions(prog, mainPkg, regionRoots, ProbeOptions{})
}

func ProbeWithOptions(prog *ssa.Program, mainPkg *ssa.Package, regionRoots []*ssa.Function, options ProbeOptions) (ProbeResult, error) {
	if prog == nil {
		return ProbeResult{}, errors.New("entrypath: nil SSA program")
	}
	mode := options.FunctionIndexMode.OrDefault()
	if !mode.Valid() {
		return ProbeResult{}, errors.New("entrypath: invalid function index mode")
	}
	boundaryMode := options.BoundaryDiscoveryMode.OrDefault()
	if !boundaryMode.Valid() {
		return ProbeResult{}, errors.New("entrypath: invalid boundary discovery mode")
	}
	start := time.Now()
	phases := newPhaseRecorder(options.PhaseObserver)
	graph := phases.runCallGraph("callgraph", func() graphBuild {
		return buildApplicationCallGraph(prog, mainPkg)
	})
	reverse := phases.runReverseBFS("reverse_bfs", func() reverseBFSResult {
		return reverseBFS(prog, graph.graph, regionRoots)
	})
	touchpoints := reverse.Touchpoints
	reverseDiagnostics := reverse.Diagnostics
	var seedStats FunctionIndexSeedStats
	var boundaryStats BoundaryDiscoveryStats
	var bridgeStats BridgeDiscoveryStats
	var bridgeIndexPriority bridgeIndexPriorityContext
	var indexDiagnostics []Diagnostic
	var oracleReverseOwners []*ssa.Function
	var oracleAdjacentOwners []*ssa.Function
	var oracleBoundaryCandidateOwners []*ssa.Function
	var oracleSeeds *FunctionIndexSeedSet
	indexOptions := FunctionRefIndexOptions{
		ProgressInstructionInterval: options.FunctionRefIndexProgressInterval,
		PhaseObserver:               options.PhaseObserver,
		Budget:                      options.FunctionRefIndexBudget,
		MaxFunctions:                options.FunctionRefIndexMaxFunctions,
	}
	var refIndex FunctionRefIndex
	switch mode {
	case FunctionIndexModeAll:
		refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
			return BuildFunctionRefIndexWithOptions(prog, indexOptions)
		})
	case FunctionIndexModeReversePath:
		seeds := reversePathSeedSet(graph.graph, regionRoots)
		oracleSeeds = seeds
		seedStats = seeds.Stats()
		refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
			return BuildFunctionRefIndexForSeeds(seeds, indexOptions)
		})
	case FunctionIndexModeHTTPSinks:
		seedStarted := time.Now()
		if boundaryMode == BoundaryDiscoveryModeFrontier {
			frontier := discoverBoundaryFrontier(graph.graph, regionRoots, boundaryFrontierOptionsFromProbe(options), phases)
			assemblyStarted := phases.start("boundary_seed_set_assembly")
			seeds := frontier.BoundarySeeds
			oracleReverseOwners = frontier.ReverseOwners
			oracleAdjacentOwners = frontier.AdjacentOwners
			oracleBoundaryCandidateOwners = frontier.BoundaryCandidateOwners
			oracleSeeds = seeds
			boundaryStats = frontier.Stats
			boundaryStats.SeedSetOwners = seeds.Len()
			phases.finish("boundary_seed_set_assembly", assemblyStarted)
			seedStats = seeds.Stats()
			indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
			finalIndexOptions := indexOptions
			refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
				return BuildFunctionRefIndexForSeeds(seeds, finalIndexOptions)
			})
		} else {
			refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
				seeds := httpSinkSeedSetWithOptions(prog, seedStarted, indexOptions)
				oracleSeeds = seeds
				boundaryStats = boundaryDiscoveryStatsForSeeds(boundaryMode, seeds)
				seedStats = seeds.Stats()
				indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
				finalIndexOptions := indexOptions
				finalIndexOptions.Budget = remainingFunctionIndexBudget(seedStarted, indexOptions.Budget)
				return BuildFunctionRefIndexForSeeds(seeds, finalIndexOptions)
			})
		}
	case FunctionIndexModeTargeted:
		seedStarted := time.Now()
		targetedOptions := TargetedExpansionOptions{
			MaxFunctions: options.TargetedExpansionMaxFunctions,
			MaxDepth:     options.TargetedExpansionMaxDepth,
			MaxDuration:  options.TargetedExpansionMaxDuration,
			MaxQueue:     options.TargetedExpansionMaxQueue,
		}.WithDefaults()
		if boundaryMode == BoundaryDiscoveryModeFrontier {
			frontier := discoverBoundaryFrontier(graph.graph, regionRoots, boundaryFrontierOptionsFromProbe(options), phases)
			assemblyStarted := phases.start("boundary_seed_set_assembly")
			seeds := seedSetFromOwners(frontier.ReverseOwners, FunctionIndexSeedReasonReversePath)
			seeds.Merge(frontier.BoundarySeeds)
			oracleReverseOwners = frontier.ReverseOwners
			oracleAdjacentOwners = frontier.AdjacentOwners
			oracleBoundaryCandidateOwners = frontier.BoundaryCandidateOwners
			boundaryStats = frontier.Stats
			boundaryStats.SeedSetOwners = seeds.Len()
			phases.finish("boundary_seed_set_assembly", assemblyStarted)
			expandTargetedSeeds(seeds, seedStarted, indexOptions, targetedOptions)
			oracleSeeds = seeds
			seedStats = seeds.Stats()
			indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
			finalIndexOptions := indexOptions
			if targetedOptions.MaxFunctions > 0 && (finalIndexOptions.MaxFunctions == 0 || targetedOptions.MaxFunctions < finalIndexOptions.MaxFunctions) {
				finalIndexOptions.MaxFunctions = targetedOptions.MaxFunctions
			}
			refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
				return BuildFunctionRefIndexForSeeds(seeds, finalIndexOptions)
			})
		} else {
			refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
				seeds := targetedSeedSet(prog, graph.graph, regionRoots, seedStarted, indexOptions, targetedOptions)
				oracleSeeds = seeds
				boundaryStats = boundaryDiscoveryStatsForSeeds(boundaryMode, seeds)
				seedStats = seeds.Stats()
				indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
				finalIndexOptions := indexOptions
				finalIndexOptions.Budget = remainingFunctionIndexBudget(seedStarted, indexOptions.Budget)
				if targetedOptions.MaxFunctions > 0 && (finalIndexOptions.MaxFunctions == 0 || targetedOptions.MaxFunctions < finalIndexOptions.MaxFunctions) {
					finalIndexOptions.MaxFunctions = targetedOptions.MaxFunctions
				}
				return BuildFunctionRefIndexForSeeds(seeds, finalIndexOptions)
			})
		}
	case FunctionIndexModeBridge:
		bridgeStarted := phases.start("bridge_seed_discovery")
		bridge := discoverBridgeSeedsWithOracleSpec(prog, reverse.TouchpointFunctions, bridgeOptionsFromProbe(options), options.OracleSpec)
		phases.finish("bridge_seed_discovery", bridgeStarted)
		seeds := bridge.Seeds
		oracleSeeds = seeds
		bridgeStats = bridge.Stats
		bridgeIndexPriority = bridge.IndexPriority
		boundaryStats = boundaryDiscoveryStatsForSeeds(boundaryMode, seeds)
		seedStats = seeds.Stats()
		indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
		finalIndexOptions := bridgeFunctionIndexOptions(indexOptions)
		refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
			return buildBridgeFunctionRefIndex(seeds, bridgeIndexPriority, finalIndexOptions)
		})
	case FunctionIndexModeOracleBridge:
		seedStarted := time.Now()
		bridgeStarted := phases.start("oracle_bridge_seed_discovery")
		seeds := oracleBridgeSeedSet(prog, options.OracleSpec, oracleBridgeOptionsFromProbe(options))
		phases.finish("oracle_bridge_seed_discovery", bridgeStarted)
		oracleSeeds = seeds
		boundaryStats = boundaryDiscoveryStatsForSeeds(boundaryMode, seeds)
		seedStats = seeds.Stats()
		indexDiagnostics = append(indexDiagnostics, seeds.Diagnostics...)
		finalIndexOptions := indexOptions
		finalIndexOptions.Budget = remainingFunctionIndexBudget(seedStarted, indexOptions.Budget)
		refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
			return BuildFunctionRefIndexForSeeds(seeds, finalIndexOptions)
		})
	default:
		refIndex = phases.runFunctionRefIndex("function_ref_index", func() FunctionRefIndex {
			return BuildFunctionRefIndexWithOptions(prog, indexOptions)
		})
	}
	flow := phases.runFunctionFlow("function_value_flow", func() functionFlowResult {
		return analyzeFunctionValueFlow(prog, refIndex, regionRoots)
	})
	stats := callgraphStats(prog, graph.graph)
	stats.CallgraphAlgorithm = graph.algorithm
	stats.WallClockMillis = time.Since(start).Milliseconds()
	stats.PeakRSSBytes = processMemoryBytes()
	stats.PhaseTimings = phases.timings()
	stats.FunctionRefIndex = refIndex.Stats
	stats.FunctionIndexSeeds = seedStats
	stats.BoundaryDiscovery = finalizeBoundaryDiscoveryStats(boundaryStats, refIndex)
	stats.BridgeDiscovery = finalizeBridgeDiscoveryStats(bridgeStats, oracleSeeds, refIndex, bridgeIndexPriority)
	result := ProbeResult{
		RegionRoots:         traceNodesForFunctions(prog, sortedUniqueFunctions(regionRoots)),
		ExternalSurfaces:    flow.ExternalSurfaces,
		RegistrationSites:   flow.RegistrationSites,
		WrapperChains:       flow.WrapperChains,
		RegionTouchpoints:   touchpoints,
		BootStartCandidates: []TraceNode{},
		OracleTrace:         oracleTraceForSpec(prog, options.OracleSpec, touchpoints, oracleReverseOwners, oracleAdjacentOwners, oracleBoundaryCandidateOwners, oracleSeeds, refIndex, flow),
		Diagnostics:         append(append(append(append(append([]Diagnostic{}, graph.diagnostic...), reverseDiagnostics...), indexDiagnostics...), refIndex.Diagnostics...), flow.Diagnostics...),
		Stats:               stats,
	}
	return result, nil
}

func finalizeBoundaryDiscoveryStats(stats BoundaryDiscoveryStats, refIndex FunctionRefIndex) BoundaryDiscoveryStats {
	stats.FinalIndexedOwners = refIndex.Stats.ScannedFunctions
	if diagnosticsContain(refIndex.Diagnostics, "function_ref_index_budget_exceeded") {
		stats = appendBoundaryBudgetStop(stats, BudgetStopReason{Budget: "index", Reason: "index_budget"})
	}
	return stats
}

func appendBoundaryBudgetStop(stats BoundaryDiscoveryStats, stop BudgetStopReason) BoundaryDiscoveryStats {
	for _, existing := range stats.BudgetStops {
		if existing.Budget == stop.Budget && existing.Reason == stop.Reason {
			return stats
		}
	}
	stats.BudgetStops = append(stats.BudgetStops, stop)
	sort.Slice(stats.BudgetStops, func(i, j int) bool {
		if stats.BudgetStops[i].Budget == stats.BudgetStops[j].Budget {
			return stats.BudgetStops[i].Reason < stats.BudgetStops[j].Reason
		}
		return stats.BudgetStops[i].Budget < stats.BudgetStops[j].Budget
	})
	for _, reason := range stats.StopReasons {
		if reason == stop.Reason {
			return stats
		}
	}
	stats.StopReasons = append(stats.StopReasons, stop.Reason)
	sort.Strings(stats.StopReasons)
	return stats
}

func diagnosticsContain(diagnostics []Diagnostic, kind string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			return true
		}
	}
	return false
}

func remainingFunctionIndexBudget(started time.Time, budget time.Duration) time.Duration {
	if budget <= 0 {
		return budget
	}
	remaining := budget - time.Since(started)
	if remaining <= 0 {
		return time.Nanosecond
	}
	return remaining
}

func bridgeFunctionIndexOptions(options FunctionRefIndexOptions) FunctionRefIndexOptions {
	// Bridge seed discovery has its own explicit budget. Keep the function-index
	// budget phase-local so diagnostics and defaults describe indexing cost.
	return options
}

type phaseRecorder struct {
	observer func(PhaseEvent)
	timing   []PhaseTiming
}

func newPhaseRecorder(observer func(PhaseEvent)) *phaseRecorder {
	return &phaseRecorder{observer: observer}
}

func (rec *phaseRecorder) timings() []PhaseTiming {
	return append([]PhaseTiming(nil), rec.timing...)
}

func (rec *phaseRecorder) start(name string) time.Time {
	if rec.observer != nil {
		rec.observer(PhaseEvent{Name: name, Status: "start", PeakRSSBytes: processMemoryBytes()})
	}
	return time.Now()
}

func (rec *phaseRecorder) finish(name string, started time.Time) {
	timing := PhaseTiming{
		Name:            name,
		WallClockMillis: time.Since(started).Milliseconds(),
		PeakRSSBytes:    processMemoryBytes(),
	}
	rec.timing = append(rec.timing, timing)
	if rec.observer != nil {
		rec.observer(PhaseEvent{
			Name:            name,
			Status:          "end",
			WallClockMillis: timing.WallClockMillis,
			PeakRSSBytes:    timing.PeakRSSBytes,
		})
	}
}

func (rec *phaseRecorder) runCallGraph(name string, fn func() graphBuild) graphBuild {
	started := rec.start(name)
	defer rec.finish(name, started)
	return fn()
}

func (rec *phaseRecorder) runReverseBFS(name string, fn func() reverseBFSResult) reverseBFSResult {
	started := rec.start(name)
	defer rec.finish(name, started)
	return fn()
}

func (rec *phaseRecorder) runFunctionRefIndex(name string, fn func() FunctionRefIndex) FunctionRefIndex {
	started := rec.start(name)
	defer rec.finish(name, started)
	return fn()
}

func (rec *phaseRecorder) runFunctionFlow(name string, fn func() functionFlowResult) functionFlowResult {
	started := rec.start(name)
	defer rec.finish(name, started)
	return fn()
}

func traceNodesForFunctions(prog *ssa.Program, funcs []*ssa.Function) []TraceNode {
	out := make([]TraceNode, 0, len(funcs))
	for _, fn := range funcs {
		out = append(out, traceNodeForFunction(prog, fn))
	}
	return out
}

func traceNodeForFunction(prog *ssa.Program, fn *ssa.Function) TraceNode {
	if fn == nil {
		return TraceNode{}
	}
	identity := reportv2.SymbolIdentity{
		ModulePath:  functionPackagePath(fn),
		PackagePath: functionPackagePath(fn),
		ObjectName:  functionObjectName(fn),
		Kind:        functionKind(fn),
	}
	label := fn.String()
	position := sourcePosition(prog, fn.Pos())
	id := identity.PackagePath + "." + identity.ObjectName + "@" + position.Filename + ":" + itoa(position.Line)
	return TraceNode{
		ID:       id,
		Label:    label,
		Identity: identity,
		Position: position,
	}
}

func sourcePosition(prog *ssa.Program, pos token.Pos) SourcePosition {
	if prog == nil || prog.Fset == nil || !pos.IsValid() {
		return SourcePosition{}
	}
	p := prog.Fset.Position(pos)
	return SourcePosition{
		Filename: p.Filename,
		Line:     p.Line,
		Column:   p.Column,
	}
}

func sortedUniqueFunctions(funcs []*ssa.Function) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	out := make([]*ssa.Function, 0, len(funcs))
	for _, fn := range funcs {
		if fn == nil || seen[fn] {
			continue
		}
		seen[fn] = true
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

func countProgramFunctions(prog *ssa.Program) int {
	if prog == nil {
		return 0
	}
	return len(ssautil.AllFunctions(prog))
}

func functionPackagePath(fn *ssa.Function) string {
	if fn == nil || fn.Package() == nil || fn.Package().Pkg == nil {
		return ""
	}
	return fn.Package().Pkg.Path()
}

func functionObjectName(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if recv := fn.Signature.Recv(); recv != nil {
		return receiverName(recv.Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func receiverName(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Pointer:
		return "(*" + receiverName(t.Elem()) + ")"
	case *types.Named:
		return t.Obj().Name()
	default:
		return types.TypeString(typ, func(*types.Package) string { return "" })
	}
}

func functionKind(fn *ssa.Function) string {
	if fn != nil && fn.Signature != nil && fn.Signature.Recv() != nil {
		return "method"
	}
	return "function"
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func processMemoryBytes() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.Sys
}
