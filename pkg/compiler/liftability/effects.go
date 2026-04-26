package liftability

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

type functionFacts struct {
	hasGo               bool
	hasLoopBackEdge     bool
	hasReceive          bool
	hasPanic            bool
	globalWrites        []string
	globalReads         []string
	paramMutations      []string
	receiverMutations   []string
	paramEscapes        []string
	paramInterfaceCalls []string
}

type callgraphFacts struct {
	packagePaths []string
	symbols      []string
}

func (ctx *Context) facts(fn *ssa.Function) *functionFacts {
	if fn == nil {
		return nil
	}
	if facts, ok := ctx.factFor(fn); ok {
		return facts
	}
	facts := &functionFacts{}
	for _, block := range fn.Blocks {
		for _, succ := range block.Succs {
			if succ != nil && succ.Index <= block.Index {
				facts.hasLoopBackEdge = true
			}
		}
		for _, instr := range block.Instrs {
			switch typed := instr.(type) {
			case *ssa.Go:
				facts.hasGo = true
				for _, arg := range typed.Common().Args {
					if subject := boundaryOrigin(fn, arg, map[ssa.Value]bool{}); subject != "" {
						facts.paramEscapes = append(facts.paramEscapes, "boundary value escapes to goroutine via "+subject)
					}
				}
			case *ssa.UnOp:
				if typed.Op == token.ARROW {
					facts.hasReceive = true
				}
				if global, ok := typed.X.(*ssa.Global); ok {
					facts.globalReads = append(facts.globalReads, global.String())
				}
			case *ssa.Select:
				for _, state := range typed.States {
					if state.Dir == types.RecvOnly || state.Dir == types.SendRecv {
						facts.hasReceive = true
					}
				}
			case *ssa.Panic:
				facts.hasPanic = true
			case *ssa.Store:
				if global, ok := typed.Addr.(*ssa.Global); ok {
					facts.globalWrites = append(facts.globalWrites, global.String())
					if subject := boundaryOrigin(fn, typed.Val, map[ssa.Value]bool{}); subject != "" {
						facts.paramEscapes = append(facts.paramEscapes, "boundary value stored in global via "+subject)
					}
					continue
				}
				if subject := boundaryOrigin(fn, typed.Addr, map[ssa.Value]bool{}); subject != "" {
					detail := fmt.Sprintf("store through %s-derived address", subject)
					if subject == SubjectReceiver {
						facts.receiverMutations = append(facts.receiverMutations, detail)
					} else {
						facts.paramMutations = append(facts.paramMutations, detail)
					}
				}
			case *ssa.MakeClosure:
				for _, binding := range typed.Bindings {
					if subject := boundaryOrigin(fn, binding, map[ssa.Value]bool{}); subject != "" {
						facts.paramEscapes = append(facts.paramEscapes, "boundary value captured by closure via "+subject)
					}
				}
			}
			if call, ok := instr.(ssa.CallInstruction); ok {
				common := call.Common()
				if common != nil && common.IsInvoke() {
					if subject := boundaryOrigin(fn, common.Value, map[ssa.Value]bool{}); subject != "" {
						facts.paramInterfaceCalls = append(facts.paramInterfaceCalls, "invoke on "+subject+" receiver")
					}
				}
			}
			for _, operand := range instr.Operands(nil) {
				if operand == nil || *operand == nil {
					continue
				}
				if global, ok := (*operand).(*ssa.Global); ok {
					facts.globalReads = append(facts.globalReads, global.String())
				}
			}
		}
	}
	sort.Strings(facts.globalWrites)
	sort.Strings(facts.globalReads)
	sort.Strings(facts.paramMutations)
	sort.Strings(facts.receiverMutations)
	sort.Strings(facts.paramEscapes)
	sort.Strings(facts.paramInterfaceCalls)
	ctx.storeFact(fn, facts)
	if cached, ok := ctx.factFor(fn); ok {
		return cached
	}
	return facts
}

func (ctx *Context) callgraphFactsFor(fn *ssa.Function) callgraphFacts {
	if facts, ok := ctx.callgraphFactFor(fn); ok {
		return facts
	}
	if fn == nil || ctx.CallGraph == nil {
		return callgraphFacts{}
	}
	seen := map[*ssa.Function]bool{}
	pkgs := map[string]bool{}
	symbols := map[string]bool{}
	queue := []*ssa.Function{fn}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == nil || seen[current] {
			continue
		}
		seen[current] = true
		if current.Package() != nil && current.Package().Pkg != nil {
			pkgPath := current.Package().Pkg.Path()
			pkgs[pkgPath] = true
			symbols[pkgPath+"."+current.Name()] = true
		}
		node := ctx.CallGraph.Nodes[current]
		if node == nil {
			continue
		}
		for _, edge := range node.Out {
			if edge == nil || edge.Callee == nil || edge.Callee.Func == nil {
				continue
			}
			queue = append(queue, edge.Callee.Func)
		}
	}
	pkgList := make([]string, 0, len(pkgs))
	for pkg := range pkgs {
		pkgList = append(pkgList, pkg)
	}
	symbolList := make([]string, 0, len(symbols))
	for symbol := range symbols {
		symbolList = append(symbolList, symbol)
	}
	sort.Strings(pkgList)
	sort.Strings(symbolList)
	facts := callgraphFacts{packagePaths: pkgList, symbols: symbolList}
	ctx.storeCallgraphFact(fn, facts)
	if cached, ok := ctx.callgraphFactFor(fn); ok {
		return cached
	}
	return facts
}

func boundaryOrigin(fn *ssa.Function, value ssa.Value, seen map[ssa.Value]bool) string {
	if value == nil || seen[value] {
		return ""
	}
	seen[value] = true
	switch typed := value.(type) {
	case *ssa.Parameter:
		for i, param := range fn.Params {
			if param != typed {
				continue
			}
			if fn.Signature != nil && fn.Signature.Recv() != nil && i == 0 {
				return SubjectReceiver
			}
			offset := i
			if fn.Signature != nil && fn.Signature.Recv() != nil {
				offset--
			}
			return fmt.Sprintf("param[%d]", offset)
		}
	case *ssa.FieldAddr:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.IndexAddr:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Field:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Index:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Lookup:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Slice:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.UnOp:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.ChangeType:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Convert:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.MakeInterface:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.TypeAssert:
		return boundaryOrigin(fn, typed.X, seen)
	case *ssa.Phi:
		for _, edge := range typed.Edges {
			if subject := boundaryOrigin(fn, edge, seen); subject != "" {
				return subject
			}
		}
	}
	return ""
}

type effectsNoParamHeapMutationDetector struct{}

func (effectsNoParamHeapMutationDetector) ID() PropertyID { return PropertyEffectsNoParamHeapMutation }

func (effectsNoParamHeapMutationDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoParamHeapMutation, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.paramMutations) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoParamHeapMutation, VerdictHold, SourceSSA, "no param-derived heap mutation observed")}, nil
	}
	out := make([]Evidence, 0, len(facts.paramMutations))
	for _, detail := range facts.paramMutations {
		out = append(out, Evidence{
			PropertyID: PropertyEffectsNoParamHeapMutation,
			Subject:    SubjectBody,
			Verdict:    VerdictViolate,
			Source:     SourceSSA,
			Detail:     detail,
		})
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type effectsNoParamEscapeDetector struct{}

func (effectsNoParamEscapeDetector) ID() PropertyID { return PropertyEffectsNoParamEscape }

func (effectsNoParamEscapeDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoParamEscape, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.paramEscapes) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoParamEscape, VerdictHold, SourceSSA, "no obvious boundary escapes observed")}, nil
	}
	out := make([]Evidence, 0, len(facts.paramEscapes))
	for _, detail := range facts.paramEscapes {
		out = append(out, bodyEvidence(PropertyEffectsNoParamEscape, VerdictViolate, SourceSSA, detail))
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type effectsNoGlobalWritesDetector struct{}

func (effectsNoGlobalWritesDetector) ID() PropertyID { return PropertyEffectsNoGlobalWrites }

func (effectsNoGlobalWritesDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoGlobalWrites, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.globalWrites) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoGlobalWrites, VerdictHold, SourceSSA, "no mutable global writes observed")}, nil
	}
	out := make([]Evidence, 0, len(facts.globalWrites))
	for _, detail := range facts.globalWrites {
		out = append(out, bodyEvidence(PropertyEffectsNoGlobalWrites, VerdictViolate, SourceSSA, "store to mutable global "+detail))
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type effectsNoGlobalReadsDetector struct{}

func (effectsNoGlobalReadsDetector) ID() PropertyID { return PropertyEffectsNoGlobalReads }

func (effectsNoGlobalReadsDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoGlobalReads, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.globalReads) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoGlobalReads, VerdictHold, SourceSSA, "no mutable global reads observed")}, nil
	}
	out := make([]Evidence, 0, len(facts.globalReads))
	for _, detail := range facts.globalReads {
		out = append(out, bodyEvidence(PropertyEffectsNoGlobalReads, VerdictViolate, SourceSSA, "load from global "+detail))
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type effectsNoParamInterfaceCallbacksDetector struct{}

func (effectsNoParamInterfaceCallbacksDetector) ID() PropertyID {
	return PropertyEffectsNoParamInterfaceCallbacks
}

func (effectsNoParamInterfaceCallbacksDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoParamInterfaceCallbacks, VerdictUnknown, SourceSSA, "no SSA facts available")}, nil
	}
	if len(facts.paramInterfaceCalls) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoParamInterfaceCallbacks, VerdictHold, SourceSSA, "no boundary-derived interface callbacks observed")}, nil
	}
	out := make([]Evidence, 0, len(facts.paramInterfaceCalls))
	for _, detail := range facts.paramInterfaceCalls {
		out = append(out, bodyEvidence(PropertyEffectsNoParamInterfaceCallbacks, VerdictViolate, SourceSSA, detail))
	}
	sortEvidence(out)
	return VerdictViolate, out, nil
}

type effectsNoReflectUnsafeDetector struct{}

func (effectsNoReflectUnsafeDetector) ID() PropertyID { return PropertyEffectsNoReflectUnsafe }

func (effectsNoReflectUnsafeDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoReflectUnsafe, VerdictUnknown, SourceCallgraph, "no SSA facts available")}, nil
	}
	callgraphFacts := ctx.callgraphFactsFor(op.Function)
	for _, symbol := range callgraphFacts.symbols {
		if strings.HasPrefix(symbol, "reflect.") || strings.HasPrefix(symbol, "runtime.SetFinalizer") || strings.HasPrefix(symbol, "unsafe.") {
			return VerdictViolate, []Evidence{bodyEvidence(PropertyEffectsNoReflectUnsafe, VerdictViolate, SourceCallgraph, "reachable symbol "+symbol)}, nil
		}
	}
	return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoReflectUnsafe, VerdictHold, SourceCallgraph, "no reflect/unsafe/finalizer symbols reachable")}, nil
}

type effectsNoOSSideEffectsDetector struct{}

func (effectsNoOSSideEffectsDetector) ID() PropertyID { return PropertyEffectsNoOSSideEffects }

func (effectsNoOSSideEffectsDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	facts := ctx.facts(op.Function)
	if facts == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyEffectsNoOSSideEffects, VerdictUnknown, SourceCallgraph, "no SSA facts available")}, nil
	}
	callgraphFacts := ctx.callgraphFactsFor(op.Function)
	var hits []Evidence
	for _, pkgPath := range callgraphFacts.packagePaths {
		if pkgPath == "os" || pkgPath == "syscall" || strings.HasPrefix(pkgPath, "net") {
			hits = append(hits, bodyEvidence(PropertyEffectsNoOSSideEffects, VerdictViolate, SourceCallgraph, "reachable package "+pkgPath))
		}
	}
	if len(hits) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyEffectsNoOSSideEffects, VerdictHold, SourceCallgraph, "no direct OS-side-effect packages reachable")}, nil
	}
	sortEvidence(hits)
	return VerdictViolate, hits, nil
}
