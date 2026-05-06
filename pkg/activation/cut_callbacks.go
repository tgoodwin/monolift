package activation

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func classifyCallbacks(fn *ssa.Function, aboveCut []*Node) CallbackClass {
	if fn == nil || fn.Signature == nil {
		return ZeroEstimated
	}

	evidence := countCallbackBoundaryValues(fn)
	evidence += countFunctionTypedFreeVars(fn)
	bodyAvailable := len(fn.Blocks) > 0
	if bodyAvailable {
		evidence += countReverseCalls(fn, aboveCut)
	}

	if evidence == 0 {
		if !bodyAvailable {
			return ZeroEstimated
		}
		return ZeroConfirmed
	}
	switch {
	case evidence == 1:
		return Low
	case evidence <= 3:
		return Moderate
	default:
		return Many
	}
}

func countCallbackBoundaryValues(fn *ssa.Function) int {
	count := 0
	signature := fn.Signature
	params := signature.Params()
	for i := 0; i < params.Len(); i++ {
		typ := params.At(i).Type()
		if signature.Variadic() && i == params.Len()-1 {
			if slice, ok := types.Unalias(typ).(*types.Slice); ok {
				typ = slice.Elem()
			}
		}
		if typeContainsCallback(typ, map[types.Type]bool{}) {
			count++
		}
	}
	return count
}

func countFunctionTypedFreeVars(fn *ssa.Function) int {
	count := 0
	for _, freeVar := range fn.FreeVars {
		if freeVar != nil && typeContainsCallback(freeVar.Type(), map[types.Type]bool{}) {
			count++
		}
	}
	return count
}

func countReverseCalls(fn *ssa.Function, aboveCut []*Node) int {
	aboveKeys := map[string]bool{}
	aboveFuncs := map[*ssa.Function]bool{}
	for _, node := range aboveCut {
		if node == nil {
			continue
		}
		if node.Func != nil {
			aboveFuncs[node.Func] = true
		}
		if !node.Key.IsZero() {
			aboveKeys[node.Key.String()] = true
		}
	}
	if len(aboveKeys) == 0 && len(aboveFuncs) == 0 {
		return 0
	}

	count := 0
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok || call.Common() == nil {
				continue
			}
			callee := call.Common().StaticCallee()
			if callee == nil {
				continue
			}
			if aboveFuncs[callee] || aboveKeys[FunctionKeyForSSA(callee).String()] {
				count++
			}
		}
	}
	return count
}

func typeContainsCallback(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	switch t := typ.(type) {
	case *types.Signature:
		return true
	case *types.Pointer:
		return typeContainsCallback(t.Elem(), seen)
	case *types.Slice:
		return typeContainsCallback(t.Elem(), seen)
	case *types.Array:
		return typeContainsCallback(t.Elem(), seen)
	case *types.Map:
		return typeContainsCallback(t.Key(), seen) || typeContainsCallback(t.Elem(), seen)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if typeContainsCallback(t.Field(i).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Interface:
		t = t.Complete()
		for i := 0; i < t.NumMethods(); i++ {
			if methodSignatureContainsCallback(t.Method(i).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Named:
		return typeContainsCallback(t.Underlying(), seen)
	default:
		return false
	}
}

func methodSignatureContainsCallback(typ types.Type, seen map[types.Type]bool) bool {
	signature, ok := types.Unalias(typ).(*types.Signature)
	if !ok {
		return false
	}
	params := signature.Params()
	for i := 0; i < params.Len(); i++ {
		paramType := params.At(i).Type()
		if signature.Variadic() && i == params.Len()-1 {
			if slice, ok := types.Unalias(paramType).(*types.Slice); ok {
				paramType = slice.Elem()
			}
		}
		if typeContainsCallback(paramType, seen) {
			return true
		}
	}
	results := signature.Results()
	for i := 0; i < results.Len(); i++ {
		if typeContainsCallback(results.At(i).Type(), seen) {
			return true
		}
	}
	return false
}
