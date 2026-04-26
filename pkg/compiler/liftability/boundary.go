package liftability

import (
	"fmt"
	"go/types"
	"strings"
)

type boundaryContextFirstDetector struct{}

func (boundaryContextFirstDetector) ID() PropertyID { return PropertyBoundaryContextFirst }

func (boundaryContextFirstDetector) Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error) {
	contextType, ok := contextContextType(ctx.Loaded)
	if !ok || op.Signature == nil || op.Signature.Params().Len() == 0 {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryContextFirst, VerdictUnknown, SourceTypes, "context package unavailable or signature has no parameters")}, nil
	}
	if types.Identical(op.Signature.Params().At(0).Type(), contextType) {
		return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryContextFirst, VerdictHold, SourceTypes, "first parameter is context.Context")}, nil
	}
	return VerdictViolate, []Evidence{bodyEvidence(PropertyBoundaryContextFirst, VerdictViolate, SourceTypes, "first parameter is not context.Context")}, nil
}

type boundaryVariadicFreeDetector struct{}

func (boundaryVariadicFreeDetector) ID() PropertyID { return PropertyBoundaryVariadicFree }

func (boundaryVariadicFreeDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryVariadicFree, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	if op.Signature != nil && op.Signature.Variadic() {
		return VerdictViolate, []Evidence{bodyEvidence(PropertyBoundaryVariadicFree, VerdictViolate, SourceTypes, "signature is variadic")}, nil
	}
	return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryVariadicFree, VerdictHold, SourceTypes, "signature is not variadic")}, nil
}

type boundaryNoCallableValuesDetector struct{}

func (boundaryNoCallableValuesDetector) ID() PropertyID { return PropertyBoundaryNoCallableValues }

func (boundaryNoCallableValuesDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryNoCallableValues, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	items := collectBoundaryTypeMatches(op.Signature, func(typ types.Type) (bool, string) {
		if _, ok := typ.(*types.Signature); ok {
			return true, "func-typed boundary value"
		}
		return false, ""
	})
	if len(items) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryNoCallableValues, VerdictHold, SourceTypes, "no func-typed boundary values")}, nil
	}
	return VerdictViolate, itemsToEvidence(PropertyBoundaryNoCallableValues, VerdictViolate, SourceTypes, items), nil
}

type boundaryNoStreamingValuesDetector struct{}

func (boundaryNoStreamingValuesDetector) ID() PropertyID { return PropertyBoundaryNoStreamingValues }

func (boundaryNoStreamingValuesDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryNoStreamingValues, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	items := collectBoundaryTypeMatches(op.Signature, func(typ types.Type) (bool, string) {
		if _, ok := typ.(*types.Chan); ok {
			return true, "channel-typed boundary value"
		}
		return false, ""
	})
	if len(items) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryNoStreamingValues, VerdictHold, SourceTypes, "no channel-typed boundary values")}, nil
	}
	return VerdictViolate, itemsToEvidence(PropertyBoundaryNoStreamingValues, VerdictViolate, SourceTypes, items), nil
}

type boundaryNoSyncPrimitivesDetector struct{}

func (boundaryNoSyncPrimitivesDetector) ID() PropertyID { return PropertyBoundaryNoSyncPrimitives }

func (boundaryNoSyncPrimitivesDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryNoSyncPrimitives, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	items := collectBoundaryTypeMatches(op.Signature, func(typ types.Type) (bool, string) {
		if named, ok := typ.(*types.Named); ok && named.Obj() != nil && named.Obj().Pkg() != nil {
			pkgPath := named.Obj().Pkg().Path()
			name := named.Obj().Name()
			if pkgPath == "sync" && (name == "Mutex" || name == "RWMutex" || name == "WaitGroup" || name == "Once" || name == "Cond" || name == "Pool") {
				return true, "sync primitive " + pkgPath + "." + name
			}
			if pkgPath == "sync/atomic" {
				return true, "sync primitive " + pkgPath + "." + name
			}
		}
		return false, ""
	})
	if len(items) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryNoSyncPrimitives, VerdictHold, SourceTypes, "no sync primitives at boundary")}, nil
	}
	return VerdictViolate, itemsToEvidence(PropertyBoundaryNoSyncPrimitives, VerdictViolate, SourceTypes, items), nil
}

type boundaryFullyInstantiatedDetector struct{}

func (boundaryFullyInstantiatedDetector) ID() PropertyID { return PropertyBoundaryFullyInstantiated }

func (boundaryFullyInstantiatedDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundaryFullyInstantiated, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	items := collectBoundaryTypeMatches(op.Signature, func(typ types.Type) (bool, string) {
		if _, ok := typ.(*types.TypeParam); ok {
			return true, "unresolved type parameter"
		}
		return false, ""
	})
	if len(items) == 0 {
		return VerdictHold, []Evidence{bodyEvidence(PropertyBoundaryFullyInstantiated, VerdictHold, SourceTypes, "boundary is fully instantiated")}, nil
	}
	return VerdictViolate, itemsToEvidence(PropertyBoundaryFullyInstantiated, VerdictViolate, SourceTypes, items), nil
}

type boundarySerializableViaCustomEncodingDetector struct{}

func (boundarySerializableViaCustomEncodingDetector) ID() PropertyID {
	return PropertyBoundarySerializableViaCustomEncoding
}

func (boundarySerializableViaCustomEncodingDetector) Evaluate(_ *Context, op Operation) (Verdict, []Evidence, error) {
	if op.Signature == nil {
		return VerdictUnknown, []Evidence{bodyEvidence(PropertyBoundarySerializableViaCustomEncoding, VerdictUnknown, SourceTypes, "missing signature")}, nil
	}
	var out []Evidence
	overall := VerdictHold
	walkBoundaryTypes(op.Signature, func(subject string, typ types.Type) {
		verdict, detail := serializableVerdict(typ)
		switch verdict {
		case VerdictViolate:
			overall = VerdictViolate
		case VerdictUnknown:
			if overall != VerdictViolate {
				overall = VerdictUnknown
			}
		}
		out = append(out, Evidence{
			PropertyID: PropertyBoundarySerializableViaCustomEncoding,
			Subject:    subject,
			Verdict:    verdict,
			Source:     SourceTypes,
			Detail:     detail,
		})
	})
	if len(out) == 0 {
		out = append(out, bodyEvidence(PropertyBoundarySerializableViaCustomEncoding, VerdictHold, SourceTypes, "boundary is serializable"))
	}
	sortEvidence(out)
	return overall, out, nil
}

type boundaryMatch struct {
	subject string
	detail  string
}

func collectBoundaryTypeMatches(signature *types.Signature, match func(types.Type) (bool, string)) []boundaryMatch {
	var out []boundaryMatch
	walkBoundaryTypes(signature, func(subject string, typ types.Type) {
		if ok, detail := match(typ); ok {
			out = append(out, boundaryMatch{subject: subject, detail: detail})
		}
	})
	return out
}

func walkBoundaryTypes(signature *types.Signature, visit func(subject string, typ types.Type)) {
	if signature == nil {
		return
	}
	if recv := signature.Recv(); recv != nil {
		walkType(recv.Type(), map[types.Type]bool{}, func(typ types.Type) {
			visit(SubjectReceiver, typ)
		})
	}
	for i := 0; i < signature.Params().Len(); i++ {
		subject := fmt.Sprintf("param[%d]", i)
		walkType(signature.Params().At(i).Type(), map[types.Type]bool{}, func(typ types.Type) {
			visit(subject, typ)
		})
	}
	for i := 0; i < signature.Results().Len(); i++ {
		subject := fmt.Sprintf("result[%d]", i)
		walkType(signature.Results().At(i).Type(), map[types.Type]bool{}, func(typ types.Type) {
			visit(subject, typ)
		})
	}
}

func walkType(typ types.Type, seen map[types.Type]bool, visit func(types.Type)) {
	if typ == nil || seen[typ] {
		return
	}
	seen[typ] = true
	visit(typ)
	switch t := typ.(type) {
	case *types.Pointer:
		walkType(t.Elem(), seen, visit)
	case *types.Slice:
		walkType(t.Elem(), seen, visit)
	case *types.Array:
		walkType(t.Elem(), seen, visit)
	case *types.Map:
		walkType(t.Key(), seen, visit)
		walkType(t.Elem(), seen, visit)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			walkType(t.Field(i).Type(), seen, visit)
		}
	case *types.Named:
		if hasJSONMethods(t) || isContextType(t) || isErrorType(t) {
			return
		}
		walkType(t.Underlying(), seen, visit)
	case *types.Alias:
		walkType(types.Unalias(t), seen, visit)
	case *types.Chan, *types.Signature, *types.Interface, *types.TypeParam:
		return
	}
}

func serializableVerdict(typ types.Type) (Verdict, string) {
	if isContextType(typ) {
		return VerdictHold, "context.Context is control-plane metadata"
	}
	switch t := typ.(type) {
	case nil:
		return VerdictUnknown, "missing type"
	case *types.Basic:
		if t.Kind() == types.UnsafePointer {
			return VerdictViolate, "unsafe.Pointer is not serializable"
		}
		return VerdictHold, "basic value is serializable"
	case *types.Pointer:
		verdict, detail := serializableVerdict(t.Elem())
		if verdict == VerdictHold {
			return VerdictHold, "pointer to serializable element"
		}
		return verdict, detail
	case *types.Array, *types.Slice:
		return VerdictHold, "collection element serializability delegated to element walk"
	case *types.Map:
		return VerdictHold, "map serializability delegated to key/value walk"
	case *types.Struct:
		return VerdictHold, "struct serializability delegated to field walk"
	case *types.Named:
		if hasJSONMethods(t) {
			return VerdictHold, "custom JSON encoding on " + t.Obj().Name()
		}
		return serializableVerdict(t.Underlying())
	case *types.Interface:
		if isErrorType(t) {
			return VerdictHold, "error interface is transport-visible"
		}
		return VerdictUnknown, "interface value needs runtime concrete type"
	case *types.Signature:
		return VerdictViolate, "function values are not serializable"
	case *types.Chan:
		return VerdictViolate, "channel values are not serializable"
	case *types.TypeParam:
		return VerdictViolate, "type parameter is not instantiated"
	default:
		return VerdictUnknown, "serializability not proven for " + strings.TrimSpace(types.TypeString(typ, nil))
	}
}

func isContextType(typ types.Type) bool {
	named, ok := typ.(*types.Named)
	if !ok || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

func hasJSONMethods(named *types.Named) bool {
	if named == nil || named.Obj() == nil || named.Obj().Pkg() == nil {
		return false
	}
	for _, recv := range []types.Type{named, types.NewPointer(named)} {
		methods := types.NewMethodSet(recv)
		hasMarshal := false
		hasUnmarshal := false
		for i := 0; i < methods.Len(); i++ {
			method := methods.At(i)
			if method == nil || method.Obj() == nil {
				continue
			}
			switch method.Obj().Name() {
			case "MarshalJSON":
				hasMarshal = true
			case "UnmarshalJSON":
				hasUnmarshal = true
			}
		}
		if hasMarshal && hasUnmarshal {
			return true
		}
	}
	return false
}

func itemsToEvidence(id PropertyID, verdict Verdict, source Source, items []boundaryMatch) []Evidence {
	out := make([]Evidence, 0, len(items))
	for _, item := range items {
		out = append(out, Evidence{
			PropertyID: id,
			Subject:    item.subject,
			Verdict:    verdict,
			Source:     source,
			Detail:     item.detail,
		})
	}
	sortEvidence(out)
	return out
}

func bodyEvidence(id PropertyID, verdict Verdict, source Source, detail string) Evidence {
	return Evidence{
		PropertyID: id,
		Subject:    SubjectBody,
		Verdict:    verdict,
		Source:     source,
		Detail:     detail,
	}
}
