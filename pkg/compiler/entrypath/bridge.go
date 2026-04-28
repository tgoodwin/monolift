package entrypath

import (
	"sort"
	"time"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const (
	defaultBridgeMaxStarts           = 1000
	defaultBridgeMaxPackages         = 64
	defaultBridgeMaxPackageFunctions = 2000
	defaultBridgeMaxOwners           = 2000
	defaultBridgeMaxBoundaryOwners   = 2000
	defaultBridgeMaxInstructions     = 250000
	defaultBridgeMaxDuration         = 30 * time.Second
)

type BridgeOptions struct {
	MaxStarts           int
	MaxPackages         int
	MaxPackageFunctions int
	MaxOwners           int
	MaxBoundaryOwners   int
	MaxInstructions     int
	MaxDuration         time.Duration
}

func bridgeOptionsFromProbe(options ProbeOptions) BridgeOptions {
	return BridgeOptions{
		MaxStarts:           options.BridgeMaxStarts,
		MaxPackages:         options.BridgeMaxPackages,
		MaxPackageFunctions: options.BridgeMaxPackageFunctions,
		MaxOwners:           options.BridgeMaxOwners,
		MaxBoundaryOwners:   options.BridgeMaxBoundaryOwners,
		MaxInstructions:     options.BridgeMaxInstructions,
		MaxDuration:         options.BridgeMaxDuration,
	}.WithDefaults()
}

func (options BridgeOptions) WithDefaults() BridgeOptions {
	if options.MaxStarts == 0 {
		options.MaxStarts = defaultBridgeMaxStarts
	}
	if options.MaxPackages == 0 {
		options.MaxPackages = defaultBridgeMaxPackages
	}
	if options.MaxPackageFunctions == 0 {
		options.MaxPackageFunctions = defaultBridgeMaxPackageFunctions
	}
	if options.MaxOwners == 0 {
		options.MaxOwners = defaultBridgeMaxOwners
	}
	if options.MaxBoundaryOwners == 0 {
		options.MaxBoundaryOwners = defaultBridgeMaxBoundaryOwners
	}
	if options.MaxInstructions == 0 {
		options.MaxInstructions = defaultBridgeMaxInstructions
	}
	if options.MaxDuration == 0 {
		options.MaxDuration = defaultBridgeMaxDuration
	}
	return options
}

type bridgeDiscoveryResult struct {
	Seeds         *FunctionIndexSeedSet
	Stats         BridgeDiscoveryStats
	IndexPriority bridgeIndexPriorityContext
}

type bridgeIndexPriorityContext struct {
	selectedTouchpointPackages map[string]bool
	directTouchpointRefs       map[*ssa.Function]int
	boundaryEvidenceCounts     map[*ssa.Function]int
}

func discoverBridgeSeeds(prog *ssa.Program, touchpointFunctions []*ssa.Function, options BridgeOptions) bridgeDiscoveryResult {
	return discoverBridgeSeedsWithOracleSpec(prog, touchpointFunctions, options, OracleSpec{})
}

func discoverBridgeSeedsWithOracleSpec(prog *ssa.Program, touchpointFunctions []*ssa.Function, options BridgeOptions, oracleSpec OracleSpec) bridgeDiscoveryResult {
	options = options.WithDefaults()
	collector := newBridgeCollector(options)
	seeds := NewFunctionIndexSeedSet()
	starts := collector.selectStarts(touchpointFunctions)
	for _, start := range starts {
		if !collector.addBridgeOwner(start, seeds) {
			break
		}
	}
	startsByPackage := functionsByPackage(starts)
	collector.preparePackageCoverage(startsByPackage)
	programFunctionsByPackage := collector.programFunctionsBySelectedPackage(prog, startsByPackage)
	predicates := defaultBoundaryPredicates()
	scheduledPackages := bridgeScheduledPackageKeys(startsByPackage)
	for _, pkg := range scheduledPackages {
		if !collector.startPackage(pkg) {
			break
		}
		localStarts := startValueSet(startsByPackage[pkg])
		scannedInPackage := 0
		completedPackage := true
		for _, owner := range programFunctionsByPackage[pkg] {
			if !collector.beforeOwnerScan(scannedInPackage) {
				completedPackage = false
				break
			}
			scannedInPackage++
			if !collector.scanOwner(owner, localStarts, predicates, seeds) {
				completedPackage = false
				break
			}
		}
		collector.finishPackage(pkg, completedPackage)
		if collector.stopped() {
			break
		}
	}
	collector.finalizeUnscheduledPackages(scheduledPackages)
	seeds.Diagnostics = append(seeds.Diagnostics, collector.diagnostics...)
	indexPriority := collector.indexPriorityContext(startsByPackage, seeds)
	stats := collector.stats(prog, startsByPackage, programFunctionsByPackage, seeds, oracleSpec)
	return bridgeDiscoveryResult{Seeds: seeds, Stats: stats, IndexPriority: indexPriority}
}

type bridgeCollector struct {
	options                    BridgeOptions
	started                    time.Time
	startCandidates            map[*ssa.Function]bool
	selectedStarts             map[*ssa.Function]bool
	bridgeOwners               map[*ssa.Function]bool
	boundaryCandidateOwners    map[*ssa.Function]bool
	boundaryOwners             map[*ssa.Function]bool
	scannedPackages            map[string]bool
	packageCoverage            map[string]*bridgePackageCoverageState
	currentPackage             string
	scannedOwners              map[*ssa.Function]bool
	refMatcherInspectedOwners  map[*ssa.Function]bool
	ownerRefMatches            map[*ssa.Function]int
	boundaryPredicateOwners    map[*ssa.Function]bool
	boundaryPredicateRejected  map[*ssa.Function]bool
	boundaryPredicateSkipped   map[*ssa.Function]string
	ownerDuplicateSuppressions map[*ssa.Function]int
	ownerPositions             map[*ssa.Function]int
	skipReasons                map[string]int
	stopReasons                map[string]bool
	budgetStops                map[string]BudgetStopReason
	diagnostics                []Diagnostic
	touchpointCount            int
	scannedPackageFunctions    int
	scannedInstructions        int
	duplicateOwnerSuppressions int
}

func newBridgeCollector(options BridgeOptions) *bridgeCollector {
	return &bridgeCollector{
		options:                    options,
		started:                    time.Now(),
		startCandidates:            map[*ssa.Function]bool{},
		selectedStarts:             map[*ssa.Function]bool{},
		bridgeOwners:               map[*ssa.Function]bool{},
		boundaryCandidateOwners:    map[*ssa.Function]bool{},
		boundaryOwners:             map[*ssa.Function]bool{},
		scannedPackages:            map[string]bool{},
		packageCoverage:            map[string]*bridgePackageCoverageState{},
		scannedOwners:              map[*ssa.Function]bool{},
		refMatcherInspectedOwners:  map[*ssa.Function]bool{},
		ownerRefMatches:            map[*ssa.Function]int{},
		boundaryPredicateOwners:    map[*ssa.Function]bool{},
		boundaryPredicateRejected:  map[*ssa.Function]bool{},
		boundaryPredicateSkipped:   map[*ssa.Function]string{},
		ownerDuplicateSuppressions: map[*ssa.Function]int{},
		ownerPositions:             map[*ssa.Function]int{},
		skipReasons:                map[string]int{},
		stopReasons:                map[string]bool{},
		budgetStops:                map[string]BudgetStopReason{},
	}
}

type bridgePackageCoverageState struct {
	PackagePath            string
	SelectedStartCount     int
	Scheduled              bool
	Scanned                bool
	Completed              bool
	ScannedFunctionCount   int
	InstructionCount       int
	BridgeOwnersAdmitted   int
	BoundaryOwnersAdmitted int
	StopReasons            map[string]bool
	SkipCauses             map[string]bool
}

func (collector *bridgeCollector) selectStarts(touchpointFunctions []*ssa.Function) []*ssa.Function {
	collector.touchpointCount = len(touchpointFunctions)
	for _, fn := range touchpointFunctions {
		if fn == nil {
			collector.skipStart("unresolved")
			continue
		}
		if functionPackagePath(fn) == "" {
			collector.skipStart("missing_package")
			continue
		}
		if functionObjectName(fn) == "" {
			collector.skipStart("missing_object")
			continue
		}
		if collector.startCandidates[fn] {
			collector.skipStart("duplicate")
			continue
		}
		collector.startCandidates[fn] = true
	}
	candidates := sortedBridgeStartCandidates(collector.startCandidates)
	for _, candidate := range candidates {
		if collector.options.MaxStarts > 0 && len(collector.selectedStarts) >= collector.options.MaxStarts {
			collector.skipStart("start_budget")
			collector.addStop("start", "start_budget", "bridge start budget reached")
			continue
		}
		collector.selectedStarts[candidate] = true
	}
	return sortedBridgeStartCandidates(collector.selectedStarts)
}

func (collector *bridgeCollector) skipStart(reason string) {
	if reason == "" {
		return
	}
	collector.skipReasons[reason]++
}

func sortedBridgeStartCandidates(values map[*ssa.Function]bool) []*ssa.Function {
	out := sortedFunctionMapKeys(values)
	sort.Slice(out, func(i, j int) bool {
		return bridgeStartSortKey(out[i]) < bridgeStartSortKey(out[j])
	})
	return out
}

func bridgeStartSortKey(fn *ssa.Function) string {
	receiverRank := "0"
	if fn == nil || fn.Signature == nil || fn.Signature.Recv() != nil {
		receiverRank = "1"
	}
	return receiverRank + "|" + functionPackagePath(fn) + "|" + functionObjectName(fn) + "|" + functionString(fn)
}

func functionsByPackage(functions []*ssa.Function) map[string][]*ssa.Function {
	byPackage := map[string][]*ssa.Function{}
	for _, fn := range sortedUniqueFunctions(functions) {
		pkg := functionPackagePath(fn)
		if pkg == "" {
			continue
		}
		byPackage[pkg] = append(byPackage[pkg], fn)
	}
	return byPackage
}

func sortedPackageKeys(values map[string][]*ssa.Function) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func bridgeScheduledPackageKeys(startsByPackage map[string][]*ssa.Function) []string {
	keys := sortedPackageKeys(startsByPackage)
	sort.SliceStable(keys, func(i, j int) bool {
		left := len(sortedUniqueFunctions(startsByPackage[keys[i]]))
		right := len(sortedUniqueFunctions(startsByPackage[keys[j]]))
		if left == right {
			return keys[i] < keys[j]
		}
		return left > right
	})
	return keys
}

func (collector *bridgeCollector) preparePackageCoverage(startsByPackage map[string][]*ssa.Function) {
	for pkg, starts := range startsByPackage {
		state := collector.packageState(pkg)
		state.SelectedStartCount = len(sortedUniqueFunctions(starts))
	}
}

func (collector *bridgeCollector) packageState(pkg string) *bridgePackageCoverageState {
	if pkg == "" {
		return nil
	}
	state := collector.packageCoverage[pkg]
	if state != nil {
		return state
	}
	state = &bridgePackageCoverageState{
		PackagePath: pkg,
		StopReasons: map[string]bool{},
		SkipCauses:  map[string]bool{},
	}
	collector.packageCoverage[pkg] = state
	return state
}

func (state *bridgePackageCoverageState) addStop(reason string) {
	if state == nil || reason == "" {
		return
	}
	if state.StopReasons == nil {
		state.StopReasons = map[string]bool{}
	}
	state.StopReasons[reason] = true
	state.addSkipCause(reason)
}

func (state *bridgePackageCoverageState) addSkipCause(reason string) {
	if state == nil || reason == "" {
		return
	}
	if state.SkipCauses == nil {
		state.SkipCauses = map[string]bool{}
	}
	state.SkipCauses[reason] = true
}

func startValueSet(functions []*ssa.Function) map[ssa.Value]bool {
	values := map[ssa.Value]bool{}
	for _, fn := range functions {
		if fn != nil {
			values[fn] = true
		}
	}
	return values
}

func (collector *bridgeCollector) programFunctionsBySelectedPackage(prog *ssa.Program, startsByPackage map[string][]*ssa.Function) map[string][]*ssa.Function {
	selectedPackages := map[string]bool{}
	for pkg := range startsByPackage {
		selectedPackages[pkg] = true
	}
	out := map[string][]*ssa.Function{}
	if prog == nil {
		return out
	}
	for fn := range ssautil.AllFunctions(prog) {
		pkg := functionPackagePath(fn)
		if pkg == "" || !selectedPackages[pkg] {
			continue
		}
		out[pkg] = append(out[pkg], fn)
	}
	for pkg := range out {
		sort.Slice(out[pkg], func(i, j int) bool { return functionSortKey(out[pkg][i]) < functionSortKey(out[pkg][j]) })
		for i, fn := range out[pkg] {
			collector.ownerPositions[fn] = i
		}
	}
	return out
}

func (collector *bridgeCollector) startPackage(pkg string) bool {
	if pkg == "" {
		return false
	}
	collector.currentPackage = pkg
	state := collector.packageState(pkg)
	if state != nil {
		state.Scheduled = true
	}
	if collector.durationExceeded() {
		return false
	}
	if collector.scannedPackages[pkg] {
		return true
	}
	if collector.options.MaxPackages > 0 && len(collector.scannedPackages) >= collector.options.MaxPackages {
		collector.addStop("package", "package_budget", "bridge package budget reached")
		return false
	}
	collector.scannedPackages[pkg] = true
	if state != nil {
		state.Scanned = true
	}
	return true
}

func (collector *bridgeCollector) beforeOwnerScan(scannedInPackage int) bool {
	if collector.durationExceeded() || collector.ownerBudgetExceeded() || collector.instructionBudgetExceeded() {
		return false
	}
	if collector.options.MaxPackageFunctions > 0 && scannedInPackage >= collector.options.MaxPackageFunctions {
		collector.addStop("package_function", "package_function_budget", "bridge package function budget reached")
		return false
	}
	collector.scannedPackageFunctions++
	if state := collector.packageState(collector.currentPackage); state != nil {
		state.ScannedFunctionCount++
	}
	return true
}

func (collector *bridgeCollector) scanOwner(owner *ssa.Function, starts map[ssa.Value]bool, predicates []BoundaryPredicate, seeds *FunctionIndexSeedSet) bool {
	if owner == nil {
		return true
	}
	collector.scannedOwners[owner] = true
	collector.refMatcherInspectedOwners[owner] = true
	collector.scanBoundaryOwner(owner, predicates, seeds)
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			if collector.durationExceeded() || collector.instructionBudgetExceeded() || collector.ownerBudgetExceeded() {
				return false
			}
			collector.scannedInstructions++
			if state := collector.packageState(collector.currentPackage); state != nil {
				state.InstructionCount++
			}
			for _, ref := range refsForInstruction(owner, instr) {
				if !starts[ref.Operand] {
					continue
				}
				collector.ownerRefMatches[owner]++
				if !collector.addBridgeOwner(owner, seeds) {
					return false
				}
				for _, callee := range bridgeCalleesForRef(ref) {
					if !collector.addBridgeOwner(callee, seeds) {
						return false
					}
					collector.scanBoundaryOwner(callee, predicates, seeds)
				}
			}
		}
	}
	return true
}

func bridgeCalleesForRef(ref FunctionRef) []*ssa.Function {
	if ref.Kind != "call_arg" && ref.Kind != "go_arg" {
		return nil
	}
	call, ok := ref.Instruction.(ssa.CallInstruction)
	if !ok || call.Common() == nil {
		return nil
	}
	callee := call.Common().StaticCallee()
	if callee == nil {
		return nil
	}
	return []*ssa.Function{callee}
}

func (collector *bridgeCollector) scanBoundaryOwner(owner *ssa.Function, predicates []BoundaryPredicate, seeds *FunctionIndexSeedSet) bool {
	if owner == nil || seeds == nil {
		return false
	}
	if collector.boundaryCandidateOwners[owner] {
		return true
	}
	if collector.boundaryOwnerBudgetExceeded() {
		collector.boundaryPredicateSkipped[owner] = "boundary_owner_budget"
		return false
	}
	collector.boundaryCandidateOwners[owner] = true
	collector.boundaryPredicateOwners[owner] = true
	var evidence []BoundaryPredicateEvidence
	for _, predicate := range predicates {
		if predicate == nil {
			continue
		}
		evidence = append(evidence, predicate.MatchOwner(owner)...)
	}
	if len(evidence) == 0 {
		collector.boundaryPredicateRejected[owner] = true
		return true
	}
	if !collector.addBridgeOwner(owner, seeds) {
		return false
	}
	if !collector.boundaryOwners[owner] {
		collector.boundaryOwners[owner] = true
		if state := collector.packageState(collector.currentPackage); state != nil {
			state.BoundaryOwnersAdmitted++
		}
	}
	for _, item := range evidence {
		seeds.AddBoundaryEvidence(item)
	}
	return true
}

func (collector *bridgeCollector) addBridgeOwner(owner *ssa.Function, seeds *FunctionIndexSeedSet) bool {
	if owner == nil || seeds == nil {
		return false
	}
	if collector.bridgeOwners[owner] {
		collector.duplicateOwnerSuppressions++
		collector.ownerDuplicateSuppressions[owner]++
		if state := collector.packageState(collector.currentPackage); state != nil {
			state.addSkipCause("duplicate_suppression")
		}
		return true
	}
	if collector.ownerBudgetExceeded() {
		return false
	}
	collector.bridgeOwners[owner] = true
	admittedPackage := collector.currentPackage
	if admittedPackage == "" {
		admittedPackage = functionPackagePath(owner)
	}
	if state := collector.packageState(admittedPackage); state != nil {
		state.BridgeOwnersAdmitted++
	}
	seeds.Add(owner, FunctionIndexSeedReasonBridge)
	return true
}

func (collector *bridgeCollector) durationExceeded() bool {
	if collector.options.MaxDuration <= 0 || time.Since(collector.started) < collector.options.MaxDuration {
		return false
	}
	collector.addStop("duration", "duration_budget", "bridge duration budget reached")
	return true
}

func (collector *bridgeCollector) ownerBudgetExceeded() bool {
	if collector.options.MaxOwners <= 0 || len(collector.bridgeOwners) < collector.options.MaxOwners {
		return false
	}
	collector.addStop("owner", "owner_budget", "bridge owner budget reached")
	return true
}

func (collector *bridgeCollector) boundaryOwnerBudgetExceeded() bool {
	if collector.options.MaxBoundaryOwners <= 0 || len(collector.boundaryCandidateOwners) < collector.options.MaxBoundaryOwners {
		return false
	}
	collector.addStop("boundary_owner", "boundary_owner_budget", "bridge boundary owner budget reached")
	return true
}

func (collector *bridgeCollector) instructionBudgetExceeded() bool {
	if collector.options.MaxInstructions <= 0 || collector.scannedInstructions < collector.options.MaxInstructions {
		return false
	}
	collector.addStop("instruction", "instruction_budget", "bridge instruction budget reached")
	return true
}

func (collector *bridgeCollector) addStop(budget, reason, diagnosticReason string) {
	if collector.stopReasons[reason] {
		return
	}
	collector.stopReasons[reason] = true
	collector.budgetStops[budget] = BudgetStopReason{Budget: budget, Reason: reason}
	if state := collector.packageState(collector.currentPackage); state != nil {
		state.addStop(reason)
	}
	collector.diagnostics = append(collector.diagnostics, Diagnostic{
		Kind:   "bridge_" + reason + "_exceeded",
		Reason: diagnosticReason,
	})
}

func (collector *bridgeCollector) finishPackage(pkg string, completed bool) {
	if state := collector.packageState(pkg); state != nil {
		state.Completed = completed
	}
	collector.currentPackage = ""
}

func (collector *bridgeCollector) finalizeUnscheduledPackages(packages []string) {
	reasons := collector.sortedStopReasons()
	if len(reasons) == 0 {
		return
	}
	for _, pkg := range packages {
		state := collector.packageState(pkg)
		if state == nil || state.Scheduled {
			continue
		}
		for _, reason := range reasons {
			state.addStop(reason)
		}
	}
}

func (collector *bridgeCollector) stopped() bool {
	return collector.stopReasons["duration_budget"] ||
		collector.stopReasons["owner_budget"] ||
		collector.stopReasons["instruction_budget"] ||
		collector.stopReasons["package_budget"]
}

func (collector *bridgeCollector) stats(prog *ssa.Program, startsByPackage map[string][]*ssa.Function, programFunctionsByPackage map[string][]*ssa.Function, seeds *FunctionIndexSeedSet, oracleSpec OracleSpec) BridgeDiscoveryStats {
	startPackages := map[string]bool{}
	for start := range collector.selectedStarts {
		if pkg := functionPackagePath(start); pkg != "" {
			startPackages[pkg] = true
		}
	}
	return BridgeDiscoveryStats{
		TouchpointCount:              collector.touchpointCount,
		StartCandidateCount:          len(collector.startCandidates),
		SelectedStartCount:           len(collector.selectedStarts),
		SkippedStartCount:            collector.skippedStartCount(),
		SkipReasons:                  collector.sortedSkipReasons(),
		StartPackageCount:            len(startPackages),
		ScannedPackageCount:          len(collector.scannedPackages),
		ScannedPackageFunctions:      collector.scannedPackageFunctions,
		ScannedInstructions:          collector.scannedInstructions,
		BridgeOwnerCount:             len(collector.bridgeOwners),
		BridgeBoundaryCandidateCount: len(collector.boundaryCandidateOwners),
		BridgeBoundaryOwnerCount:     len(collector.boundaryOwners),
		BridgeSeedOwnerCount:         seeds.Len(),
		DuplicateOwnerSuppressions:   collector.duplicateOwnerSuppressions,
		BudgetStops:                  collector.sortedBudgetStops(),
		StopReasons:                  collector.sortedStopReasons(),
		Coverage:                     collector.coverage(prog, startsByPackage, programFunctionsByPackage, seeds, oracleSpec),
	}
}

func (collector *bridgeCollector) indexPriorityContext(startsByPackage map[string][]*ssa.Function, seeds *FunctionIndexSeedSet) bridgeIndexPriorityContext {
	context := bridgeIndexPriorityContext{
		selectedTouchpointPackages: map[string]bool{},
		directTouchpointRefs:       map[*ssa.Function]int{},
		boundaryEvidenceCounts:     map[*ssa.Function]int{},
	}
	for pkg := range startsByPackage {
		if pkg != "" {
			context.selectedTouchpointPackages[pkg] = true
		}
	}
	for owner, count := range collector.ownerRefMatches {
		if owner != nil && count > 0 {
			context.directTouchpointRefs[owner] = count
		}
	}
	if seeds != nil {
		for _, item := range seeds.BoundaryEvidence {
			if item.Owner != nil {
				context.boundaryEvidenceCounts[item.Owner]++
			}
		}
	}
	return context
}

func buildBridgeFunctionRefIndex(seeds *FunctionIndexSeedSet, priority bridgeIndexPriorityContext, options FunctionRefIndexOptions) FunctionRefIndex {
	return buildFunctionRefIndexFromFunctions(bridgeIndexOwners(seeds, priority), options)
}

func bridgeIndexOwners(seeds *FunctionIndexSeedSet, priority bridgeIndexPriorityContext) []*ssa.Function {
	owners := seeds.Owners()
	sort.SliceStable(owners, func(i, j int) bool {
		return bridgeIndexOwnerLess(seeds, priority, owners[i], owners[j])
	})
	return owners
}

func bridgeIndexOwnerLess(seeds *FunctionIndexSeedSet, priority bridgeIndexPriorityContext, left, right *ssa.Function) bool {
	leftInputs := bridgeIndexPriorityInputs(seeds, priority, left)
	rightInputs := bridgeIndexPriorityInputs(seeds, priority, right)
	leftClassRank := bridgeIndexPriorityClassRank(leftInputs)
	rightClassRank := bridgeIndexPriorityClassRank(rightInputs)
	if leftClassRank != rightClassRank {
		return leftClassRank < rightClassRank
	}
	if leftInputs.BoundaryEvidenceCount != rightInputs.BoundaryEvidenceCount {
		return leftInputs.BoundaryEvidenceCount > rightInputs.BoundaryEvidenceCount
	}
	if leftInputs.DirectTouchpointRefs != rightInputs.DirectTouchpointRefs {
		return leftInputs.DirectTouchpointRefs > rightInputs.DirectTouchpointRefs
	}
	if leftInputs.SelectedTouchpointPackage != rightInputs.SelectedTouchpointPackage {
		return leftInputs.SelectedTouchpointPackage
	}
	if functionPackagePath(left) != functionPackagePath(right) {
		return functionPackagePath(left) < functionPackagePath(right)
	}
	if functionObjectName(left) != functionObjectName(right) {
		return functionObjectName(left) < functionObjectName(right)
	}
	if functionString(left) != functionString(right) {
		return functionString(left) < functionString(right)
	}
	return seedReasonsSortKey(leftInputs.SeedReasons) < seedReasonsSortKey(rightInputs.SeedReasons)
}

func bridgeIndexPriorityInputs(seeds *FunctionIndexSeedSet, priority bridgeIndexPriorityContext, owner *ssa.Function) BridgeIndexPriorityInputs {
	reasons := seeds.Reasons(owner)
	inputs := BridgeIndexPriorityInputs{
		BridgeSeed:                hasSeedReason(seeds, owner, FunctionIndexSeedReasonBridge),
		BoundarySeed:              hasSeedReason(seeds, owner, FunctionIndexSeedReasonBoundary),
		BoundaryEvidenceCount:     priority.boundaryEvidenceCounts[owner],
		SelectedTouchpointPackage: priority.selectedTouchpointPackages[functionPackagePath(owner)],
		DirectTouchpointRefs:      priority.directTouchpointRefs[owner],
		SeedReasons:               reasons,
	}
	if inputs.BoundaryEvidenceCount > 0 {
		inputs.BoundarySeed = true
	}
	return inputs
}

func bridgeIndexPriorityClass(inputs BridgeIndexPriorityInputs) string {
	switch {
	case inputs.BridgeSeed && inputs.BoundarySeed:
		return "boundary_bridge"
	case inputs.BridgeSeed && inputs.SelectedTouchpointPackage && inputs.DirectTouchpointRefs > 0:
		return "touchpoint_ref_bridge"
	case inputs.BridgeSeed && inputs.SelectedTouchpointPackage:
		return "selected_package_bridge"
	case inputs.BridgeSeed:
		return "other_bridge"
	default:
		return "non_bridge"
	}
}

func bridgeIndexPriorityClassRank(inputs BridgeIndexPriorityInputs) int {
	switch bridgeIndexPriorityClass(inputs) {
	case "boundary_bridge":
		return 0
	case "touchpoint_ref_bridge":
		return 1
	case "selected_package_bridge":
		return 2
	case "other_bridge":
		return 3
	default:
		return 4
	}
}

func seedReasonsSortKey(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	key := reasons[0]
	for _, reason := range reasons[1:] {
		key += "\x00" + reason
	}
	return key
}

func (collector *bridgeCollector) coverage(prog *ssa.Program, startsByPackage map[string][]*ssa.Function, programFunctionsByPackage map[string][]*ssa.Function, seeds *FunctionIndexSeedSet, oracleSpec OracleSpec) BridgeCoverage {
	return BridgeCoverage{
		Starts:        collector.startCoverage(),
		Packages:      collector.packageCoverages(),
		OracleTargets: collector.oracleTargetCoverage(prog, oracleSpec, seeds),
		RefMatchAudit: collector.refMatchAudit(prog, oracleSpec, startsByPackage, programFunctionsByPackage, seeds),
	}
}

func (collector *bridgeCollector) startCoverage() []BridgeStartCoverage {
	starts := sortedBridgeStartCandidates(collector.selectedStarts)
	out := make([]BridgeStartCoverage, 0, len(starts))
	for _, start := range starts {
		pkg := functionPackagePath(start)
		state := collector.packageState(pkg)
		coverage := BridgeStartCoverage{
			Function:        functionString(start),
			PackagePath:     pkg,
			ObjectName:      functionObjectName(start),
			SelectedPackage: pkg,
		}
		if state != nil {
			coverage.Scheduled = state.Scheduled
			coverage.Scanned = state.Scanned
			coverage.Completed = state.Completed
			coverage.ScannedFunctionCount = state.ScannedFunctionCount
			coverage.InstructionCount = state.InstructionCount
			coverage.BridgeOwnersAdmitted = state.BridgeOwnersAdmitted
			coverage.BoundaryOwnersAdmitted = state.BoundaryOwnersAdmitted
			coverage.StopReasons = sortedBoolMapKeys(state.StopReasons)
			coverage.SkipCauses = sortedBoolMapKeys(state.SkipCauses)
		}
		out = append(out, coverage)
	}
	return out
}

func (collector *bridgeCollector) packageCoverages() []BridgePackageCoverage {
	keys := make([]string, 0, len(collector.packageCoverage))
	for pkg := range collector.packageCoverage {
		keys = append(keys, pkg)
	}
	sort.Strings(keys)
	out := make([]BridgePackageCoverage, 0, len(keys))
	for _, pkg := range keys {
		state := collector.packageCoverage[pkg]
		if state == nil {
			continue
		}
		out = append(out, BridgePackageCoverage{
			PackagePath:            state.PackagePath,
			SelectedStartCount:     state.SelectedStartCount,
			SelectedStarts:         bridgePackageStartStrings(state.PackagePath, collector.selectedStarts),
			Scheduled:              state.Scheduled,
			Scanned:                state.Scanned,
			Completed:              state.Completed,
			ScannedFunctionCount:   state.ScannedFunctionCount,
			InstructionCount:       state.InstructionCount,
			BridgeOwnersAdmitted:   state.BridgeOwnersAdmitted,
			BoundaryOwnersAdmitted: state.BoundaryOwnersAdmitted,
			StopReasons:            sortedBoolMapKeys(state.StopReasons),
			SkipCauses:             sortedBoolMapKeys(state.SkipCauses),
		})
	}
	return out
}

func bridgePackageStartStrings(pkg string, starts map[*ssa.Function]bool) []string {
	var out []string
	for start := range starts {
		if functionPackagePath(start) == pkg {
			out = append(out, functionString(start))
		}
	}
	sort.Strings(out)
	return out
}

func (collector *bridgeCollector) oracleTargetCoverage(prog *ssa.Program, spec OracleSpec, seeds *FunctionIndexSeedSet) []BridgeOracleTargetCoverage {
	builder := newOracleTraceBuilder(prog, spec)
	if builder == nil {
		return nil
	}
	out := make([]BridgeOracleTargetCoverage, 0, len(spec.Nodes))
	for _, node := range spec.Nodes {
		if node.ID == "" {
			continue
		}
		fn := builder.nodesByID[node.ID]
		pkg := node.PackagePath
		objectName := node.ObjectName
		if fn != nil {
			pkg = functionPackagePath(fn)
			objectName = functionObjectName(fn)
		}
		state := collector.packageCoverage[pkg]
		evidence := bridgeBoundaryEvidenceStringsFromSeeds(seeds, fn)
		coverage := BridgeOracleTargetCoverage{
			ID:                        node.ID,
			Function:                  functionString(fn),
			PackagePath:               pkg,
			ObjectName:                objectName,
			PackageSelected:           state != nil,
			OwnerScanned:              collector.scannedOwners[fn],
			RefMatcherInspected:       collector.refMatcherInspectedOwners[fn],
			ProducedBridgeSeed:        hasSeedReason(seeds, fn, FunctionIndexSeedReasonBridge),
			BoundaryPredicatesRan:     collector.boundaryPredicateOwners[fn],
			BoundaryPredicateRejected: collector.boundaryPredicateOwners[fn] && len(evidence) == 0,
			BoundaryEvidence:          evidence,
		}
		if state != nil {
			coverage.PackageScheduled = state.Scheduled
			coverage.PackageScanned = state.Scanned
			coverage.PackageCompleted = state.Completed
			coverage.StopReasons = sortedBoolMapKeys(state.StopReasons)
			coverage.ScanningStoppedBeforeOwner = collector.scanningStoppedBeforeOwner(fn, state)
		}
		coverage.SkipCauses = collector.oracleTargetSkipCauses(fn, state, seeds, evidence)
		out = append(out, coverage)
	}
	return out
}

func (collector *bridgeCollector) scanningStoppedBeforeOwner(fn *ssa.Function, state *bridgePackageCoverageState) bool {
	if fn == nil || state == nil || collector.scannedOwners[fn] {
		return false
	}
	if !state.Scheduled {
		return len(state.StopReasons) > 0
	}
	if !state.Scanned {
		return len(state.StopReasons) > 0
	}
	if state.Completed {
		return false
	}
	position, ok := collector.ownerPositions[fn]
	if !ok {
		return true
	}
	return position >= state.ScannedFunctionCount
}

func (collector *bridgeCollector) oracleTargetSkipCauses(fn *ssa.Function, state *bridgePackageCoverageState, seeds *FunctionIndexSeedSet, evidence []string) []string {
	causes := map[string]bool{}
	if state == nil {
		causes["package_not_selected"] = true
	} else {
		for reason := range state.SkipCauses {
			causes[reason] = true
		}
	}
	if fn == nil {
		causes["target_unresolved"] = true
		return sortedBoolMapKeys(causes)
	}
	if collector.ownerDuplicateSuppressions[fn] > 0 {
		causes["duplicate_suppression"] = true
	}
	if collector.refMatcherInspectedOwners[fn] && collector.ownerRefMatches[fn] == 0 {
		causes["no_ref_match"] = true
	}
	if skipped := collector.boundaryPredicateSkipped[fn]; skipped != "" {
		causes[skipped] = true
	}
	if collector.boundaryPredicateOwners[fn] && len(evidence) == 0 {
		causes["no_boundary_predicate_evidence"] = true
	}
	if !hasSeedReason(seeds, fn, FunctionIndexSeedReasonBridge) && collector.stopReasons["owner_budget"] {
		causes["owner_budget"] = true
	}
	return sortedBoolMapKeys(causes)
}

func (collector *bridgeCollector) refMatchAudit(prog *ssa.Program, spec OracleSpec, startsByPackage map[string][]*ssa.Function, programFunctionsByPackage map[string][]*ssa.Function, seeds *FunctionIndexSeedSet) []BridgeRefMatchAudit {
	builder := newOracleTraceBuilder(prog, spec)
	if builder == nil {
		return nil
	}
	out := make([]BridgeRefMatchAudit, 0, len(spec.Nodes))
	for _, node := range spec.Nodes {
		if node.ID == "" {
			continue
		}
		fn := builder.nodesByID[node.ID]
		audit := BridgeRefMatchAudit{
			ID:                  node.ID,
			Function:            functionString(fn),
			PackagePath:         node.PackagePath,
			ObjectName:          node.ObjectName,
			BridgeScanned:       collector.scannedOwners[fn],
			RefMatcherInspected: collector.refMatcherInspectedOwners[fn],
			SeedReasons:         seeds.Reasons(fn),
			ProducedBridgeSeed:  hasSeedReason(seeds, fn, FunctionIndexSeedReasonBridge),
		}
		if fn != nil {
			audit.Audited = true
			audit.PackagePath = functionPackagePath(fn)
			audit.ObjectName = functionObjectName(fn)
			audit.BoundaryEvidence = bridgeBoundaryEvidenceStrings(defaultBoundaryPredicates(), fn)
			collector.populateRefMatchAudit(prog, fn, startsByPackage, &audit)
		}
		switch {
		case audit.ProducedBridgeSeed:
			audit.SeedResult = "bridge_seed"
		case len(audit.SeedReasons) > 0:
			audit.SeedResult = "seeded_without_bridge_reason"
		default:
			audit.SeedResult = "not_seeded"
		}
		_ = programFunctionsByPackage
		out = append(out, audit)
	}
	return out
}

func (collector *bridgeCollector) populateRefMatchAudit(prog *ssa.Program, owner *ssa.Function, startsByPackage map[string][]*ssa.Function, audit *BridgeRefMatchAudit) {
	if owner == nil || audit == nil {
		return
	}
	starts := startValueSet(startsByPackage[functionPackagePath(owner)])
	if len(starts) == 0 {
		return
	}
	staticCallees := map[string]bool{}
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			for _, ref := range refsForInstruction(owner, instr) {
				if !starts[ref.Operand] {
					continue
				}
				audit.Counts.DirectTouchpointRefs++
				switch ref.Kind {
				case "call_arg", "go_arg":
					audit.Counts.CallArgs++
				case "store":
					audit.Counts.Stores++
				case "return":
					audit.Counts.Returns++
				case "capture":
					audit.Counts.Closures++
				case "direct_invoke", "go_launch":
					audit.Counts.DirectInvokes++
				}
				item := BridgeRefMatchRef{
					Kind:        ref.Kind,
					Touchpoint:  bridgeValueString(ref.Operand),
					Instruction: bridgeInstructionString(ref.Instruction),
				}
				if ref.Instruction != nil {
					item.Position = sourcePosition(prog, ref.Instruction.Pos())
				}
				for _, callee := range bridgeCalleesForRef(ref) {
					calleeString := functionString(callee)
					if calleeString == "" {
						continue
					}
					item.StaticCallee = calleeString
					staticCallees[calleeString] = true
				}
				audit.Refs = append(audit.Refs, item)
			}
		}
	}
	audit.StaticCalleesReceivingTouchpoint = sortedBoolMapKeys(staticCallees)
}

func bridgeBoundaryEvidenceStrings(predicates []BoundaryPredicate, owner *ssa.Function) []string {
	var out []string
	for _, predicate := range predicates {
		if predicate == nil {
			continue
		}
		for _, item := range predicate.MatchOwner(owner) {
			out = append(out, boundaryEvidenceString(item))
		}
	}
	sort.Strings(out)
	return out
}

func bridgeBoundaryEvidenceStringsFromSeeds(seeds *FunctionIndexSeedSet, owner *ssa.Function) []string {
	if seeds == nil || owner == nil {
		return nil
	}
	var out []string
	for _, item := range seeds.BoundaryEvidence {
		if item.Owner == owner {
			out = append(out, boundaryEvidenceString(item))
		}
	}
	sort.Strings(out)
	return out
}

func bridgeValueString(value ssa.Value) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func bridgeInstructionString(instr ssa.Instruction) string {
	if instr == nil {
		return ""
	}
	return instr.String()
}

func sortedBoolMapKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		if key != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func (collector *bridgeCollector) skippedStartCount() int {
	count := 0
	for _, value := range collector.skipReasons {
		count += value
	}
	return count
}

func (collector *bridgeCollector) sortedSkipReasons() []BridgeSkipReason {
	reasons := make([]BridgeSkipReason, 0, len(collector.skipReasons))
	for reason, count := range collector.skipReasons {
		reasons = append(reasons, BridgeSkipReason{Reason: reason, Count: count})
	}
	sort.Slice(reasons, func(i, j int) bool { return reasons[i].Reason < reasons[j].Reason })
	return reasons
}

func (collector *bridgeCollector) sortedBudgetStops() []BudgetStopReason {
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

func (collector *bridgeCollector) sortedStopReasons() []string {
	reasons := make([]string, 0, len(collector.stopReasons))
	for reason := range collector.stopReasons {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	return reasons
}

func finalizeBridgeDiscoveryStats(stats BridgeDiscoveryStats, seeds *FunctionIndexSeedSet, refIndex FunctionRefIndex, priority bridgeIndexPriorityContext) BridgeDiscoveryStats {
	if seeds == nil {
		return stats
	}
	owners := bridgeIndexOwnerDiagnostics(seeds, priority, refIndex)
	stats.Coverage.IndexOwners = owners
	stats.IndexPriorityClassCounts = bridgeIndexClassCounts(owners)
	stats.IndexSkipReasonCounts = bridgeIndexSkipReasonCounts(owners)
	stats.IndexedBridgeOwnerCount = 0
	for _, owner := range owners {
		if owner.Indexed {
			stats.IndexedBridgeOwnerCount++
		}
	}
	if diagnosticsContain(refIndex.Diagnostics, "function_ref_index_budget_exceeded") {
		stats = appendBridgeBudgetStop(stats, BudgetStopReason{Budget: "index", Reason: "index_budget"})
	}
	for i := range stats.Coverage.OracleTargets {
		target := &stats.Coverage.OracleTargets[i]
		target.FunctionRefIndexed = bridgeCoverageOwnerIndexed(target.PackagePath, target.ObjectName, refIndex)
	}
	return stats
}

func bridgeIndexOwnerDiagnostics(seeds *FunctionIndexSeedSet, priority bridgeIndexPriorityContext, refIndex FunctionRefIndex) []BridgeIndexOwnerDiagnostic {
	owners := bridgeIndexOwners(seeds, priority)
	out := make([]BridgeIndexOwnerDiagnostic, 0, len(owners))
	for _, owner := range owners {
		inputs := bridgeIndexPriorityInputs(seeds, priority, owner)
		if !inputs.BridgeSeed {
			continue
		}
		skip := refIndex.SkippedOwners[owner]
		diagnostic := BridgeIndexOwnerDiagnostic{
			Function:       functionString(owner),
			PackagePath:    functionPackagePath(owner),
			ObjectName:     functionObjectName(owner),
			SeedReasons:    inputs.SeedReasons,
			PriorityClass:  bridgeIndexPriorityClass(inputs),
			PriorityRank:   len(out) + 1,
			PriorityInputs: inputs,
			IndexOrder:     refIndex.OwnerOrder[owner],
			Indexed:        refIndex.ScannedOwners[owner],
		}
		if !diagnostic.Indexed {
			diagnostic.SkipReason = skip.Reason
			diagnostic.BudgetResponsible = skip.BudgetResponsible
			if diagnostic.SkipReason == "" {
				diagnostic.SkipReason = "not_indexed"
			}
		}
		out = append(out, diagnostic)
	}
	return out
}

func bridgeIndexClassCounts(owners []BridgeIndexOwnerDiagnostic) []BridgeIndexClassCount {
	byClass := map[string]*BridgeIndexClassCount{}
	for _, owner := range owners {
		class := owner.PriorityClass
		if class == "" {
			class = "unknown"
		}
		count := byClass[class]
		if count == nil {
			count = &BridgeIndexClassCount{PriorityClass: class}
			byClass[class] = count
		}
		count.Count++
		if owner.Indexed {
			count.Indexed++
		} else {
			count.Skipped++
		}
	}
	order := []string{"boundary_bridge", "touchpoint_ref_bridge", "selected_package_bridge", "other_bridge", "non_bridge", "unknown"}
	var out []BridgeIndexClassCount
	for _, class := range order {
		if count := byClass[class]; count != nil {
			out = append(out, *count)
			delete(byClass, class)
		}
	}
	var remaining []string
	for class := range byClass {
		remaining = append(remaining, class)
	}
	sort.Strings(remaining)
	for _, class := range remaining {
		out = append(out, *byClass[class])
	}
	return out
}

func bridgeIndexSkipReasonCounts(owners []BridgeIndexOwnerDiagnostic) []BridgeIndexSkipReasonCount {
	byReason := map[string]int{}
	for _, owner := range owners {
		if owner.Indexed {
			continue
		}
		reason := owner.SkipReason
		if reason == "" {
			reason = "not_indexed"
		}
		byReason[reason]++
	}
	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	out := make([]BridgeIndexSkipReasonCount, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, BridgeIndexSkipReasonCount{Reason: reason, Count: byReason[reason]})
	}
	return out
}

func bridgeCoverageOwnerIndexed(packagePath, objectName string, refIndex FunctionRefIndex) bool {
	if packagePath == "" && objectName == "" {
		return false
	}
	for owner := range refIndex.ScannedOwners {
		if owner == nil {
			continue
		}
		if packagePath != "" && functionPackagePath(owner) != packagePath {
			continue
		}
		if objectName != "" && functionObjectName(owner) != objectName {
			continue
		}
		return true
	}
	return false
}

func hasSeedReason(seeds *FunctionIndexSeedSet, owner *ssa.Function, reason string) bool {
	for _, existing := range seeds.Reasons(owner) {
		if existing == reason {
			return true
		}
	}
	return false
}

func appendBridgeBudgetStop(stats BridgeDiscoveryStats, stop BudgetStopReason) BridgeDiscoveryStats {
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
