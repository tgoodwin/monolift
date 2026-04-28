package entrypath

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

const netHTTPPackagePath = "net/http"

// BoundaryPredicate detects one family of InvocationBoundary evidence without
// tying SeedSet construction to a specific protocol or framework.
type BoundaryPredicate interface {
	Name() string
	MatchOwner(owner *ssa.Function) []BoundaryPredicateEvidence
}

// BoundaryPredicateEvidence is the local record used to turn predicate matches
// into BoundarySeeds. Instruction is nil for signature-level evidence.
type BoundaryPredicateEvidence struct {
	Predicate   string
	Owner       *ssa.Function
	Instruction ssa.Instruction
	StaticType  string
	Reason      string
}

type netHTTPBoundaryPredicate struct{}

func defaultBoundaryPredicates() []BoundaryPredicate {
	return []BoundaryPredicate{netHTTPBoundaryPredicate{}}
}

func (netHTTPBoundaryPredicate) Name() string {
	return netHTTPPackagePath
}

func (predicate netHTTPBoundaryPredicate) MatchOwner(owner *ssa.Function) []BoundaryPredicateEvidence {
	if owner == nil {
		return nil
	}
	var evidence []BoundaryPredicateEvidence
	evidence = append(evidence, predicate.signatureEvidence(owner, owner.Signature)...)
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			evidence = append(evidence, predicate.instructionEvidence(owner, instr)...)
		}
	}
	return evidence
}

func (predicate netHTTPBoundaryPredicate) signatureEvidence(owner *ssa.Function, sig *types.Signature) []BoundaryPredicateEvidence {
	if sig == nil || sig.Params() == nil {
		return nil
	}
	var evidence []BoundaryPredicateEvidence
	for i := 0; i < sig.Params().Len(); i++ {
		param := sig.Params().At(i)
		if match, ok := netHTTPBoundaryTypeMatch(param.Type()); ok {
			evidence = append(evidence, predicate.evidence(owner, nil, param.Type(), "signature_"+match.reason))
		}
	}
	return evidence
}

func (predicate netHTTPBoundaryPredicate) instructionEvidence(owner *ssa.Function, instr ssa.Instruction) []BoundaryPredicateEvidence {
	switch typed := instr.(type) {
	case *ssa.Store:
		var evidence []BoundaryPredicateEvidence
		evidence = append(evidence, predicate.valueEvidence(owner, instr, typed.Val, "stored_value")...)
		evidence = append(evidence, predicate.valueEvidence(owner, instr, typed.Addr, "store_address")...)
		return evidence
	case ssa.CallInstruction:
		return predicate.callEvidence(owner, typed)
	default:
		return nil
	}
}

func (predicate netHTTPBoundaryPredicate) callEvidence(owner *ssa.Function, call ssa.CallInstruction) []BoundaryPredicateEvidence {
	if call == nil || call.Common() == nil {
		return nil
	}
	common := call.Common()
	var evidence []BoundaryPredicateEvidence
	if callee := common.StaticCallee(); callee != nil {
		for _, item := range predicate.signatureEvidence(owner, callee.Signature) {
			item.Instruction = call
			item.Reason = "callee_" + item.Reason
			evidence = append(evidence, item)
		}
	}
	for i, arg := range common.Args {
		if arg != nil {
			evidence = append(evidence, predicate.valueEvidence(owner, call, arg, "argument")...)
		}
		if paramType := callParamType(common, i); paramType != nil {
			if match, ok := netHTTPBoundaryTypeMatch(paramType); ok {
				evidence = append(evidence, predicate.evidence(owner, call, paramType, "parameter_"+match.reason))
			}
		}
	}
	return evidence
}

func (predicate netHTTPBoundaryPredicate) valueEvidence(owner *ssa.Function, instr ssa.Instruction, value ssa.Value, role string) []BoundaryPredicateEvidence {
	if value == nil {
		return nil
	}
	var evidence []BoundaryPredicateEvidence
	if match, ok := netHTTPBoundaryTypeMatch(value.Type()); ok {
		evidence = append(evidence, predicate.evidence(owner, instr, value.Type(), role+"_"+match.reason))
	}
	switch typed := value.(type) {
	case *ssa.FieldAddr:
		if match, ok := netHTTPBoundaryTypeMatch(typed.X.Type()); ok {
			evidence = append(evidence, predicate.evidence(owner, instr, typed.X.Type(), role+"_field_owner_"+match.reason))
		}
	case *ssa.UnOp:
		if typed.X != nil {
			if match, ok := netHTTPBoundaryTypeMatch(typed.X.Type()); ok {
				evidence = append(evidence, predicate.evidence(owner, instr, typed.X.Type(), role+"_unop_"+match.reason))
			}
		}
	}
	return evidence
}

func (predicate netHTTPBoundaryPredicate) evidence(owner *ssa.Function, instr ssa.Instruction, typ types.Type, reason string) BoundaryPredicateEvidence {
	return BoundaryPredicateEvidence{
		Predicate:   predicate.Name(),
		Owner:       owner,
		Instruction: instr,
		StaticType:  typeString(typ),
		Reason:      reason,
	}
}

type netHTTPBoundaryMatch struct {
	reason string
}

func netHTTPBoundaryTypeMatch(typ types.Type) (netHTTPBoundaryMatch, bool) {
	switch {
	case isHTTPHandlerType(typ):
		return netHTTPBoundaryMatch{reason: "http_handler"}, true
	case isHTTPHandlerFuncType(typ):
		return netHTTPBoundaryMatch{reason: "http_handler_func"}, true
	case isHTTPServerType(typ):
		return netHTTPBoundaryMatch{reason: "http_server"}, true
	case typeHasServeHTTP(typ):
		return netHTTPBoundaryMatch{reason: "serve_http_shape"}, true
	default:
		return netHTTPBoundaryMatch{}, false
	}
}

func signatureAcceptsHandler(sig *types.Signature) bool {
	return len(netHTTPBoundaryPredicate{}.signatureEvidence(nil, sig)) > 0
}

func isInterfaceType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	_, ok := typ.Underlying().(*types.Interface)
	return ok
}

func typeHasServeHTTP(typ types.Type) bool {
	if typ == nil {
		return false
	}
	switch typ.(type) {
	case *types.Named, *types.Pointer, *types.Interface:
	default:
		return false
	}
	if hasServeHTTP(types.NewMethodSet(typ)) {
		return true
	}
	if _, ok := typ.(*types.Pointer); !ok {
		return hasServeHTTP(types.NewMethodSet(types.NewPointer(typ)))
	}
	return false
}

func hasServeHTTP(methods *types.MethodSet) bool {
	if methods == nil {
		return false
	}
	for i := 0; i < methods.Len(); i++ {
		selection := methods.At(i)
		if selection == nil {
			continue
		}
		method, ok := selection.Obj().(*types.Func)
		if ok && isServeHTTPMethod(method) {
			return true
		}
	}
	return false
}

func isHTTPHandlerType(typ types.Type) bool {
	if typ == nil {
		return false
	}
	if named, ok := typ.(*types.Named); ok {
		obj := named.Obj()
		if obj != nil && obj.Pkg() != nil && obj.Pkg().Path() == netHTTPPackagePath && obj.Name() == "Handler" {
			return true
		}
		return isHTTPHandlerType(named.Underlying())
	}
	if iface, ok := typ.Underlying().(*types.Interface); ok {
		return interfaceHasServe(iface)
	}
	return false
}

func isHTTPBoundaryType(typ types.Type) bool {
	_, ok := netHTTPBoundaryTypeMatch(typ)
	return ok
}

func isHTTPHandlerFuncType(typ types.Type) bool {
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == netHTTPPackagePath && named.Obj().Name() == "HandlerFunc"
}

func isHTTPServerType(typ types.Type) bool {
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == netHTTPPackagePath && named.Obj().Name() == "Server"
}

func interfaceHasServe(iface *types.Interface) bool {
	if iface == nil {
		return false
	}
	iface.Complete()
	for i := 0; i < iface.NumMethods(); i++ {
		if isServeHTTPMethod(iface.Method(i)) {
			return true
		}
	}
	return false
}

func isServeHTTPMethod(method *types.Func) bool {
	if method == nil || method.Name() != "ServeHTTP" {
		return false
	}
	sig, ok := method.Type().(*types.Signature)
	if !ok || sig.Params() == nil || sig.Params().Len() != 2 {
		return false
	}
	return isHTTPResponseWriterType(sig.Params().At(0).Type()) && isHTTPRequestPointerType(sig.Params().At(1).Type())
}

func isHTTPResponseWriterType(typ types.Type) bool {
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == netHTTPPackagePath && named.Obj().Name() == "ResponseWriter"
}

func isHTTPRequestPointerType(typ types.Type) bool {
	pointer, ok := typ.(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := pointer.Elem().(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == netHTTPPackagePath && named.Obj().Name() == "Request"
}

func typeString(typ types.Type) string {
	if typ == nil {
		return ""
	}
	return typ.String()
}
