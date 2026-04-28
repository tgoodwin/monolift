package entrypath

import (
	"go/token"
	"go/types"
	"sort"
	"time"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type FunctionRefIndex struct {
	Sources       map[ssa.Value]FunctionValue
	Uses          map[ssa.Value][]FunctionRef
	ScannedOwners map[*ssa.Function]bool
	OwnerOrder    map[*ssa.Function]int
	SkippedOwners map[*ssa.Function]FunctionRefIndexOwnerSkip
	Stats         FunctionRefIndexStats
	Diagnostics   []Diagnostic
}

type FunctionRefIndexOptions struct {
	ProgressInstructionInterval int
	PhaseObserver               func(PhaseEvent)
	Budget                      time.Duration
	MaxFunctions                int
}

type FunctionValue struct {
	Value    ssa.Value
	Function *ssa.Function
	Closure  *ssa.MakeClosure
}

type FunctionRef struct {
	Owner       *ssa.Function
	Instruction ssa.Instruction
	Operand     ssa.Value
	Kind        string
	ArgIndex    int
}

type FunctionRefIndexOwnerSkip struct {
	Reason            string
	BudgetResponsible string
}

type functionFlowResult struct {
	ExternalSurfaces  []ExternalSurface
	RegistrationSites []RegistrationSite
	WrapperChains     []WrapperChain
	Diagnostics       []Diagnostic
}

type taintedValue struct {
	Value  ssa.Value
	Source FunctionValue
	Chain  []WrapperLink
}

func BuildFunctionRefIndex(prog *ssa.Program) FunctionRefIndex {
	return BuildFunctionRefIndexWithOptions(prog, FunctionRefIndexOptions{})
}

func BuildFunctionRefIndexWithOptions(prog *ssa.Program, options FunctionRefIndexOptions) FunctionRefIndex {
	return buildFunctionRefIndexFromFunctions(sortedProgramFunctions(prog), options)
}

func BuildFunctionRefIndexForSeeds(seeds *FunctionIndexSeedSet, options FunctionRefIndexOptions) FunctionRefIndex {
	return buildFunctionRefIndexFromFunctions(seeds.Owners(), options)
}

func buildFunctionRefIndexFromFunctions(functions []*ssa.Function, options FunctionRefIndexOptions) FunctionRefIndex {
	started := time.Now()
	index := FunctionRefIndex{
		Sources:       map[ssa.Value]FunctionValue{},
		Uses:          map[ssa.Value][]FunctionRef{},
		ScannedOwners: map[*ssa.Function]bool{},
		OwnerOrder:    map[*ssa.Function]int{},
		SkippedOwners: map[*ssa.Function]FunctionRefIndexOwnerSkip{},
	}
	for i, fn := range functions {
		if fn != nil {
			index.OwnerOrder[fn] = i + 1
		}
	}
	index.observeRSS()
	limit := len(functions)
	if options.MaxFunctions > 0 && options.MaxFunctions < limit {
		limit = options.MaxFunctions
	}
	scannedPrefix := true
	for i := 0; i < limit; i++ {
		fn := functions[i]
		if !index.scanFunction(fn, started, options) {
			for j := i; j < len(functions); j++ {
				index.recordOwnerSkip(functions[j], "index_budget", "index")
			}
			index.Stats.SkippedFunctions += len(functions) - i - 1
			scannedPrefix = false
			break
		}
	}
	if scannedPrefix && limit < len(functions) {
		for j := limit; j < len(functions); j++ {
			index.recordOwnerSkip(functions[j], "max_functions", "max_functions")
		}
		index.Stats.SkippedFunctions += len(functions) - limit
	}
	index.sort(started, options)
	index.Stats.ElapsedMillis = time.Since(started).Milliseconds()
	index.observeRSS()
	return index
}

func (index *FunctionRefIndex) recordOwnerSkip(fn *ssa.Function, reason, budgetResponsible string) {
	if index == nil || fn == nil || index.ScannedOwners[fn] {
		return
	}
	if index.SkippedOwners == nil {
		index.SkippedOwners = map[*ssa.Function]FunctionRefIndexOwnerSkip{}
	}
	if _, exists := index.SkippedOwners[fn]; exists {
		return
	}
	index.SkippedOwners[fn] = FunctionRefIndexOwnerSkip{
		Reason:            reason,
		BudgetResponsible: budgetResponsible,
	}
}

func (index *FunctionRefIndex) scanFunction(fn *ssa.Function, started time.Time, options FunctionRefIndexOptions) bool {
	if fn == nil {
		index.Stats.SkippedFunctions++
		return true
	}
	if index.budgetExceeded(started, options) {
		index.Stats.SkippedFunctions++
		index.recordBudgetExceeded(fn)
		return false
	}
	index.Stats.ScannedFunctions++
	index.ScannedOwners[fn] = true
	if fn.Signature != nil {
		index.addSource(fn, FunctionValue{Value: fn, Function: fn})
	}
	for _, block := range fn.Blocks {
		index.Stats.ScannedBlocks++
		for _, instr := range block.Instrs {
			index.Stats.ScannedInstructions++
			if closure, ok := instr.(*ssa.MakeClosure); ok {
				index.addSource(closure, FunctionValue{
					Value:    closure,
					Function: closureFunction(closure),
					Closure:  closure,
				})
			}
			for _, ref := range refsForInstruction(fn, instr) {
				index.Uses[ref.Operand] = append(index.Uses[ref.Operand], ref)
				index.recordRef(ref)
				if fnValue, ok := functionValue(ref.Operand); ok {
					index.addSource(ref.Operand, fnValue)
				}
			}
			index.observeProgress(fn, started, options)
			if index.budgetExceeded(started, options) {
				index.recordBudgetExceeded(fn)
				return false
			}
		}
	}
	index.observeRSS()
	return true
}

func (index *FunctionRefIndex) addSource(value ssa.Value, source FunctionValue) {
	if value == nil {
		return
	}
	if _, exists := index.Sources[value]; exists {
		return
	}
	index.Sources[value] = source
	if source.Closure != nil {
		index.Stats.ClosureSources++
	} else if source.Function != nil {
		index.Stats.DiscoveredFunctionSources++
	}
}

func (index *FunctionRefIndex) recordRef(ref FunctionRef) {
	switch ref.Kind {
	case "operand":
		index.Stats.OperandRefs++
	case "call_arg", "go_arg":
		index.Stats.CallArgRefs++
	case "store":
		index.Stats.StoreRefs++
	case "return":
		index.Stats.ReturnRefs++
	}
}

func (index *FunctionRefIndex) observeRSS() {
	if rss := processMemoryBytes(); rss > index.Stats.PeakRSSBytes {
		index.Stats.PeakRSSBytes = rss
	}
}

func (index *FunctionRefIndex) observeProgress(fn *ssa.Function, started time.Time, options FunctionRefIndexOptions) {
	interval := options.ProgressInstructionInterval
	if interval <= 0 || options.PhaseObserver == nil {
		return
	}
	if index.Stats.ScannedInstructions%interval != 0 {
		return
	}
	index.observeRSS()
	options.PhaseObserver(PhaseEvent{
		Name:                "function_ref_index",
		Status:              "progress",
		WallClockMillis:     time.Since(started).Milliseconds(),
		PeakRSSBytes:        index.Stats.PeakRSSBytes,
		ScannedFunctions:    index.Stats.ScannedFunctions,
		ScannedBlocks:       index.Stats.ScannedBlocks,
		ScannedInstructions: index.Stats.ScannedInstructions,
		CurrentPackagePath:  functionPackagePath(fn),
	})
}

func (index *FunctionRefIndex) budgetExceeded(started time.Time, options FunctionRefIndexOptions) bool {
	return options.Budget > 0 && time.Since(started) >= options.Budget
}

func (index *FunctionRefIndex) recordBudgetExceeded(fn *ssa.Function) {
	for _, diagnostic := range index.Diagnostics {
		if diagnostic.Kind == "function_ref_index_budget_exceeded" {
			return
		}
	}
	index.Diagnostics = append(index.Diagnostics, Diagnostic{
		Kind:     "function_ref_index_budget_exceeded",
		Reason:   "function reference index budget exceeded; downstream results use a partial index",
		Function: functionString(fn),
	})
}

func sortedProgramFunctions(prog *ssa.Program) []*ssa.Function {
	if prog == nil {
		return nil
	}
	functions := make([]*ssa.Function, 0, len(ssautil.AllFunctions(prog)))
	for fn := range ssautil.AllFunctions(prog) {
		if fn != nil {
			functions = append(functions, fn)
		}
	}
	sort.Slice(functions, func(i, j int) bool { return functionSortKey(functions[i]) < functionSortKey(functions[j]) })
	return functions
}

func functionSortKey(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	return functionString(fn)
}

func refsForInstruction(owner *ssa.Function, instr ssa.Instruction) []FunctionRef {
	var refs []FunctionRef
	addOperandRefs := func(kind string) {
		for _, operand := range instr.Operands(nil) {
			if operand == nil || *operand == nil {
				continue
			}
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     *operand,
				Kind:        kind,
				ArgIndex:    -1,
			})
		}
	}
	switch typed := instr.(type) {
	case *ssa.Store:
		if typed.Val != nil {
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     typed.Val,
				Kind:        "store",
				ArgIndex:    -1,
			})
		}
	case *ssa.Return:
		for i, value := range typed.Results {
			if value == nil {
				continue
			}
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     value,
				Kind:        "return",
				ArgIndex:    i,
			})
		}
	case *ssa.MakeClosure:
		for i, value := range typed.Bindings {
			if value == nil {
				continue
			}
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     value,
				Kind:        "capture",
				ArgIndex:    i,
			})
		}
	case ssa.CallInstruction:
		common := typed.Common()
		if common == nil {
			break
		}
		callKind := "call_arg"
		invokeKind := "direct_invoke"
		if _, ok := instr.(*ssa.Go); ok {
			callKind = "go_arg"
			invokeKind = "go_launch"
		}
		if common.Value != nil {
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     common.Value,
				Kind:        invokeKind,
				ArgIndex:    -1,
			})
		}
		for i, arg := range common.Args {
			if arg == nil {
				continue
			}
			refs = append(refs, FunctionRef{
				Owner:       owner,
				Instruction: instr,
				Operand:     arg,
				Kind:        callKind,
				ArgIndex:    i,
			})
		}
	default:
		addOperandRefs("operand")
	}
	return refs
}

func (index *FunctionRefIndex) sort(started time.Time, options FunctionRefIndexOptions) bool {
	for _, value := range index.sortedUseValues(options) {
		if index.budgetExceeded(started, options) {
			index.recordBudgetExceeded(nil)
			return false
		}
		sort.Slice(index.Uses[value], func(i, j int) bool {
			left := refSortKey(index.Uses[value][i])
			right := refSortKey(index.Uses[value][j])
			return left < right
		})
	}
	return true
}

func (index *FunctionRefIndex) sortedUseValues(options FunctionRefIndexOptions) []ssa.Value {
	values := make([]ssa.Value, 0, len(index.Uses))
	for value := range index.Uses {
		values = append(values, value)
	}
	return values
}

func refSortKey(ref FunctionRef) string {
	owner := ""
	if ref.Owner != nil {
		owner = ref.Owner.String()
	}
	instr := ""
	if ref.Instruction != nil {
		instr = ref.Instruction.String()
	}
	return owner + "|" + ref.Kind + "|" + instr
}

func functionValue(value ssa.Value) (FunctionValue, bool) {
	switch typed := value.(type) {
	case *ssa.Function:
		return FunctionValue{Value: typed, Function: typed}, true
	case *ssa.MakeClosure:
		return FunctionValue{Value: typed, Function: closureFunction(typed), Closure: typed}, true
	default:
		return FunctionValue{}, false
	}
}

func closureFunction(closure *ssa.MakeClosure) *ssa.Function {
	if closure == nil {
		return nil
	}
	if fn, ok := closure.Fn.(*ssa.Function); ok {
		return fn
	}
	return nil
}

func analyzeFunctionValueFlow(prog *ssa.Program, index FunctionRefIndex, regionRoots []*ssa.Function) functionFlowResult {
	recorder := newFlowRecorder(prog, regionRoots)
	queue := make([]taintedValue, 0, len(index.Sources))
	for value, source := range index.Sources {
		if len(index.Uses[value]) == 0 {
			continue
		}
		queue = append(queue, taintedValue{Value: value, Source: source})
	}
	sort.Slice(queue, func(i, j int) bool { return valueSortKey(queue[i].Value) < valueSortKey(queue[j].Value) })

	visited := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		key := taintKey(current)
		if visited[key] {
			continue
		}
		visited[key] = true
		for _, ref := range index.Uses[current.Value] {
			next := propagateTaint(prog, index, recorder, current, ref)
			queue = append(queue, next...)
		}
	}
	return recorder.result()
}

func propagateTaint(prog *ssa.Program, index FunctionRefIndex, recorder *flowRecorder, current taintedValue, ref FunctionRef) []taintedValue {
	instr := ref.Instruction
	if instr == nil {
		return nil
	}
	switch typed := instr.(type) {
	case *ssa.Store:
		if typed.Val != current.Value {
			return nil
		}
		recorder.recordStore(current, typed, ref.Owner)
	case *ssa.Return:
		recorder.recordWrapper(current, ref.Owner, EdgeFunctionValueReturned, instr.Pos())
	case *ssa.MakeClosure:
		if bindingContains(typed, current.Value) {
			chain := appendWrapperLink(prog, current.Chain, current.Source, closureFunction(typed), EdgeClosureCapture, typed.Pos())
			recorder.recordWrapper(current, closureFunction(typed), EdgeClosureCapture, typed.Pos())
			return []taintedValue{{Value: typed, Source: current.Source, Chain: chain}}
		}
	case ssa.CallInstruction:
		return propagateCallTaint(prog, recorder, current, ref, typed)
	default:
		if value := passthroughValue(instr); value != nil {
			return []taintedValue{{Value: value, Source: current.Source, Chain: current.Chain}}
		}
	}
	_ = index
	return nil
}

func propagateCallTaint(prog *ssa.Program, recorder *flowRecorder, current taintedValue, ref FunctionRef, call ssa.CallInstruction) []taintedValue {
	common := call.Common()
	if common == nil {
		return nil
	}
	if common.Value == current.Value {
		edge := EdgeDirectInvoke
		if _, ok := call.(*ssa.Go); ok {
			edge = EdgeGoroutineLaunch
		}
		recorder.recordWrapper(current, common.StaticCallee(), edge, common.Pos())
		return nil
	}
	var next []taintedValue
	if ref.ArgIndex >= 0 && ref.ArgIndex < len(common.Args) && common.Args[ref.ArgIndex] == current.Value {
		paramType := callParamType(common, ref.ArgIndex)
		if paramType != nil && isInterfaceType(paramType) {
			edge := EdgeFunctionValueArg
			if _, ok := call.(*ssa.Go); ok {
				edge = EdgeGoroutineLaunch
			}
			recorder.recordCallArg(current, ref.Owner, call, edge, paramType)
		}
		if callee := common.StaticCallee(); callee != nil {
			if param := calleeParam(callee, ref.ArgIndex); param != nil {
				next = append(next, taintedValue{Value: param, Source: current.Source, Chain: current.Chain})
			}
			if call.Value() != nil {
				chain := appendWrapperLink(prog, current.Chain, current.Source, callee, EdgeFunctionValueArg, common.Pos())
				next = append(next, taintedValue{Value: call.Value(), Source: current.Source, Chain: chain})
			}
		} else if call.Value() != nil {
			next = append(next, taintedValue{Value: call.Value(), Source: current.Source, Chain: current.Chain})
		}
	}
	return next
}

func passthroughValue(instr ssa.Instruction) ssa.Value {
	switch typed := instr.(type) {
	case *ssa.ChangeInterface:
		return typed
	case *ssa.ChangeType:
		return typed
	case *ssa.Convert:
		return typed
	case *ssa.Extract:
		return typed
	case *ssa.MakeInterface:
		return typed
	case *ssa.Phi:
		return typed
	case *ssa.TypeAssert:
		return typed
	case *ssa.UnOp:
		return typed
	default:
		return nil
	}
}

func bindingContains(closure *ssa.MakeClosure, value ssa.Value) bool {
	if closure == nil {
		return false
	}
	for _, binding := range closure.Bindings {
		if binding == value {
			return true
		}
	}
	return false
}

func callParamType(common *ssa.CallCommon, argIndex int) types.Type {
	if common == nil || argIndex < 0 {
		return nil
	}
	sig := common.Signature()
	if sig == nil {
		return nil
	}
	paramIndex := argIndex
	if callee := common.StaticCallee(); callee != nil && callee.Signature != nil && callee.Signature.Recv() != nil {
		if argIndex == 0 {
			return callee.Signature.Recv().Type()
		}
		paramIndex = argIndex - 1
	}
	if sig.Params() == nil || paramIndex < 0 || paramIndex >= sig.Params().Len() {
		return nil
	}
	return sig.Params().At(paramIndex).Type()
}

func calleeParam(callee *ssa.Function, argIndex int) *ssa.Parameter {
	if callee == nil || argIndex < 0 {
		return nil
	}
	paramIndex := argIndex
	if callee.Signature != nil && callee.Signature.Recv() != nil {
		if argIndex == 0 {
			return nil
		}
		paramIndex = argIndex - 1
	}
	if paramIndex < 0 || paramIndex >= len(callee.Params) {
		return nil
	}
	return callee.Params[paramIndex]
}

func valueSortKey(value ssa.Value) string {
	if value == nil {
		return ""
	}
	return value.Name() + "|" + value.String()
}

func taintKey(value taintedValue) string {
	source := ""
	if value.Source.Function != nil {
		source = value.Source.Function.String()
	}
	return source + "|" + valueSortKey(value.Value)
}

type flowRecorder struct {
	prog          *ssa.Program
	regionRoots   []TraceNode
	external      map[string]ExternalSurface
	registrations map[string]RegistrationSite
	chains        map[string]WrapperChain
	diagnostics   []Diagnostic
}

func newFlowRecorder(prog *ssa.Program, regionRoots []*ssa.Function) *flowRecorder {
	return &flowRecorder{
		prog:          prog,
		regionRoots:   traceNodesForFunctions(prog, sortedUniqueFunctions(regionRoots)),
		external:      map[string]ExternalSurface{},
		registrations: map[string]RegistrationSite{},
		chains:        map[string]WrapperChain{},
	}
}

func (rec *flowRecorder) recordStore(current taintedValue, store *ssa.Store, owner *ssa.Function) {
	if store == nil {
		return
	}
	switch addr := store.Addr.(type) {
	case *ssa.FieldAddr:
		if typeHasServeHTTP(addr.X.Type()) {
			rec.recordRegistration(current, owner, store, EdgeFunctionValueStoredField, pointeeTypeString(addr.Type()), "field-http-handler")
			return
		}
		rec.diagnostics = append(rec.diagnostics, Diagnostic{
			Kind:        "funcvalue_terminated_at_unknown_sink",
			Function:    functionString(owner),
			Instruction: store.String(),
			Position:    sourcePosition(rec.prog, store.Pos()),
		})
	case *ssa.Global:
		rec.recordRegistration(current, owner, store, EdgeFunctionValueStoredGlobal, pointeeTypeString(addr.Type()), "global")
	case *ssa.IndexAddr:
		rec.recordRegistration(current, owner, store, EdgeFunctionValueStoredElement, pointeeTypeString(addr.Type()), "element")
	default:
		rec.diagnostics = append(rec.diagnostics, Diagnostic{
			Kind:        "funcvalue_terminated_at_unknown_sink",
			Function:    functionString(owner),
			Instruction: store.String(),
			Position:    sourcePosition(rec.prog, store.Pos()),
		})
	}
}

func (rec *flowRecorder) recordCallArg(current taintedValue, owner *ssa.Function, call ssa.CallInstruction, edge string, paramType types.Type) {
	if call == nil {
		return
	}
	sinkKind := "interface"
	if isHTTPHandlerType(paramType) {
		sinkKind = "http-handler"
	}
	rec.recordRegistration(current, owner, call, edge, paramType.String(), sinkKind)
}

func (rec *flowRecorder) recordRegistration(current taintedValue, owner *ssa.Function, instr ssa.Instruction, edge, staticType, sinkKind string) {
	source := rec.sourceNode(current)
	if source.ID == "" {
		return
	}
	site := traceNodeForFunction(rec.prog, owner)
	key := source.ID + "|" + site.ID + "|" + edge + "|" + instr.String()
	if _, exists := rec.registrations[key]; exists {
		return
	}
	rec.external[source.ID] = ExternalSurface{
		Node:      source,
		EdgeKind:  edge,
		Evidence:  []string{instr.String()},
		RegionIDs: rec.regionIDs(),
	}
	rec.registrations[key] = RegistrationSite{
		Node:                site,
		EdgeKind:            edge,
		StaticParameterType: staticType,
		SinkKind:            sinkKind,
		Handler:             source,
		RegionIDs:           rec.regionIDs(),
	}
	if len(current.Chain) > 0 {
		rec.recordChain(current, site)
	}
}

func (rec *flowRecorder) recordWrapper(current taintedValue, target *ssa.Function, edge string, pos token.Pos) {
	if target == nil {
		return
	}
	site := traceNodeForFunction(rec.prog, target)
	source := rec.sourceNode(current)
	if source.ID == "" || site.ID == "" {
		return
	}
	chain := appendWrapperLink(rec.prog, current.Chain, current.Source, target, edge, pos)
	current.Chain = chain
	rec.recordChain(current, site)
}

func (rec *flowRecorder) recordChain(current taintedValue, site TraceNode) {
	source := rec.sourceNode(current)
	if source.ID == "" || len(current.Chain) == 0 {
		return
	}
	key := source.ID + "|" + site.ID
	rec.chains[key] = WrapperChain{
		RegionRoot:       rec.firstRegionRoot(),
		ExternalSurface:  source,
		Links:            append([]WrapperLink(nil), current.Chain...),
		RegistrationSite: site,
	}
}

func (rec *flowRecorder) sourceNode(current taintedValue) TraceNode {
	return traceNodeForFunction(rec.prog, current.Source.Function)
}

func (rec *flowRecorder) firstRegionRoot() TraceNode {
	if len(rec.regionRoots) == 0 {
		return TraceNode{}
	}
	return rec.regionRoots[0]
}

func (rec *flowRecorder) regionIDs() []string {
	out := make([]string, 0, len(rec.regionRoots))
	for _, root := range rec.regionRoots {
		if root.ID != "" {
			out = append(out, root.ID)
		}
	}
	return out
}

func (rec *flowRecorder) result() functionFlowResult {
	result := functionFlowResult{
		ExternalSurfaces:  []ExternalSurface{},
		RegistrationSites: []RegistrationSite{},
		WrapperChains:     []WrapperChain{},
		Diagnostics:       append([]Diagnostic{}, rec.diagnostics...),
	}
	for _, surface := range rec.external {
		result.ExternalSurfaces = append(result.ExternalSurfaces, surface)
	}
	for _, site := range rec.registrations {
		result.RegistrationSites = append(result.RegistrationSites, site)
	}
	for _, chain := range rec.chains {
		result.WrapperChains = append(result.WrapperChains, chain)
	}
	sort.Slice(result.ExternalSurfaces, func(i, j int) bool {
		return result.ExternalSurfaces[i].Node.ID < result.ExternalSurfaces[j].Node.ID
	})
	sort.Slice(result.RegistrationSites, func(i, j int) bool {
		return result.RegistrationSites[i].Handler.ID+result.RegistrationSites[i].Node.ID < result.RegistrationSites[j].Handler.ID+result.RegistrationSites[j].Node.ID
	})
	sort.Slice(result.WrapperChains, func(i, j int) bool {
		return result.WrapperChains[i].ExternalSurface.ID+result.WrapperChains[i].RegistrationSite.ID < result.WrapperChains[j].ExternalSurface.ID+result.WrapperChains[j].RegistrationSite.ID
	})
	return result
}

func appendWrapperLink(prog *ssa.Program, chain []WrapperLink, source FunctionValue, target *ssa.Function, edge string, pos token.Pos) []WrapperLink {
	if target == nil {
		return chain
	}
	out := append([]WrapperLink(nil), chain...)
	out = append(out, WrapperLink{
		From:     traceNodeForFunction(prog, source.Function),
		To:       traceNodeForFunction(prog, target),
		EdgeKind: edge,
		Site:     sourcePosition(prog, pos),
	})
	return out
}

func functionString(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	return fn.String()
}

func pointeeTypeString(typ types.Type) string {
	if pointer, ok := typ.(*types.Pointer); ok {
		return pointer.Elem().String()
	}
	if typ == nil {
		return ""
	}
	return typ.String()
}
