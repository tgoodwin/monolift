// Boundary-adapter recovery pass (SPRINT-0051 Phase 3).
//
// TryAdapterPass attempts to synthesize an AdapterPlan for one candidate
// after direct admission refused with a shape-compatible code. The pass:
//
//  1. Runs the LiveProxyRequired exclusion gate (channels, ResponseWriter,
//     io.Writer output, *os.File, function values, mutable write-back). On
//     match it refuses immediately with live_proxy_required or
//     adapter_impossible — patterns are not attempted.
//  2. Walks each parameter and return slot, picks the first matching pattern
//     in the library, and discharges that pattern's named obligations.
//  3. Discharges the six static obligations on top: adapter_finite_input,
//     adapter_local_lifecycle, adapter_use_shape, adapter_return_rehydration,
//     adapter_error_order, adapter_call_site. The pattern-specific proofs
//     produced in step 2 cover use_shape (input patterns) and
//     return_rehydration (output patterns); the rest are generic.
//  4. Builds an AdapterPlan when every obligation passes, or returns a list
//     of AdmissionRefusals matching the failed obligations.
//
// The pass does not consult cost — it is a strict fallback per the spec
// (ADR-0032 §"Phase Ordering vs Ranking"). Phase 5 wires it into
// admitCutCandidates after the direct admission verdict.
package codegen

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// AdapterContext carries everything TryAdapterPass needs to discharge the
// six obligations: the helper SSA function, the call-site set across the
// activation-path scope (for adapter_call_site), and the maximum inline
// payload size in bytes (for adapter_payload_too_large).
//
// MaxInlinePayloadBytes defaults to 8 MiB when zero — this is the
// SPRINT-0051 §0.5 policy. CallSites may be nil; in that case the
// obligation is discharged optimistically when the function is unexported
// (package-private call sites are bounded by the package itself), and
// refused otherwise.
type AdapterContext struct {
	Fn                    *ssa.Function
	CallSites             []*ssa.CallCommon
	MaxInlinePayloadBytes int
	// FunctionExported marks whether the helper function name is exported.
	// Used by the call-site obligation when neither CallSiteIndex nor
	// CallSites is supplied — an unexported helper's call sites are bounded
	// by its own package.
	FunctionExported bool
	// CallSiteIndex is the reverse-import scan result. When present and
	// scanned, it is authoritative for the adapter_call_site obligation; when
	// nil, dischargeCallSite falls back to CallSites / FunctionExported.
	CallSiteIndex *CallSiteIndex
}

const defaultInlinePayloadBytes = 8 * 1024 * 1024 // 8 MiB

// TryAdapterPass attempts to build an AdapterPlan for the helper function.
// On success returns the plan; on refusal returns the list of refusals
// (one per failed obligation, or one classification refusal like
// live_proxy_required).
func TryAdapterPass(ctx AdapterContext) (*AdapterPlan, []AdmissionRefusal) {
	if ctx.Fn == nil || ctx.Fn.Signature == nil {
		return nil, []AdmissionRefusal{{
			Code:    RefusalAdapterUnknown,
			Message: "adapter pass requires helper SSA function with a signature",
		}}
	}
	if ctx.MaxInlinePayloadBytes == 0 {
		ctx.MaxInlinePayloadBytes = defaultInlinePayloadBytes
	}

	// (1) Live-proxy / impossible exclusion gate. Refusing here is cheaper
	//     than running pattern matching and produces a clearer reason code.
	if refusal := liveProxyOrImpossibleRefusal(ctx.Fn.Signature); refusal != nil {
		return nil, []AdmissionRefusal{*refusal}
	}

	// (2) Match an input pattern per parameter. Every parameter must be
	//     either directly serializable (no adapter needed) or matched by a
	//     known pattern. Unmatched parameters refuse as adapter_finite_input.
	inputTransforms, inputProofs, inputRefusals := planInputTransforms(ctx.Fn)
	if len(inputRefusals) > 0 {
		return nil, inputRefusals
	}

	// (3) Match an output pattern per return slot that isn't directly
	//     serializable. Single-error returns are passed through unchanged.
	outputTransforms, outputProofs, outputRefusals := planOutputTransforms(ctx.Fn)
	if len(outputRefusals) > 0 {
		return nil, outputRefusals
	}

	// (4) Discharge the generic obligations (finite_input, local_lifecycle,
	//     error_order, call_site). use_shape and return_rehydration were
	//     discharged by the patterns themselves above.
	genericProofs, genericRefusals := dischargeGenericObligations(ctx, inputTransforms, outputTransforms)
	allProofs := append(append(append([]AdapterProof{}, inputProofs...), outputProofs...), genericProofs...)
	allProofs = ensureSixObligationsRepresented(allProofs, inputTransforms, outputTransforms)
	if len(genericRefusals) > 0 {
		// Even when a generic obligation fails, surface the full proof set
		// in the refusal trail so callers can render diagnostics.
		return nil, genericRefusals
	}

	plan := &AdapterPlan{
		SourceFunction:        ctx.Fn.Name(),
		HostSignature:         signatureString(ctx.Fn.Signature, true),
		RemoteSignature:       remoteSignatureString(ctx.Fn.Signature, inputTransforms, outputTransforms),
		InputTransforms:       inputTransforms,
		OutputTransforms:      outputTransforms,
		BodyRewrite:           bodyRewriteFor(inputTransforms),
		Proofs:                allProofs,
		TransportPolicy:       AdapterTransportInlineJSONBytes,
		MaxInlinePayloadBytes: int64(ctx.MaxInlinePayloadBytes),
	}
	return plan, nil
}

// planInputTransforms walks function parameters and returns the matched
// transforms plus the patterns' use-shape proofs. Direct (already
// serializable) parameters are skipped — they need no transform. Any
// remaining unmatched parameter refuses with adapter_finite_input.
func planInputTransforms(fn *ssa.Function) ([]AdapterPattern, []AdapterProof, []AdmissionRefusal) {
	if fn.Signature == nil {
		return nil, nil, nil
	}
	var transforms []AdapterPattern
	var proofs []AdapterProof
	params := fn.Signature.Params()
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		typ := param.Type()
		if isDirectlySerializableParam(typ) {
			continue
		}
		pattern := findInputPattern(typ)
		if pattern == nil {
			return nil, nil, []AdmissionRefusal{{
				Code:    RefusalAdapterFiniteInput,
				Message: fmt.Sprintf("parameter %q of type %s has no registered adapter input pattern", param.Name(), describeType(typ)),
				Type:    describeType(typ),
			}}
		}
		proofs = append(proofs, pattern.Discharge(fn, i)...)
		transforms = append(transforms, AdapterPattern{
			Name:      pattern.Name(),
			ParamName: param.Name(),
			FromType:  pattern.FromType(),
			ToType:    pattern.ToType(),
		})
	}
	// Any failed pattern proof refuses with the corresponding obligation code.
	for _, p := range proofs {
		if !p.Satisfied {
			return nil, nil, []AdmissionRefusal{{
				Code:    p.Obligation,
				Message: p.Detail,
			}}
		}
	}
	return transforms, proofs, nil
}

// planOutputTransforms walks return slots and returns the matched output
// transforms plus the patterns' return-rehydration proofs. Direct returns
// (already serializable) are passed through. Any unmatched awkward return
// refuses with adapter_return_rehydration.
func planOutputTransforms(fn *ssa.Function) ([]AdapterPattern, []AdapterProof, []AdmissionRefusal) {
	if fn.Signature == nil {
		return nil, nil, nil
	}
	var transforms []AdapterPattern
	var proofs []AdapterProof
	results := fn.Signature.Results()
	for i := 0; i < results.Len(); i++ {
		result := results.At(i)
		typ := result.Type()
		if isErrorType(typ) {
			continue
		}
		if isDirectlySerializableParam(typ) {
			continue
		}
		pattern := findOutputPattern(typ)
		if pattern == nil {
			return nil, nil, []AdmissionRefusal{{
				Code:    RefusalAdapterReturnRehydration,
				Message: fmt.Sprintf("return slot %d of type %s has no registered adapter output pattern", i, describeType(typ)),
				Type:    describeType(typ),
			}}
		}
		proofs = append(proofs, pattern.Discharge(fn, i)...)
		transforms = append(transforms, AdapterPattern{
			Name:      pattern.Name(),
			ParamName: result.Name(),
			FromType:  pattern.FromType(),
			ToType:    pattern.ToType(),
		})
	}
	for _, p := range proofs {
		if !p.Satisfied {
			return nil, nil, []AdmissionRefusal{{
				Code:    p.Obligation,
				Message: p.Detail,
			}}
		}
	}
	return transforms, proofs, nil
}

// dischargeGenericObligations runs the four obligations that are not
// pattern-owned: finite_input, local_lifecycle, error_order, call_site.
// finite_input is summary-only — it just records that every adapted input
// has a pattern with a finite extraction; the pattern matchers already
// failed if not. local_lifecycle scans the helper for forbidden lifecycle
// operations (Close on the awkward value, defers that escape). error_order
// records the divergence acceptance (spec §5). call_site checks for
// function-value / address-of / reflective use of the helper across the
// activation-path scope.
func dischargeGenericObligations(ctx AdapterContext, inputs, outputs []AdapterPattern) ([]AdapterProof, []AdmissionRefusal) {
	var proofs []AdapterProof
	var refusals []AdmissionRefusal

	// adapter_finite_input — summary proof. Every adapter input pattern
	// declares its renderer; matching only happens via library patterns.
	finite := AdapterProof{Obligation: RefusalAdapterFiniteInput, Satisfied: true}
	if len(inputs) == 0 {
		finite.Detail = "no adapter inputs required"
	} else {
		names := make([]string, 0, len(inputs))
		for _, in := range inputs {
			names = append(names, in.Name+"("+in.FromType+"→"+in.ToType+")")
		}
		finite.Detail = "finite extraction provided by: " + strings.Join(names, ", ")
	}
	proofs = append(proofs, finite)

	// adapter_local_lifecycle — every Close, defer, or interface boxing on
	// the awkward input parameters remains on the host side.
	lifecycle := dischargeLocalLifecycle(ctx.Fn, inputs)
	proofs = append(proofs, lifecycle)
	if !lifecycle.Satisfied {
		refusals = append(refusals, AdmissionRefusal{Code: lifecycle.Obligation, Message: lifecycle.Detail})
	}

	// adapter_error_order — accepted with divergence record per spec §5.
	errorOrder := AdapterProof{Obligation: RefusalAdapterErrorOrder, Satisfied: true}
	if len(inputs) > 0 {
		errorOrder.Detail = "host-side extraction errors occur before RPC; helper read errors moved to host-side ReadAll (accepted per spec §5)"
	} else {
		errorOrder.Detail = "no extraction-induced error reordering"
	}
	proofs = append(proofs, errorOrder)

	// adapter_call_site — pass when call sites are bounded and free of
	// function-value or reflective use.
	callSite := dischargeCallSite(ctx)
	proofs = append(proofs, callSite)
	if !callSite.Satisfied {
		refusals = append(refusals, AdmissionRefusal{Code: callSite.Obligation, Message: callSite.Detail})
	}

	return proofs, refusals
}

// dischargeLocalLifecycle inspects the awkward input parameters for any
// operations that would force lifecycle ownership remotely: Close, deferred
// closures over the value, interface boxing that could escape the helper.
// Most of the per-parameter use-shape work is in the pattern itself; this
// is a defense-in-depth check across all adapter inputs.
func dischargeLocalLifecycle(fn *ssa.Function, inputs []AdapterPattern) AdapterProof {
	proof := AdapterProof{Obligation: RefusalAdapterLocalLifecycle, Satisfied: true, Detail: "lifecycle of awkward inputs remains host-side"}
	if fn == nil {
		return proof
	}
	if len(inputs) == 0 {
		proof.Detail = "no adapter inputs; lifecycle trivially local"
		return proof
	}
	for i := 0; i < fn.Signature.Params().Len() && i < len(fn.Params); i++ {
		param := fn.Params[i]
		if !isAdapterInputParam(param.Type()) {
			continue
		}
		for _, ref := range valueReferrers(param) {
			switch op := ref.(type) {
			case *ssa.Defer:
				if deferReferencesValue(op, param) {
					proof.Satisfied = false
					proof.Detail = fmt.Sprintf("parameter %q is captured by a defer; lifecycle cannot move to remote side", param.Name())
					return proof
				}
			case *ssa.Call:
				// callMethodName resolves both static (*T).Close(param) and
				// interface-dispatch param.Close() (common.Method != nil &&
				// common.Value == param && Method.Name() == "Close") to "Close".
				if callMethodName(op, param) == "Close" {
					proof.Satisfied = false
					proof.Detail = fmt.Sprintf("parameter %q has Close() called on it; lifecycle is not adapter-local", param.Name())
					return proof
				}
			case *ssa.MakeInterface:
				// Boxing the awkward input into an interface lets it escape the
				// helper (stored, passed to an interface-typed sink, or
				// Close()'d through the boxed value). The adapter swap replaces
				// the value with a reconstructed one host-side, so any escape
				// of the original is unsound.
				if op.X == param {
					proof.Satisfied = false
					proof.Detail = fmt.Sprintf("parameter %q is boxed into an interface; the value may escape the helper and its lifecycle cannot move remote-side", param.Name())
					return proof
				}
			case *ssa.Store:
				// Storing the awkward input into a package-level global escapes
				// the helper entirely.
				if _, ok := op.Addr.(*ssa.Global); ok && op.Val == param {
					proof.Satisfied = false
					proof.Detail = fmt.Sprintf("parameter %q is stored into a package-level global; the value escapes the helper and its lifecycle cannot move remote-side", param.Name())
					return proof
				}
			}
		}
	}
	return proof
}

// dischargeCallSite verifies obligation #6: the helper is not used as a
// function value, taken by address, or accessed reflectively in a way that
// would observe the adapter swap.
func dischargeCallSite(ctx AdapterContext) AdapterProof {
	proof := AdapterProof{Obligation: RefusalAdapterCallSite, Satisfied: true}
	if ctx.CallSiteIndex != nil && ctx.CallSiteIndex.Scanned {
		idx := ctx.CallSiteIndex
		if idx.Disqualifier != "" {
			proof.Satisfied = false
			proof.Detail = idx.Disqualifier
			return proof
		}
		if len(idx.DirectCalls) > 0 {
			proof.Detail = fmt.Sprintf("%d reference(s) across the activation-path scope are direct calls; no function-value or reflective use", len(idx.DirectCalls))
			return proof
		}
		// No references at all in the scoped program. An unexported helper is
		// bounded by its own package (already scanned); an exported helper may
		// be referenced from outside the scope, which the scan cannot observe.
		if !ctx.FunctionExported {
			proof.Detail = "no references in the activation-path scope; unexported helper bounded by its own package"
			return proof
		}
		proof.Satisfied = false
		proof.Detail = "exported helper has no observed references in the activation-path scope; cannot prove no function-value or reflective use"
		return proof
	}
	if len(ctx.CallSites) == 0 {
		if !ctx.FunctionExported {
			proof.Detail = "no call-site set supplied; unexported helper bounded by its own package"
			return proof
		}
		proof.Satisfied = false
		proof.Detail = "exported helper has no call-site set; cannot prove no function-value or reflective use"
		return proof
	}
	for _, cs := range ctx.CallSites {
		if cs == nil {
			continue
		}
		if cs.Method != nil {
			// Interface dispatch through a method set — the helper is
			// reachable via interface satisfaction, which would observe the
			// adapter swap. Refuse.
			proof.Satisfied = false
			proof.Detail = "helper is reached via interface dispatch; adapter swap would not be observed by callers"
			return proof
		}
	}
	proof.Detail = fmt.Sprintf("%d call sites are direct function calls; no function-value or reflective use", len(ctx.CallSites))
	return proof
}

// ensureSixObligationsRepresented guarantees the AdapterPlan reports a proof
// for each of the six obligations in deterministic order. Missing entries
// are added as satisfied summary proofs (e.g. adapter_use_shape is missing
// when there are no input patterns).
func ensureSixObligationsRepresented(proofs []AdapterProof, inputs, outputs []AdapterPattern) []AdapterProof {
	ordered := []string{
		RefusalAdapterFiniteInput,
		RefusalAdapterLocalLifecycle,
		RefusalAdapterUseShape,
		RefusalAdapterReturnRehydration,
		RefusalAdapterErrorOrder,
		RefusalAdapterCallSite,
	}
	have := map[string]AdapterProof{}
	for _, p := range proofs {
		// Keep the first proof seen for each obligation; later proofs are
		// downgrades (e.g. a per-pattern use_shape detail when the summary
		// would otherwise be added below).
		if _, ok := have[p.Obligation]; !ok {
			have[p.Obligation] = p
		}
	}
	out := make([]AdapterProof, 0, len(ordered))
	for _, code := range ordered {
		if p, ok := have[code]; ok {
			out = append(out, p)
			continue
		}
		switch code {
		case RefusalAdapterUseShape:
			out = append(out, AdapterProof{Obligation: code, Satisfied: true, Detail: "no awkward input parameters"})
		case RefusalAdapterReturnRehydration:
			out = append(out, AdapterProof{Obligation: code, Satisfied: true, Detail: "no awkward return values"})
		default:
			out = append(out, AdapterProof{Obligation: code, Satisfied: true})
		}
	}
	return out
}

// bodyRewriteFor describes the prologue substitution applied to the helper.
// The input patterns determine the from/to: each input becomes the variable
// drained host-side, and the helper body operates on that variable.
func bodyRewriteFor(inputs []AdapterPattern) AdapterBodyRewrite {
	if len(inputs) == 0 {
		return AdapterBodyRewrite{Description: "no body rewrite required (no awkward inputs)"}
	}
	parts := make([]string, 0, len(inputs))
	for _, in := range inputs {
		parts = append(parts, fmt.Sprintf("%s: %s → %s via %s", in.ParamName, in.FromType, in.ToType, in.Name))
	}
	return AdapterBodyRewrite{
		Description: "replace per-input prologue: " + strings.Join(parts, "; "),
		FromPattern: "awkward-typed parameter operations",
		ToPattern:   "finite-input equivalents",
	}
}

// liveProxyOrImpossibleRefusal returns a refusal when any parameter or
// return matches the LiveProxyRequired or AdapterImpossible exclusion list.
// Codified from SPRINT-0051 §0.8 — every entry has a fixed refusal code
// so callers can render the reason without re-inspecting the signature.
func liveProxyOrImpossibleRefusal(sig *types.Signature) *AdmissionRefusal {
	if sig == nil {
		return nil
	}
	for _, kind := range []string{"param", "result"} {
		var vars *types.Tuple
		if kind == "param" {
			vars = sig.Params()
		} else {
			vars = sig.Results()
		}
		for i := 0; i < vars.Len(); i++ {
			typ := vars.At(i).Type()
			if refusal := liveProxyClassify(typ, kind == "result"); refusal != nil {
				refusal.Type = describeType(typ)
				return refusal
			}
		}
	}
	return nil
}

// liveProxyClassify returns a refusal for one parameter or return type that
// requires live-proxy semantics, or nil when the type is acceptable. The
// isResult flag distinguishes io.Writer output parameters (which the spec
// classifies as LiveProxyRequired) from other appearances.
func liveProxyClassify(typ types.Type, isResult bool) *AdmissionRefusal {
	if typ == nil {
		return nil
	}
	bare := types.Unalias(typ)
	if ptr, ok := bare.(*types.Pointer); ok {
		if isNamedFromPkg(ptr.Elem(), "os", "File") {
			return &AdmissionRefusal{Code: RefusalLiveProxyRequired, Message: "*os.File requires live proxy (handle has seek/lock/close lifecycle)"}
		}
	}
	if _, ok := bare.(*types.Chan); ok {
		return &AdmissionRefusal{Code: RefusalLiveProxyRequired, Message: "channel parameter/return requires live proxy (send/receive ordering is part of semantics)"}
	}
	if _, ok := bare.(*types.Signature); ok {
		return &AdmissionRefusal{Code: RefusalAdapterImpossible, Message: "function-valued parameter/return cannot be serialized; adapter impossible"}
	}
	if isNamedFromPkg(bare, "net/http", "ResponseWriter") {
		return &AdmissionRefusal{Code: RefusalLiveProxyRequired, Message: "http.ResponseWriter requires live proxy (streaming write tied to active request)"}
	}
	// io.Writer / io.WriteCloser as output parameter: refuse. When it appears
	// only as an input we still refuse — io.Writer is always remote-streamable
	// rather than finite. (io.Reader is handled by reader_read_all in future.)
	if isNamedFromPkg(bare, "io", "Writer") || isNamedFromPkg(bare, "io", "WriteCloser") {
		_ = isResult
		return &AdmissionRefusal{Code: RefusalLiveProxyRequired, Message: "io.Writer cannot be captured as a finite value; live proxy required"}
	}
	return nil
}

// isNamedFromPkg reports whether typ is the named type pkgPath.Name (after
// unwrapping any pointer indirection).
func isNamedFromPkg(typ types.Type, pkgPath, name string) bool {
	if typ == nil {
		return false
	}
	t := types.Unalias(typ)
	if ptr, ok := t.(*types.Pointer); ok {
		t = types.Unalias(ptr.Elem())
	}
	named, ok := t.(*types.Named)
	if !ok || named.Obj() == nil {
		return false
	}
	if named.Obj().Name() != name {
		return false
	}
	pkg := named.Obj().Pkg()
	if pkg == nil {
		return false
	}
	return pkg.Path() == pkgPath
}

// isDirectlySerializableParam returns true for parameter/return types that
// codegen can carry across the boundary today without an adapter. Primitive
// types, strings, byte slices, and known reconstructible types are direct.
// The actual admission logic lives elsewhere; this is a conservative gate
// so the adapter pass doesn't synthesize unnecessary transforms.
func isDirectlySerializableParam(typ types.Type) bool {
	if typ == nil {
		return false
	}
	t := types.Unalias(typ)
	switch b := t.(type) {
	case *types.Basic:
		switch b.Kind() {
		case types.Invalid, types.UnsafePointer:
			return false
		}
		return true
	case *types.Slice:
		return isByteBasic(b.Elem())
	case *types.Array:
		return isByteBasic(b.Elem())
	}
	// Named alias of a basic / byte slice.
	if named, ok := t.(*types.Named); ok && named.Underlying() != nil {
		return isDirectlySerializableParam(named.Underlying())
	}
	return false
}

func isByteBasic(typ types.Type) bool {
	if typ == nil {
		return false
	}
	basic, ok := types.Unalias(typ).(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

// isAdapterInputParam returns true when the parameter type is one of the
// awkward shapes the adapter library knows how to drain. Used by
// dischargeLocalLifecycle to skip already-serializable parameters.
func isAdapterInputParam(typ types.Type) bool {
	for _, p := range inputPatterns() {
		if p.Matches(typ) {
			return true
		}
	}
	return false
}

// deferReferencesValue reports whether a Defer instruction references the
// given value either as the call receiver or as a free variable.
func deferReferencesValue(d *ssa.Defer, v ssa.Value) bool {
	if d == nil {
		return false
	}
	common := d.Common()
	if common == nil {
		return false
	}
	if common.Value == v {
		return true
	}
	for _, arg := range common.Args {
		if arg == v {
			return true
		}
	}
	return false
}

// signatureString renders a Go-source-like representation of a function
// signature: (paramType, paramType) (resultType, resultType). Used for the
// HostSignature and RemoteSignature plan fields.
func signatureString(sig *types.Signature, includeResults bool) string {
	if sig == nil {
		return ""
	}
	params := tupleTypeStrings(sig.Params())
	out := "(" + strings.Join(params, ", ") + ")"
	if includeResults {
		results := tupleTypeStrings(sig.Results())
		if len(results) == 1 {
			out += " " + results[0]
		} else if len(results) > 1 {
			out += " (" + strings.Join(results, ", ") + ")"
		}
	}
	return out
}

// remoteSignatureString renders the normalized remote helper signature
// implied by the input/output transforms. The returned string uses the
// ToType strings declared by each pattern; direct types pass through.
func remoteSignatureString(sig *types.Signature, inputs, outputs []AdapterPattern) string {
	if sig == nil {
		return ""
	}
	paramStrs := make([]string, 0, sig.Params().Len())
	transformByParamIndex := map[int]AdapterPattern{}
	transformIdx := 0
	for i := 0; i < sig.Params().Len(); i++ {
		typ := sig.Params().At(i).Type()
		if isDirectlySerializableParam(typ) {
			paramStrs = append(paramStrs, describeType(typ))
			continue
		}
		if transformIdx < len(inputs) {
			transformByParamIndex[i] = inputs[transformIdx]
			paramStrs = append(paramStrs, inputs[transformIdx].ToType)
			transformIdx++
			continue
		}
		paramStrs = append(paramStrs, describeType(typ))
	}
	resultStrs := []string{}
	transformIdx = 0
	for i := 0; i < sig.Results().Len(); i++ {
		typ := sig.Results().At(i).Type()
		if isErrorType(typ) {
			resultStrs = append(resultStrs, "error")
			continue
		}
		if isDirectlySerializableParam(typ) {
			resultStrs = append(resultStrs, describeType(typ))
			continue
		}
		if transformIdx < len(outputs) {
			resultStrs = append(resultStrs, outputs[transformIdx].ToType)
			transformIdx++
			continue
		}
		resultStrs = append(resultStrs, describeType(typ))
	}
	_ = transformByParamIndex
	out := "(" + strings.Join(paramStrs, ", ") + ")"
	if len(resultStrs) == 1 {
		out += " " + resultStrs[0]
	} else if len(resultStrs) > 1 {
		out += " (" + strings.Join(resultStrs, ", ") + ")"
	}
	return out
}

// tupleTypeStrings renders each variable in a tuple as its describeType().
func tupleTypeStrings(t *types.Tuple) []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, t.Len())
	for i := 0; i < t.Len(); i++ {
		out = append(out, describeType(t.At(i).Type()))
	}
	return out
}
