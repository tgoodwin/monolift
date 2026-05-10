package activation

import (
	"fmt"
	"go/token"
	"sort"

	"golang.org/x/tools/go/ssa"
)

type callbackParamKey struct {
	fn    *ssa.Function
	index int
}

type callbackCallsite struct {
	Caller *ssa.Function
	Common *ssa.CallCommon
}

type callbackCallsiteIndex struct {
	ByCaller map[*ssa.Function][]callbackCallsite
}

// AugmentFuncArgs connects reachable callback registration calls to function
// values passed into parameters that are stored, invoked, or forwarded to a
// storing/invoking callee.
func AugmentFuncArgs(graph *Graph, program *Program, indexes ...*callbackCallsiteIndex) (*callbackCallsiteIndex, error) {
	if graph == nil {
		return nil, fmt.Errorf("graph is nil")
	}
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	var callsites *callbackCallsiteIndex
	if len(indexes) > 0 {
		callsites = indexes[0]
	}
	if callsites == nil {
		callsites = buildCallbackCallsiteIndex(program)
	}
	memo := map[callbackParamKey]bool{}
	visiting := map[callbackParamKey]bool{}

	nodes := append([]*Node(nil), graph.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodeLess(nodes[i], nodes[j])
	})
	for _, node := range nodes {
		if node == nil || node.Func == nil {
			continue
		}
		for _, callsite := range callsites.ByCaller[node.Func] {
			common := callsite.Common
			if common == nil {
				continue
			}
			callee := common.StaticCallee()
			for argIndex, arg := range common.Args {
				if !hasFunctionValueType(arg) {
					continue
				}
				targets := resolveStoredCallables(arg)
				if len(targets) == 0 {
					continue
				}
				if callee != nil && !parameterMayCallback(callee, argIndex, memo, visiting) {
					continue
				}
				for _, target := range targets {
					if target.Func == nil || hasGenericContext(target.Func) {
						continue
					}
					to := graph.AddNode(FunctionKeyForSSA(target.Func), target.Func)
					if to == nil {
						continue
					}
					graph.AddEdge(node.ID, to.ID, CallbackRegistration, positionFor(program, common.Pos()), fmt.Sprintf("callback argument to %s", callDescription(common)))
				}
			}
		}
	}
	return callsites, nil
}

func buildCallbackCallsiteIndex(program *Program) *callbackCallsiteIndex {
	index := &callbackCallsiteIndex{ByCaller: map[*ssa.Function][]callbackCallsite{}}
	if program == nil {
		return index
	}
	for _, fn := range program.Functions() {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok || call.Common() == nil {
					continue
				}
				index.ByCaller[fn] = append(index.ByCaller[fn], callbackCallsite{Caller: fn, Common: call.Common()})
			}
		}
	}
	return index
}

func parameterMayCallback(fn *ssa.Function, paramIndex int, memo map[callbackParamKey]bool, visiting map[callbackParamKey]bool) bool {
	if fn == nil || paramIndex < 0 || paramIndex >= len(fn.Params) {
		return false
	}
	key := callbackParamKey{fn: fn, index: paramIndex}
	if result, ok := memo[key]; ok {
		return result
	}
	if visiting[key] {
		return false
	}
	visiting[key] = true
	defer delete(visiting, key)

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			switch instr := instr.(type) {
			case ssa.CallInstruction:
				common := instr.Common()
				if common == nil {
					continue
				}
				if valueMatchesParameter(fn, common.Value, paramIndex) {
					memo[key] = true
					return true
				}
				callee := common.StaticCallee()
				if callee == nil {
					continue
				}
				for argIndex, arg := range common.Args {
					if valueMatchesParameter(fn, arg, paramIndex) && parameterMayCallback(callee, argIndex, memo, visiting) {
						memo[key] = true
						return true
					}
				}
			case *ssa.Store:
				if valueMatchesParameter(fn, instr.Val, paramIndex) && storesBeyondLocal(instr.Addr) {
					memo[key] = true
					return true
				}
			case *ssa.MapUpdate:
				if valueMatchesParameter(fn, instr.Value, paramIndex) {
					memo[key] = true
					return true
				}
			case *ssa.Return:
				for _, result := range instr.Results {
					if valueMatchesParameter(fn, result, paramIndex) {
						memo[key] = true
						return true
					}
				}
			}
		}
	}
	memo[key] = false
	return false
}

func storesBeyondLocal(addr ssa.Value) bool {
	addr = unwrapTransparentValue(addr)
	switch addr.(type) {
	case *ssa.FieldAddr, *ssa.Global, *ssa.IndexAddr:
		return true
	default:
		return false
	}
}

func valueMatchesParameter(fn *ssa.Function, value ssa.Value, paramIndex int) bool {
	return valueMatchesParameterSeen(fn, value, paramIndex, map[ssa.Value]bool{}, map[*ssa.Alloc]bool{})
}

func valueMatchesParameterSeen(fn *ssa.Function, value ssa.Value, paramIndex int, seenValues map[ssa.Value]bool, seenAllocs map[*ssa.Alloc]bool) bool {
	if fn == nil || paramIndex < 0 || paramIndex >= len(fn.Params) || value == nil {
		return false
	}
	param := fn.Params[paramIndex]
	value = unwrapTransparentValue(value)
	if seenValues[value] {
		return false
	}
	seenValues[value] = true
	if value == param {
		return true
	}
	if closure, ok := value.(*ssa.MakeClosure); ok {
		for _, binding := range closure.Bindings {
			if valueMatchesParameterSeen(fn, binding, paramIndex, seenValues, seenAllocs) {
				return true
			}
		}
	}
	if unop, ok := value.(*ssa.UnOp); ok && unop.Op == token.MUL {
		if alloc, ok := unwrapTransparentValue(unop.X).(*ssa.Alloc); ok {
			return allocStoresParameterSeen(fn, alloc, paramIndex, seenValues, seenAllocs)
		}
	}
	return false
}

func allocStoresParameter(fn *ssa.Function, alloc *ssa.Alloc, paramIndex int) bool {
	return allocStoresParameterSeen(fn, alloc, paramIndex, map[ssa.Value]bool{}, map[*ssa.Alloc]bool{})
}

func allocStoresParameterSeen(fn *ssa.Function, alloc *ssa.Alloc, paramIndex int, seenValues map[ssa.Value]bool, seenAllocs map[*ssa.Alloc]bool) bool {
	if fn == nil || alloc == nil {
		return false
	}
	if seenAllocs[alloc] {
		return false
	}
	seenAllocs[alloc] = true
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok || unwrapTransparentValue(store.Addr) != alloc {
				continue
			}
			if valueMatchesParameterSeen(fn, store.Val, paramIndex, seenValues, seenAllocs) {
				return true
			}
		}
	}
	return false
}

func hasFunctionValueType(value ssa.Value) bool {
	if value == nil {
		return false
	}
	_, ok := functionSignature(unwrapTransparentValue(value).Type())
	return ok
}

func callDescription(common *ssa.CallCommon) string {
	if common == nil {
		return ""
	}
	if callee := common.StaticCallee(); callee != nil {
		return FunctionKeyForSSA(callee).String()
	}
	return common.Description()
}
