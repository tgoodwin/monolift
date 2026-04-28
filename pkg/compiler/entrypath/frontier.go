package entrypath

import (
	"sort"
	"time"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

const (
	defaultBoundaryFrontierMaxOwners   = 10000
	defaultBoundaryFrontierDepth       = 1
	defaultBoundaryFrontierMaxDuration = 30 * time.Second
)

type BoundaryFrontierOptions struct {
	MaxOwners                  int
	MaxReverseOwners           int
	MaxAdjacentOwners          int
	MaxBoundaryCandidateOwners int
	MaxDepth                   int
	MaxPackages                int
	MaxDuration                time.Duration
}

func boundaryFrontierOptionsFromProbe(options ProbeOptions) BoundaryFrontierOptions {
	return BoundaryFrontierOptions{
		MaxOwners:                  options.BoundaryFrontierMaxOwners,
		MaxReverseOwners:           options.BoundaryFrontierMaxReverseOwners,
		MaxAdjacentOwners:          options.BoundaryFrontierMaxAdjacentOwners,
		MaxBoundaryCandidateOwners: options.BoundaryFrontierMaxBoundaryCandidates,
		MaxDepth:                   options.BoundaryFrontierDepth,
		MaxPackages:                options.BoundaryFrontierMaxPackages,
		MaxDuration:                options.BoundaryFrontierMaxDuration,
	}.WithDefaults()
}

func (options BoundaryFrontierOptions) WithDefaults() BoundaryFrontierOptions {
	if options.MaxReverseOwners == 0 {
		options.MaxReverseOwners = options.MaxOwners
	}
	if options.MaxReverseOwners == 0 {
		options.MaxReverseOwners = defaultBoundaryFrontierMaxOwners
	}
	if options.MaxAdjacentOwners == 0 {
		options.MaxAdjacentOwners = options.MaxOwners
	}
	if options.MaxAdjacentOwners == 0 {
		options.MaxAdjacentOwners = defaultBoundaryFrontierMaxOwners
	}
	if options.MaxBoundaryCandidateOwners == 0 {
		options.MaxBoundaryCandidateOwners = options.MaxOwners
	}
	if options.MaxBoundaryCandidateOwners == 0 {
		options.MaxBoundaryCandidateOwners = defaultBoundaryFrontierMaxOwners
	}
	if options.MaxDepth == 0 {
		options.MaxDepth = defaultBoundaryFrontierDepth
	}
	if options.MaxDuration == 0 {
		options.MaxDuration = defaultBoundaryFrontierMaxDuration
	}
	return options
}

func boundaryDiscoveryStatsForSeeds(mode BoundaryDiscoveryMode, seeds *FunctionIndexSeedSet) BoundaryDiscoveryStats {
	seedStats := seeds.Stats()
	return BoundaryDiscoveryStats{
		Mode:                  string(mode.OrDefault()),
		BoundaryEvidence:      seedStats.BoundaryEvidenceCount,
		BoundarySeedOwners:    seedStats.BoundarySeedOwners,
		BoundaryEvidenceCount: seedStats.BoundaryEvidenceCount,
		SeedSetOwners:         seedStats.OwnerCount,
	}
}

type boundaryFrontierResult struct {
	ReverseOwners           []*ssa.Function
	AdjacentOwners          []*ssa.Function
	CandidateOwners         []*ssa.Function
	BoundaryCandidateOwners []*ssa.Function
	BoundarySeeds           *FunctionIndexSeedSet
	Stats                   BoundaryDiscoveryStats
}

func discoverBoundaryFrontier(graph *callgraph.Graph, regionRoots []*ssa.Function, options BoundaryFrontierOptions, phases *phaseRecorder) boundaryFrontierResult {
	options = options.WithDefaults()
	collector := newBoundaryFrontierCollector(options)
	predicates := defaultBoundaryPredicates()
	seeds := NewFunctionIndexSeedSet()
	result := boundaryFrontierResult{
		BoundarySeeds: seeds,
		Stats: BoundaryDiscoveryStats{
			Mode: string(BoundaryDiscoveryModeFrontier),
		},
	}

	reverseStarted := phases.start("boundary_reverse_frontier")
	result.ReverseOwners = collector.collectReverseFrontier(graph, regionRoots, predicates, seeds)
	phases.finish("boundary_reverse_frontier", reverseStarted)

	adjacentStarted := phases.start("boundary_adjacent_expansion")
	result.CandidateOwners = collector.expandAdjacent(graph, result.ReverseOwners, predicates, seeds)
	phases.finish("boundary_adjacent_expansion", adjacentStarted)

	scanStarted := phases.start("boundary_predicate_scan")
	result.BoundarySeeds = finishBoundaryPredicateScan(seeds, collector)
	phases.finish("boundary_predicate_scan", scanStarted)

	result.Stats = collector.stats(result.ReverseOwners, result.CandidateOwners, result.BoundarySeeds)
	result.AdjacentOwners = collector.sortedAdjacentOwners()
	result.BoundaryCandidateOwners = collector.sortedBoundaryCandidateOwners()

	return result
}

type boundaryFrontierCollector struct {
	options                 BoundaryFrontierOptions
	started                 time.Time
	owners                  map[*ssa.Function]bool
	reverseOwners           map[*ssa.Function]bool
	adjacentOwners          map[*ssa.Function]bool
	boundaryCandidateOwners map[*ssa.Function]bool
	packages                map[string]bool
	stopReasons             map[string]bool
	budgetStops             map[string]BudgetStopReason
	diagnostics             []Diagnostic
}

func newBoundaryFrontierCollector(options BoundaryFrontierOptions) *boundaryFrontierCollector {
	return &boundaryFrontierCollector{
		options:                 options,
		started:                 time.Now(),
		owners:                  map[*ssa.Function]bool{},
		reverseOwners:           map[*ssa.Function]bool{},
		adjacentOwners:          map[*ssa.Function]bool{},
		boundaryCandidateOwners: map[*ssa.Function]bool{},
		packages:                map[string]bool{},
		stopReasons:             map[string]bool{},
		budgetStops:             map[string]BudgetStopReason{},
	}
}

func (collector *boundaryFrontierCollector) collectReverseFrontier(graph *callgraph.Graph, roots []*ssa.Function, predicates []BoundaryPredicate, seeds *FunctionIndexSeedSet) []*ssa.Function {
	queue := sortedUniqueFunctions(roots)
	visitedNodes := map[*callgraph.Node]bool{}
	for _, root := range queue {
		if !collector.addReverseOwner(root) || collector.durationExceeded() {
			return collector.sortedOwners()
		}
		collector.scanBoundaryOwner(root, predicates, seeds)
		if graph != nil && graph.Nodes[root] != nil {
			visitedNodes[graph.Nodes[root]] = true
		}
	}
	nodeQueue := make([]*callgraph.Node, 0, len(queue))
	if graph != nil {
		for _, root := range queue {
			if node := graph.Nodes[root]; node != nil {
				nodeQueue = append(nodeQueue, node)
			}
		}
	}
	for len(nodeQueue) > 0 {
		if collector.durationExceeded() || collector.reverseOwnerBudgetExceeded() {
			break
		}
		node := nodeQueue[0]
		nodeQueue = nodeQueue[1:]
		for _, caller := range sortedCallerNodes(node) {
			if collector.durationExceeded() || collector.reverseOwnerBudgetExceeded() {
				return collector.sortedOwners()
			}
			if caller == nil || caller.Func == nil {
				continue
			}
			collector.addReverseOwner(caller.Func)
			collector.scanBoundaryOwner(caller.Func, predicates, seeds)
			if visitedNodes[caller] {
				continue
			}
			visitedNodes[caller] = true
			nodeQueue = append(nodeQueue, caller)
		}
	}
	return collector.sortedOwners()
}

func (collector *boundaryFrontierCollector) expandAdjacent(graph *callgraph.Graph, reverseOwners []*ssa.Function, predicates []BoundaryPredicate, seeds *FunctionIndexSeedSet) []*ssa.Function {
	if graph == nil || collector.options.MaxDepth <= 0 {
		if graph != nil && hasUnseenAdjacent(graph, reverseOwners, collector.owners) {
			collector.addStop("depth", "depth_budget", "boundary frontier depth budget reached")
		}
		return collector.sortedOwners()
	}
	type item struct {
		owner *ssa.Function
		depth int
	}
	queue := make([]item, 0, len(reverseOwners))
	for _, owner := range sortedUniqueFunctions(reverseOwners) {
		queue = append(queue, item{owner: owner})
	}
	expanded := map[*ssa.Function]bool{}
	for len(queue) > 0 {
		if collector.durationExceeded() || collector.adjacentOwnerBudgetExceeded() {
			break
		}
		current := queue[0]
		queue = queue[1:]
		if current.owner == nil || expanded[current.owner] {
			continue
		}
		expanded[current.owner] = true
		neighbors := sortedAdjacentFunctions(graph, current.owner)
		if current.depth >= collector.options.MaxDepth {
			if hasUnseenFunction(neighbors, collector.owners) {
				collector.addStop("depth", "depth_budget", "boundary frontier depth budget reached")
			}
			continue
		}
		for _, neighbor := range neighbors {
			if collector.durationExceeded() || collector.adjacentOwnerBudgetExceeded() {
				return collector.sortedOwners()
			}
			if neighbor == nil {
				continue
			}
			before := len(collector.owners)
			if !collector.addAdjacentOwner(neighbor) {
				continue
			}
			collector.scanBoundaryOwner(neighbor, predicates, seeds)
			if len(collector.owners) > before {
				queue = append(queue, item{owner: neighbor, depth: current.depth + 1})
			}
		}
	}
	return collector.sortedOwners()
}

func finishBoundaryPredicateScan(seeds *FunctionIndexSeedSet, collector *boundaryFrontierCollector) *FunctionIndexSeedSet {
	seeds.Diagnostics = append(seeds.Diagnostics, collector.diagnostics...)
	return seeds
}

func seedSetFromOwners(owners []*ssa.Function, reason string) *FunctionIndexSeedSet {
	seeds := NewFunctionIndexSeedSet()
	for _, owner := range sortedUniqueFunctions(owners) {
		seeds.Add(owner, reason)
	}
	return seeds
}

func (collector *boundaryFrontierCollector) addOwner(owner *ssa.Function) bool {
	if owner == nil {
		return false
	}
	if collector.owners[owner] {
		return true
	}
	if collector.durationExceeded() {
		return false
	}
	pkg := functionPackagePath(owner)
	if collector.options.MaxPackages > 0 && pkg != "" && !collector.packages[pkg] && len(collector.packages) >= collector.options.MaxPackages {
		collector.addStop("package", "package_budget", "boundary frontier package budget reached")
		return false
	}
	collector.owners[owner] = true
	if pkg != "" {
		collector.packages[pkg] = true
	}
	return true
}

func (collector *boundaryFrontierCollector) addReverseOwner(owner *ssa.Function) bool {
	if owner == nil {
		return false
	}
	if collector.reverseOwners[owner] {
		return true
	}
	if collector.durationExceeded() || collector.reverseOwnerBudgetExceeded() {
		return false
	}
	if !collector.addOwner(owner) {
		return false
	}
	collector.reverseOwners[owner] = true
	return true
}

func (collector *boundaryFrontierCollector) addAdjacentOwner(owner *ssa.Function) bool {
	if owner == nil {
		return false
	}
	if collector.owners[owner] {
		return true
	}
	if collector.durationExceeded() || collector.adjacentOwnerBudgetExceeded() {
		return false
	}
	if !collector.addOwner(owner) {
		return false
	}
	collector.adjacentOwners[owner] = true
	return true
}

func (collector *boundaryFrontierCollector) scanBoundaryOwner(owner *ssa.Function, predicates []BoundaryPredicate, seeds *FunctionIndexSeedSet) bool {
	if owner == nil || seeds == nil {
		return false
	}
	if collector.boundaryCandidateOwners[owner] {
		return true
	}
	if collector.durationExceeded() || collector.boundaryCandidateBudgetExceeded() {
		return false
	}
	collector.boundaryCandidateOwners[owner] = true
	for _, predicate := range predicates {
		if predicate == nil {
			continue
		}
		for _, evidence := range predicate.MatchOwner(owner) {
			seeds.AddBoundaryEvidence(evidence)
		}
	}
	return true
}

func (collector *boundaryFrontierCollector) durationExceeded() bool {
	if collector.options.MaxDuration <= 0 || time.Since(collector.started) < collector.options.MaxDuration {
		return false
	}
	collector.addStop("duration", "duration_budget", "boundary frontier duration budget reached")
	return true
}

func (collector *boundaryFrontierCollector) reverseOwnerBudgetExceeded() bool {
	if collector.options.MaxReverseOwners <= 0 || len(collector.reverseOwners) < collector.options.MaxReverseOwners {
		return false
	}
	collector.addStop("reverse_owner", "reverse_owner_budget", "boundary frontier reverse owner budget reached")
	return true
}

func (collector *boundaryFrontierCollector) adjacentOwnerBudgetExceeded() bool {
	if collector.options.MaxAdjacentOwners <= 0 || len(collector.adjacentOwners) < collector.options.MaxAdjacentOwners {
		return false
	}
	collector.addStop("adjacent_owner", "adjacent_owner_budget", "boundary frontier adjacent owner budget reached")
	return true
}

func (collector *boundaryFrontierCollector) boundaryCandidateBudgetExceeded() bool {
	if collector.options.MaxBoundaryCandidateOwners <= 0 || len(collector.boundaryCandidateOwners) < collector.options.MaxBoundaryCandidateOwners {
		return false
	}
	collector.addStop("boundary_candidate", "boundary_candidate_budget", "boundary frontier candidate budget reached")
	return true
}

func (collector *boundaryFrontierCollector) addStop(budget, reason, diagnosticReason string) {
	if collector.stopReasons[reason] {
		return
	}
	collector.stopReasons[reason] = true
	collector.budgetStops[budget] = BudgetStopReason{Budget: budget, Reason: reason}
	collector.diagnostics = append(collector.diagnostics, Diagnostic{
		Kind:   "boundary_frontier_" + reason + "_exceeded",
		Reason: diagnosticReason,
	})
}

func (collector *boundaryFrontierCollector) sortedOwners() []*ssa.Function {
	owners := make([]*ssa.Function, 0, len(collector.owners))
	for owner := range collector.owners {
		owners = append(owners, owner)
	}
	sort.Slice(owners, func(i, j int) bool { return functionSortKey(owners[i]) < functionSortKey(owners[j]) })
	return owners
}

func (collector *boundaryFrontierCollector) sortedAdjacentOwners() []*ssa.Function {
	return sortedFunctionMapKeys(collector.adjacentOwners)
}

func (collector *boundaryFrontierCollector) sortedBoundaryCandidateOwners() []*ssa.Function {
	return sortedFunctionMapKeys(collector.boundaryCandidateOwners)
}

func sortedFunctionMapKeys(values map[*ssa.Function]bool) []*ssa.Function {
	out := make([]*ssa.Function, 0, len(values))
	for fn := range values {
		if fn != nil {
			out = append(out, fn)
		}
	}
	sort.Slice(out, func(i, j int) bool { return functionSortKey(out[i]) < functionSortKey(out[j]) })
	return out
}

func (collector *boundaryFrontierCollector) stats(reverseOwners, candidateOwners []*ssa.Function, seeds *FunctionIndexSeedSet) BoundaryDiscoveryStats {
	seedStats := seeds.Stats()
	stopReasons := make([]string, 0, len(collector.stopReasons))
	for reason := range collector.stopReasons {
		stopReasons = append(stopReasons, reason)
	}
	sort.Strings(stopReasons)
	budgetStops := collector.sortedBudgetStops()
	reverseCount := len(collector.reverseOwners)
	adjacentCount := len(collector.adjacentOwners)
	candidateCount := len(sortedUniqueFunctions(candidateOwners))
	boundaryCandidateCount := len(collector.boundaryCandidateOwners)
	return BoundaryDiscoveryStats{
		Mode:                    string(BoundaryDiscoveryModeFrontier),
		ReverseFrontierOwners:   reverseCount,
		ReverseOwners:           reverseCount,
		AdjacentExpansionOwners: adjacentCount,
		CandidateOwnerCount:     candidateCount,
		BoundaryCandidateOwners: boundaryCandidateCount,
		CandidatePackageCount:   len(collector.packages),
		BoundarySeedOwners:      seedStats.BoundarySeedOwners,
		BoundaryEvidenceCount:   seedStats.BoundaryEvidenceCount,
		BoundaryEvidence:        seedStats.BoundaryEvidenceCount,
		SeedSetOwners:           seedStats.OwnerCount,
		BudgetStops:             budgetStops,
		StopReasons:             stopReasons,
	}
}

func (collector *boundaryFrontierCollector) sortedBudgetStops() []BudgetStopReason {
	stops := make([]BudgetStopReason, 0, len(collector.budgetStops))
	for _, stop := range collector.budgetStops {
		stops = append(stops, stop)
	}
	sort.Slice(stops, func(i, j int) bool {
		if stops[i].Budget == stops[j].Budget {
			return stops[i].Reason < stops[j].Reason
		}
		return stops[i].Budget < stops[j].Budget
	})
	return stops
}

func sortedCallerNodes(node *callgraph.Node) []*callgraph.Node {
	if node == nil {
		return nil
	}
	nodes := make([]*callgraph.Node, 0, len(node.In))
	for _, edge := range node.In {
		if edge != nil && edge.Caller != nil && edge.Caller.Func != nil {
			nodes = append(nodes, edge.Caller)
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return functionSortKey(nodes[i].Func) < functionSortKey(nodes[j].Func) })
	return nodes
}

func sortedAdjacentFunctions(graph *callgraph.Graph, owner *ssa.Function) []*ssa.Function {
	if graph == nil || owner == nil {
		return nil
	}
	seen := map[*ssa.Function]bool{}
	add := func(fn *ssa.Function) {
		if fn != nil {
			seen[fn] = true
		}
	}
	if node := graph.Nodes[owner]; node != nil {
		for _, edge := range node.In {
			if edge != nil && edge.Caller != nil {
				add(edge.Caller.Func)
			}
		}
		for _, edge := range node.Out {
			if edge != nil && edge.Callee != nil {
				add(edge.Callee.Func)
			}
		}
	}
	for _, callee := range staticCalleesInFunction(owner) {
		add(callee)
	}
	delete(seen, owner)
	out := make([]*ssa.Function, 0, len(seen))
	for fn := range seen {
		out = append(out, fn)
	}
	return sortedUniqueFunctions(out)
}

func hasUnseenAdjacent(graph *callgraph.Graph, owners []*ssa.Function, seen map[*ssa.Function]bool) bool {
	for _, owner := range owners {
		if hasUnseenFunction(sortedAdjacentFunctions(graph, owner), seen) {
			return true
		}
	}
	return false
}

func hasUnseenFunction(functions []*ssa.Function, seen map[*ssa.Function]bool) bool {
	for _, fn := range functions {
		if fn != nil && !seen[fn] {
			return true
		}
	}
	return false
}
