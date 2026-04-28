package entrypath

import (
	"go/types"
	"sort"
	"time"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const (
	FunctionIndexSeedReasonReversePath       = "reverse_path"
	FunctionIndexSeedReasonBoundary          = "boundary"
	FunctionIndexSeedReasonHTTPSink          = "http_sink"
	FunctionIndexSeedReasonOnDemandExpansion = "on_demand_expansion"
	FunctionIndexSeedReasonBridge            = "bridge"
	FunctionIndexSeedReasonOracleBridge      = "oracle_bridge"
)

type FunctionIndexSeedSet struct {
	owners                         map[*ssa.Function]map[string]bool
	rejectedNonHTTPInterfaceOwners map[*ssa.Function]bool
	BoundaryEvidence               []BoundaryPredicateEvidence
	Diagnostics                    []Diagnostic
}

// NewFunctionIndexSeedSet creates the SeedSet used to bound function-reference
// scanning. Owners are selected from reverse paths, BoundarySeed evidence, or
// local expansion depending on the diagnostic mode.
func NewFunctionIndexSeedSet() *FunctionIndexSeedSet {
	return &FunctionIndexSeedSet{
		owners:                         map[*ssa.Function]map[string]bool{},
		rejectedNonHTTPInterfaceOwners: map[*ssa.Function]bool{},
	}
}

func (set *FunctionIndexSeedSet) Add(owner *ssa.Function, reason string) {
	if set == nil || owner == nil || reason == "" {
		return
	}
	if set.owners == nil {
		set.owners = map[*ssa.Function]map[string]bool{}
	}
	reasons := set.owners[owner]
	if reasons == nil {
		reasons = map[string]bool{}
		set.owners[owner] = reasons
	}
	reasons[reason] = true
}

func (set *FunctionIndexSeedSet) Merge(other *FunctionIndexSeedSet) {
	if set == nil || other == nil {
		return
	}
	for _, owner := range other.Owners() {
		for _, reason := range other.Reasons(owner) {
			set.Add(owner, reason)
		}
	}
	for owner := range other.rejectedNonHTTPInterfaceOwners {
		set.AddRejectedNonHTTPInterfaceOwner(owner)
	}
	set.BoundaryEvidence = append(set.BoundaryEvidence, other.BoundaryEvidence...)
	set.Diagnostics = append(set.Diagnostics, other.Diagnostics...)
}

func (set *FunctionIndexSeedSet) Len() int {
	if set == nil {
		return 0
	}
	return len(set.owners)
}

func (set *FunctionIndexSeedSet) Owners() []*ssa.Function {
	if set == nil {
		return nil
	}
	owners := make([]*ssa.Function, 0, len(set.owners))
	for owner := range set.owners {
		if owner != nil {
			owners = append(owners, owner)
		}
	}
	sort.Slice(owners, func(i, j int) bool { return functionSortKey(owners[i]) < functionSortKey(owners[j]) })
	return owners
}

func (set *FunctionIndexSeedSet) Reasons(owner *ssa.Function) []string {
	if set == nil || owner == nil {
		return nil
	}
	reasons := make([]string, 0, len(set.owners[owner]))
	for reason := range set.owners[owner] {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

func (set *FunctionIndexSeedSet) AddRejectedNonHTTPInterfaceOwner(owner *ssa.Function) {
	if set == nil || owner == nil {
		return
	}
	if set.rejectedNonHTTPInterfaceOwners == nil {
		set.rejectedNonHTTPInterfaceOwners = map[*ssa.Function]bool{}
	}
	set.rejectedNonHTTPInterfaceOwners[owner] = true
}

func (set *FunctionIndexSeedSet) AddBoundaryEvidence(evidence BoundaryPredicateEvidence) {
	if set == nil || evidence.Owner == nil {
		return
	}
	set.Add(evidence.Owner, FunctionIndexSeedReasonBoundary)
	if evidence.Predicate == netHTTPPackagePath {
		set.Add(evidence.Owner, FunctionIndexSeedReasonHTTPSink)
	}
	set.BoundaryEvidence = append(set.BoundaryEvidence, evidence)
}

func (set *FunctionIndexSeedSet) Stats() FunctionIndexSeedStats {
	var stats FunctionIndexSeedStats
	if set == nil {
		return stats
	}
	stats.OwnerCount = set.Len()
	stats.BoundaryEvidenceCount = len(set.BoundaryEvidence)
	stats.RejectedNonHTTPInterfaceOwners = len(set.rejectedNonHTTPInterfaceOwners)
	for owner := range set.owners {
		reasons := set.Reasons(owner)
		for _, reason := range reasons {
			switch reason {
			case FunctionIndexSeedReasonReversePath:
				stats.ReversePathOwners++
			case FunctionIndexSeedReasonBoundary:
				stats.BoundarySeedOwners++
			case FunctionIndexSeedReasonHTTPSink:
				stats.HTTPSinkOwners++
			case FunctionIndexSeedReasonOnDemandExpansion:
				stats.OnDemandExpansionOwners++
			case FunctionIndexSeedReasonBridge:
				stats.BridgeOwners++
			case FunctionIndexSeedReasonOracleBridge:
				stats.OracleBridgeOwners++
			}
		}
	}
	return stats
}

func reversePathSeedSet(graph *callgraph.Graph, regionRoots []*ssa.Function) *FunctionIndexSeedSet {
	seeds := NewFunctionIndexSeedSet()
	for _, root := range sortedUniqueFunctions(regionRoots) {
		seeds.Add(root, FunctionIndexSeedReasonReversePath)
		if graph == nil {
			continue
		}
		rootNode := graph.Nodes[root]
		if rootNode == nil {
			continue
		}
		callers, _ := reverseReachableCallers(rootNode)
		for _, caller := range callers {
			seeds.Add(caller, FunctionIndexSeedReasonReversePath)
		}
	}
	owners := seeds.Owners()
	for _, owner := range owners {
		for _, callee := range staticCalleesInFunction(owner) {
			seeds.Add(callee, FunctionIndexSeedReasonReversePath)
		}
	}
	return seeds
}

func staticCalleesInFunction(owner *ssa.Function) []*ssa.Function {
	if owner == nil {
		return nil
	}
	seen := map[*ssa.Function]bool{}
	var callees []*ssa.Function
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok || call.Common() == nil {
				continue
			}
			callee := call.Common().StaticCallee()
			if callee == nil || seen[callee] {
				continue
			}
			seen[callee] = true
			callees = append(callees, callee)
		}
	}
	sort.Slice(callees, func(i, j int) bool { return functionSortKey(callees[i]) < functionSortKey(callees[j]) })
	return callees
}

func httpSinkSeedSet(prog *ssa.Program) *FunctionIndexSeedSet {
	return httpSinkSeedSetWithOptions(prog, time.Now(), FunctionRefIndexOptions{})
}

func httpSinkSeedSetWithOptions(prog *ssa.Program, started time.Time, options FunctionRefIndexOptions) *FunctionIndexSeedSet {
	seeds := NewFunctionIndexSeedSet()
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil {
			continue
		}
		if functionIndexBudgetExceeded(started, options.Budget) {
			seeds.Diagnostics = append(seeds.Diagnostics, Diagnostic{
				Kind:     "function_ref_index_budget_exceeded",
				Reason:   "function reference index budget exceeded during HTTP seed discovery; downstream results use partial seeds",
				Function: functionString(fn),
			})
			break
		}
		if evidence := (netHTTPBoundaryPredicate{}).MatchOwner(fn); len(evidence) > 0 {
			for _, item := range evidence {
				seeds.AddBoundaryEvidence(item)
			}
		} else if functionOwnsNonHTTPInterfaceSink(fn) {
			seeds.AddRejectedNonHTTPInterfaceOwner(fn)
		}
	}
	return seeds
}

type TargetedExpansionOptions struct {
	MaxFunctions int
	MaxDepth     int
	MaxDuration  time.Duration
	MaxQueue     int
}

func (options TargetedExpansionOptions) WithDefaults() TargetedExpansionOptions {
	if options.MaxFunctions == 0 {
		options.MaxFunctions = 10000
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = 1
	}
	if options.MaxDuration == 0 {
		options.MaxDuration = 30 * time.Second
	}
	if options.MaxQueue == 0 {
		options.MaxQueue = 100000
	}
	return options
}

func targetedSeedSet(prog *ssa.Program, graph *callgraph.Graph, regionRoots []*ssa.Function, started time.Time, options FunctionRefIndexOptions, targetedOptions TargetedExpansionOptions) *FunctionIndexSeedSet {
	seeds := reversePathSeedSet(graph, regionRoots)
	httpSeeds := httpSinkSeedSetWithOptions(prog, started, options)
	seeds.Merge(httpSeeds)
	if hasSeedDiagnostic(seeds, "function_ref_index_budget_exceeded") {
		appendSeedDiagnosticOnce(seeds, "targeted_index_budget_exceeded", "targeted seed discovery exceeded the function index budget")
	}
	expandTargetedSeeds(seeds, started, options, targetedOptions)
	return seeds
}

func expandTargetedSeeds(seeds *FunctionIndexSeedSet, started time.Time, options FunctionRefIndexOptions, targetedOptions TargetedExpansionOptions) {
	for depth := 0; depth < targetedOptions.MaxDepth; depth++ {
		if functionIndexBudgetExceeded(started, options.Budget) {
			seeds.Diagnostics = append(seeds.Diagnostics, Diagnostic{
				Kind:   "function_ref_index_budget_exceeded",
				Reason: "function reference index budget exceeded during targeted expansion; downstream results use partial seeds",
			})
			appendSeedDiagnosticOnce(seeds, "targeted_index_budget_exceeded", "targeted expansion exceeded the function index budget")
			return
		}
		if targetedExpansionDurationExceeded(started, targetedOptions.MaxDuration) {
			appendSeedDiagnosticOnce(seeds, "targeted_expansion_budget_exceeded", "targeted expansion elapsed duration exceeded")
			return
		}
		indexOptions := options
		indexOptions.Budget = remainingFunctionIndexBudget(started, options.Budget)
		index := BuildFunctionRefIndexForSeeds(seeds, indexOptions)
		added := 0
		processed := 0
		for _, refs := range index.Uses {
			for _, ref := range refs {
				processed++
				if targetedOptions.MaxQueue > 0 && processed > targetedOptions.MaxQueue {
					appendSeedDiagnosticOnce(seeds, "targeted_queue_overflow", "targeted expansion work queue limit exceeded")
					return
				}
				for _, expansion := range expansionOwnersForRef(ref) {
					if targetedOptions.MaxFunctions > 0 && seeds.Len() >= targetedOptions.MaxFunctions {
						appendSeedDiagnosticOnce(seeds, "targeted_expansion_budget_exceeded", "targeted expansion function limit exceeded")
						return
					}
					before := seeds.Len()
					seeds.Add(expansion.Owner, FunctionIndexSeedReasonOnDemandExpansion)
					if seeds.Len() > before {
						added++
					}
				}
			}
		}
		seeds.Diagnostics = append(seeds.Diagnostics, index.Diagnostics...)
		if hasSeedDiagnostic(seeds, "function_ref_index_budget_exceeded") {
			appendSeedDiagnosticOnce(seeds, "targeted_index_budget_exceeded", "targeted expansion index scan exceeded the function index budget")
		}
		if added == 0 {
			appendSeedDiagnosticOnce(seeds, "targeted_completed", "targeted expansion reached a fixed point")
			return
		}
	}
	appendSeedDiagnosticOnce(seeds, "targeted_expansion_budget_exceeded", "targeted expansion depth limit exceeded")
}

func targetedExpansionDurationExceeded(started time.Time, maxDuration time.Duration) bool {
	return maxDuration > 0 && time.Since(started) >= maxDuration
}

func hasSeedDiagnostic(seeds *FunctionIndexSeedSet, kind string) bool {
	if seeds == nil {
		return false
	}
	for _, diagnostic := range seeds.Diagnostics {
		if diagnostic.Kind == kind {
			return true
		}
	}
	return false
}

func appendSeedDiagnosticOnce(seeds *FunctionIndexSeedSet, kind string, reason string) {
	if seeds == nil || hasSeedDiagnostic(seeds, kind) {
		return
	}
	seeds.Diagnostics = append(seeds.Diagnostics, Diagnostic{Kind: kind, Reason: reason})
}

type targetedExpansionCause string

const (
	targetedExpansionStaticCalleeArg targetedExpansionCause = "static_callee_arg"
	targetedExpansionReturnWrapper   targetedExpansionCause = "return_wrapper"
)

type targetedExpansion struct {
	Owner *ssa.Function
	Cause targetedExpansionCause
}

func expansionOwnersForRef(ref FunctionRef) []targetedExpansion {
	instr := ref.Instruction
	if instr == nil {
		return nil
	}
	if ref.Kind == "return" && ref.Owner != nil {
		return []targetedExpansion{{Owner: ref.Owner, Cause: targetedExpansionReturnWrapper}}
	}
	if ref.Kind == "capture" {
		if closure, ok := instr.(*ssa.MakeClosure); ok {
			return []targetedExpansion{{Owner: closureFunction(closure), Cause: targetedExpansionReturnWrapper}}
		}
	}
	call, ok := instr.(ssa.CallInstruction)
	if !ok || call.Common() == nil {
		return nil
	}
	if ref.Kind != "call_arg" && ref.Kind != "go_arg" {
		return nil
	}
	callee := call.Common().StaticCallee()
	if callee == nil {
		return nil
	}
	return []targetedExpansion{{Owner: callee, Cause: targetedExpansionStaticCalleeArg}}
}

func functionIndexBudgetExceeded(started time.Time, budget time.Duration) bool {
	return budget > 0 && time.Since(started) >= budget
}

func functionOwnsHTTPSink(fn *ssa.Function) bool {
	return len(netHTTPBoundaryPredicate{}.MatchOwner(fn)) > 0
}

func instructionHasHTTPSink(instr ssa.Instruction) bool {
	return len(netHTTPBoundaryPredicate{}.instructionEvidence(nil, instr)) > 0
}

func functionOwnsNonHTTPInterfaceSink(fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	if signatureAcceptsNonHTTPInterface(fn.Signature) {
		return true
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instructionHasNonHTTPInterfaceSink(instr) {
				return true
			}
		}
	}
	return false
}

func signatureAcceptsNonHTTPInterface(sig *types.Signature) bool {
	if sig == nil || sig.Params() == nil {
		return false
	}
	for i := 0; i < sig.Params().Len(); i++ {
		typ := sig.Params().At(i).Type()
		if isInterfaceType(typ) && !isHTTPBoundaryType(typ) {
			return true
		}
	}
	return false
}

func instructionHasNonHTTPInterfaceSink(instr ssa.Instruction) bool {
	call, ok := instr.(ssa.CallInstruction)
	if !ok || call.Common() == nil {
		return false
	}
	common := call.Common()
	for i := range common.Args {
		paramType := callParamType(common, i)
		if isInterfaceType(paramType) && !isHTTPBoundaryType(paramType) {
			return true
		}
	}
	return false
}

func callHasHTTPBoundary(call ssa.CallInstruction) bool {
	if call == nil || call.Common() == nil {
		return false
	}
	common := call.Common()
	if common.StaticCallee() != nil && signatureAcceptsHandler(common.StaticCallee().Signature) {
		return true
	}
	for i, arg := range common.Args {
		if arg != nil && isHTTPBoundaryType(arg.Type()) {
			return true
		}
		if isHTTPBoundaryType(callParamType(common, i)) {
			return true
		}
	}
	return false
}

func valueOrAddressHasHTTPBoundary(value ssa.Value) bool {
	if value == nil {
		return false
	}
	if isHTTPBoundaryType(value.Type()) {
		return true
	}
	switch typed := value.(type) {
	case *ssa.FieldAddr:
		return isHTTPBoundaryType(typed.Type()) || typeHasServeHTTP(typed.X.Type())
	case *ssa.UnOp:
		return typed.X != nil && isHTTPBoundaryType(typed.X.Type())
	default:
		return false
	}
}
