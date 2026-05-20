package codegen

import (
	"fmt"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// CallSiteIndex records the result of the reverse-import scan for references
// to the adapter helper across the activation-path package set. It backs the
// adapter_call_site obligation: the adapter swap replaces the helper with a
// remote call, which is sound only when every reference is a direct static
// call. Any other reference (closure capture, stored/passed function value,
// reflective use, goroutine/defer dispatch) would observe the swap and is
// disqualifying.
//
// Scanned distinguishes "scan ran and found nothing disqualifying" from "no
// scan supplied"; the latter falls back to the exported/unexported heuristic
// in dischargeCallSite. The scan names no target and no pattern.
type CallSiteIndex struct {
	Scanned      bool
	DirectCalls  []*ssa.CallCommon
	Disqualifier string
}

type helperRefKind int

const (
	helperRefNone helperRefKind = iota
	helperRefDirectCall
	helperRefDisqualified
)

// buildCallSiteIndex walks every function reachable in the helper's SSA
// program (which, in production, spans the reverse-import scope loaded for the
// candidate) and classifies each reference to fn. The first disqualifying
// reference short-circuits the scan — a single function-value use is enough to
// refuse the obligation.
func buildCallSiteIndex(fn *ssa.Function) *CallSiteIndex {
	idx := &CallSiteIndex{Scanned: true}
	if fn == nil || fn.Prog == nil {
		return idx
	}
	for caller := range ssautil.AllFunctions(fn.Prog) {
		if caller == nil {
			continue
		}
		for _, block := range caller.Blocks {
			for _, instr := range block.Instrs {
				switch reason, kind := classifyHelperReference(fn, instr); kind {
				case helperRefDirectCall:
					if call, ok := instr.(*ssa.Call); ok {
						idx.DirectCalls = append(idx.DirectCalls, &call.Call)
					}
				case helperRefDisqualified:
					idx.Disqualifier = reason
					return idx
				}
			}
		}
	}
	return idx
}

// classifyHelperReference reports how instr references fn: a direct static
// call (sound), a disqualifying function-value reference, or no reference.
func classifyHelperReference(fn *ssa.Function, instr ssa.Instruction) (string, helperRefKind) {
	if call, ok := instr.(*ssa.Call); ok {
		if call.Call.Value == fn && !call.Call.IsInvoke() && !ssaArgsContain(call.Call.Args, fn) {
			return "", helperRefDirectCall
		}
	}
	if !instrReferencesValue(instr, fn) {
		return "", helperRefNone
	}
	return helperReferenceReason(instr, fn), helperRefDisqualified
}

func ssaArgsContain(args []ssa.Value, target ssa.Value) bool {
	for _, a := range args {
		if a == target {
			return true
		}
	}
	return false
}

func instrReferencesValue(instr ssa.Instruction, target ssa.Value) bool {
	for _, op := range instr.Operands(nil) {
		if op != nil && *op == target {
			return true
		}
	}
	return false
}

func helperReferenceReason(instr ssa.Instruction, fn *ssa.Function) string {
	name := fn.Name()
	switch op := instr.(type) {
	case *ssa.MakeClosure:
		return fmt.Sprintf("helper %q is captured by a closure; a remote adapter swap would not be observed at the closure call site", name)
	case *ssa.Call:
		if callee := staticCalleeName(op.Call); callee != "" {
			return fmt.Sprintf("helper %q is passed as a function value to %s; higher-order or reflective use would not observe the adapter swap", name, callee)
		}
		return fmt.Sprintf("helper %q is passed as a function value; higher-order use would not observe the adapter swap", name)
	case *ssa.Go:
		return fmt.Sprintf("helper %q is dispatched in a goroutine, not called directly; lifecycle and ordering cannot be proven across the boundary", name)
	case *ssa.Defer:
		return fmt.Sprintf("helper %q is deferred, not called directly; ordering cannot be proven across the boundary", name)
	case *ssa.Store:
		return fmt.Sprintf("helper %q is stored as a function value; a later indirect call would not observe the adapter swap", name)
	default:
		return fmt.Sprintf("helper %q is referenced as a function value (non-call use); the adapter swap would not be observed", name)
	}
}

func staticCalleeName(c ssa.CallCommon) string {
	if c.IsInvoke() {
		return ""
	}
	if callee := c.StaticCallee(); callee != nil {
		return callee.String()
	}
	return ""
}
