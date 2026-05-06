package activation

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

func classifyErrorSemantics(fn *ssa.Function) ErrorSemClass {
	if fn == nil || fn.Signature == nil {
		return ErrorInfeasible
	}
	results := fn.Signature.Results()
	if results.Len() == 0 {
		return NeedsWrapper
	}
	for i := 0; i < results.Len(); i++ {
		if typeImplementsError(results.At(i).Type()) {
			return ErrorOK
		}
	}
	return NeedsWrapper
}

func typeImplementsError(typ types.Type) bool {
	if typ == nil {
		return false
	}
	errorObject := types.Universe.Lookup("error")
	if errorObject == nil {
		return false
	}
	errorInterface, ok := errorObject.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}
	typ = types.Unalias(typ)
	if types.Implements(typ, errorInterface) {
		return true
	}
	if _, ok := typ.(*types.Pointer); ok {
		return false
	}
	switch typ.(type) {
	case *types.Named, *types.Struct:
		return types.Implements(types.NewPointer(typ), errorInterface)
	default:
		return false
	}
}
