// Boundary-adapter pattern library (SPRINT-0051 Phase 3).
//
// Each pattern recognizes one shape of awkward boundary value (input or
// return), proves its pattern-specific obligations against the helper SSA,
// and renders the host-side extraction or rehydration code. The library is
// closed by design: new shapes are added by registering a new pattern, not
// by generalizing existing ones. See docs/research/activation-paths/
// boundary-adapter-strategy.md §"Pattern Library".
package codegen

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ssa"
)

// AdapterDirection identifies whether a pattern transforms an input
// parameter (host-side extraction before RPC) or a return value (host-side
// rehydration after RPC).
type AdapterDirection string

const (
	AdapterInputDir  AdapterDirection = "input"
	AdapterOutputDir AdapterDirection = "output"
)

// AdapterPatternImpl is the in-process interface for one adapter pattern.
// The serialized form used in plans/manifests is the plain AdapterPattern
// struct in types.go; the interface here owns the matching and proof logic.
type AdapterPatternImpl interface {
	// Name returns the pattern identifier (e.g. "multipart_file_read_all").
	Name() string

	// Direction reports whether this is an input or output pattern.
	Direction() AdapterDirection

	// FromType is the awkward source type the pattern matches.
	FromType() string

	// ToType is the normalized network type the pattern produces.
	ToType() string

	// Matches reports whether the pattern applies to a given Go type. Pure
	// type predicate — does not look at SSA. Returns true even when the
	// associated proofs may later refuse.
	Matches(typ types.Type) bool

	// Discharge runs pattern-specific proof obligations against the helper
	// SSA. The slot index is the parameter index for input patterns or the
	// return-slot index for output patterns. The returned proofs cover the
	// obligations this pattern owns (typically adapter_use_shape for input
	// patterns, adapter_return_rehydration for output patterns) — generic
	// obligations like adapter_call_site are discharged in adapter_pass.go.
	Discharge(fn *ssa.Function, slotIndex int) []AdapterProof

	// RenderInputExtraction returns the Go statements that drain the awkward
	// input into a finite value. Only meaningful for input patterns. The
	// caller passes the inbound parameter name, the desired output variable
	// name, and a printf-style template for returning the wrapper's zero
	// tuple with an error (e.g. "return nil, 0, 0, %s"). Phase 4 wires this
	// into the host wrapper template.
	RenderInputExtraction(inVar, outVar, errReturnFmt string) []string

	// RenderRemoteReconstruction returns a single Go expression that rebuilds
	// the awkward return value from the DTO field. Only meaningful for output
	// patterns. The argument is the Go expression referencing the field on
	// the DTO (e.g. "out.Thumbnail"). Phase 4 wires this into the host
	// wrapper template.
	RenderRemoteReconstruction(remoteFieldExpr string) string
}

// adapterPatternRegistry is the closed library of patterns the compiler
// will try in order. Insertion order is deterministic — first match wins
// for a given parameter/return slot. New patterns are added here.
var adapterPatternRegistry = []AdapterPatternImpl{
	multipartFileReadAllPattern{},
	bytesReaderReturnPattern{},
}

// inputPatterns returns the patterns whose direction is input.
func inputPatterns() []AdapterPatternImpl {
	out := make([]AdapterPatternImpl, 0, len(adapterPatternRegistry))
	for _, p := range adapterPatternRegistry {
		if p.Direction() == AdapterInputDir {
			out = append(out, p)
		}
	}
	return out
}

// outputPatterns returns the patterns whose direction is output.
func outputPatterns() []AdapterPatternImpl {
	out := make([]AdapterPatternImpl, 0, len(adapterPatternRegistry))
	for _, p := range adapterPatternRegistry {
		if p.Direction() == AdapterOutputDir {
			out = append(out, p)
		}
	}
	return out
}

// findInputPattern returns the first pattern that matches the given
// parameter type, or nil if none apply.
func findInputPattern(typ types.Type) AdapterPatternImpl {
	for _, p := range inputPatterns() {
		if p.Matches(typ) {
			return p
		}
	}
	return nil
}

// findOutputPattern returns the first pattern that matches the given
// return type, or nil if none apply.
func findOutputPattern(typ types.Type) AdapterPatternImpl {
	for _, p := range outputPatterns() {
		if p.Matches(typ) {
			return p
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// multipart_file_read_all: *multipart.FileHeader -> []byte
// ---------------------------------------------------------------------------

type multipartFileReadAllPattern struct{}

func (multipartFileReadAllPattern) Name() string                { return "multipart_file_read_all" }
func (multipartFileReadAllPattern) Direction() AdapterDirection { return AdapterInputDir }
func (multipartFileReadAllPattern) FromType() string            { return "*multipart.FileHeader" }
func (multipartFileReadAllPattern) ToType() string              { return "[]byte" }

func (multipartFileReadAllPattern) Matches(typ types.Type) bool {
	return namedPointerMatches(typ, "mime/multipart", "FileHeader")
}

// Discharge verifies the use-shape obligation: the parameter is referenced
// only via a single Open() then Read-style operation. Refuse on multiple
// Open calls, filename/header/size access, or any mutation/aliasing escape.
func (p multipartFileReadAllPattern) Discharge(fn *ssa.Function, paramIndex int) []AdapterProof {
	useShape := AdapterProof{Obligation: RefusalAdapterUseShape, Satisfied: true}
	if fn == nil || paramIndex < 0 || paramIndex >= len(fn.Params) {
		useShape.Satisfied = false
		useShape.Detail = "missing helper SSA for use-shape verification"
		return []AdapterProof{useShape}
	}
	param := fn.Params[paramIndex]
	openCalls := 0
	for _, ref := range valueReferrers(param) {
		switch op := ref.(type) {
		case *ssa.Call:
			method := callMethodName(op, param)
			switch method {
			case "Open":
				openCalls++
			case "Filename", "Header", "Size":
				useShape.Satisfied = false
				useShape.Detail = fmt.Sprintf("helper references %s on *multipart.FileHeader; only Open() is permitted", method)
				return []AdapterProof{useShape}
			default:
				useShape.Satisfied = false
				useShape.Detail = fmt.Sprintf("helper calls unsupported method %q on *multipart.FileHeader", method)
				return []AdapterProof{useShape}
			}
		case *ssa.FieldAddr, *ssa.Field:
			useShape.Satisfied = false
			useShape.Detail = "helper accesses field on *multipart.FileHeader; only Open() is permitted"
			return []AdapterProof{useShape}
		case *ssa.Store:
			useShape.Satisfied = false
			useShape.Detail = "helper stores into *multipart.FileHeader; mutation is not permitted"
			return []AdapterProof{useShape}
		case *ssa.MakeInterface, *ssa.ChangeType, *ssa.ChangeInterface:
			useShape.Satisfied = false
			useShape.Detail = "helper boxes *multipart.FileHeader into an interface; the value would escape the adapter"
			return []AdapterProof{useShape}
		}
	}
	if openCalls == 0 {
		useShape.Satisfied = false
		useShape.Detail = "helper does not call Open() on *multipart.FileHeader; cannot drain to []byte"
		return []AdapterProof{useShape}
	}
	if openCalls > 1 {
		useShape.Satisfied = false
		useShape.Detail = fmt.Sprintf("helper calls Open() %d times on *multipart.FileHeader; the adapter drains the file exactly once", openCalls)
		return []AdapterProof{useShape}
	}
	useShape.Detail = "helper opens *multipart.FileHeader exactly once and reads it"
	return []AdapterProof{useShape}
}

// rewriteInputBody removes the awkward-input prologue for the
// multipart_file_read_all pattern and rewrites uses of the opened reader to
// reference the normalized []byte input. The matched prologue is:
//
//	<readerVar>, <errVar> := <paramName>.Open()
//	if <errVar> != nil { ... }
//	defer <readerVar>.Close()    // anywhere in the block
//
// Uses of <readerVar> are replaced with bytes.NewReader(<normName>). Returns
// false (refusing, never partial-applying) when the prologue shape does not
// match — the Discharge proof guarantees the shape for accepted plans, so a
// false here is a genuine codegen mismatch the caller surfaces as an error.
func (multipartFileReadAllPattern) rewriteInputBody(body *ast.BlockStmt, paramName, normName string) bool {
	openIdx, readerVar, errVar := findReceiverMethodAssignment(body, paramName, "Open")
	if openIdx < 0 || readerVar == "" || errVar == "" {
		return false
	}
	if openIdx+1 >= len(body.List) || !isErrNilGuard(body.List[openIdx+1], errVar) {
		return false
	}
	drop := map[int]bool{openIdx: true, openIdx + 1: true}
	if deferIdx := findDeferClose(body, readerVar); deferIdx >= 0 {
		drop[deferIdx] = true
	}
	kept := make([]ast.Stmt, 0, len(body.List))
	for i, stmt := range body.List {
		if drop[i] {
			continue
		}
		kept = append(kept, stmt)
	}
	body.List = kept
	return replaceIdentUses(body, readerVar, func() ast.Expr {
		return &ast.CallExpr{
			Fun:  &ast.SelectorExpr{X: ast.NewIdent("bytes"), Sel: ast.NewIdent("NewReader")},
			Args: []ast.Expr{ast.NewIdent(normName)},
		}
	})
}

func (multipartFileReadAllPattern) RenderInputExtraction(inVar, outVar, errReturnFmt string) []string {
	srcVar := inVar + "Src"
	return []string{
		fmt.Sprintf("%s, err := %s.Open()", srcVar, inVar),
		fmt.Sprintf("if err != nil { " + errReturnFmt + " }", "err"),
		fmt.Sprintf("defer %s.Close()", srcVar),
		fmt.Sprintf("%s, err := io.ReadAll(%s)", outVar, srcVar),
		fmt.Sprintf("if err != nil { " + errReturnFmt + " }", "err"),
	}
}

func (multipartFileReadAllPattern) RenderRemoteReconstruction(remoteFieldExpr string) string {
	return remoteFieldExpr
}

// ---------------------------------------------------------------------------
// bytes_reader_return: []byte -> *bytes.Reader
// ---------------------------------------------------------------------------

type bytesReaderReturnPattern struct{}

func (bytesReaderReturnPattern) Name() string                { return "bytes_reader_return" }
func (bytesReaderReturnPattern) Direction() AdapterDirection { return AdapterOutputDir }
func (bytesReaderReturnPattern) FromType() string            { return "*bytes.Reader" }
func (bytesReaderReturnPattern) ToType() string              { return "[]byte" }

func (bytesReaderReturnPattern) Matches(typ types.Type) bool {
	return namedPointerMatches(typ, "bytes", "Reader")
}

// Discharge verifies the return-rehydration obligation: every return of
// *bytes.Reader in the helper is produced by bytes.NewReader on a []byte
// expression. Other producers (e.g. nil, a stored field, a different
// constructor) make the rehydration ambiguous and the pattern refuses.
func (bytesReaderReturnPattern) Discharge(fn *ssa.Function, resultIndex int) []AdapterProof {
	rehydration := AdapterProof{Obligation: RefusalAdapterReturnRehydration, Satisfied: true}
	if fn == nil || resultIndex < 0 {
		rehydration.Satisfied = false
		rehydration.Detail = "missing helper SSA for return-rehydration verification"
		return []AdapterProof{rehydration}
	}
	if fn.Signature == nil || resultIndex >= fn.Signature.Results().Len() {
		rehydration.Satisfied = false
		rehydration.Detail = fmt.Sprintf("result index %d out of range for helper signature", resultIndex)
		return []AdapterProof{rehydration}
	}
	producers := returnValueProducers(fn, resultIndex)
	if len(producers) == 0 {
		rehydration.Satisfied = false
		rehydration.Detail = "no return instructions found; cannot verify *bytes.Reader rehydration"
		return []AdapterProof{rehydration}
	}
	for _, prod := range producers {
		if !isBytesNewReaderCall(prod) {
			rehydration.Satisfied = false
			rehydration.Detail = fmt.Sprintf("helper returns *bytes.Reader from an unrecognized producer (%T); only bytes.NewReader is rehydratable", prod)
			return []AdapterProof{rehydration}
		}
	}
	rehydration.Detail = "helper returns *bytes.Reader exclusively via bytes.NewReader"
	return []AdapterProof{rehydration}
}

// rewriteOutputBody rewrites every return slot of the form bytes.NewReader(X)
// into X for the bytes_reader_return pattern, so the normalized helper returns
// the finite []byte the DTO carries. Returns false when no such return slot is
// found — the Discharge proof guarantees the producer shape for accepted
// plans, so a false here is a genuine codegen mismatch.
func (bytesReaderReturnPattern) rewriteOutputBody(body *ast.BlockStmt) bool {
	replaced := false
	astutil.Apply(body, func(c *astutil.Cursor) bool {
		ret, ok := c.Node().(*ast.ReturnStmt)
		if !ok {
			return true
		}
		for i, res := range ret.Results {
			if inner, ok := bytesNewReaderArg(res); ok {
				ret.Results[i] = inner
				replaced = true
			}
		}
		return true
	}, nil)
	return replaced
}

func (bytesReaderReturnPattern) RenderInputExtraction(string, string, string) []string {
	return nil
}

func (bytesReaderReturnPattern) RenderRemoteReconstruction(remoteFieldExpr string) string {
	return "bytes.NewReader(" + remoteFieldExpr + ")"
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// namedPointerMatches returns true when typ is *pkg.Name for the given
// package path and type name. Pointer indirection is required (the patterns
// matched by SPRINT-0051 are all *T shapes).
func namedPointerMatches(typ types.Type, pkgPath, name string) bool {
	if typ == nil {
		return false
	}
	ptr, ok := types.Unalias(typ).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(ptr.Elem()).(*types.Named)
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

// valueReferrers returns the SSA instructions that reference the given
// value (parameters and returns expose Referrers via the Value interface).
func valueReferrers(v ssa.Value) []ssa.Instruction {
	if v == nil {
		return nil
	}
	refs := v.Referrers()
	if refs == nil {
		return nil
	}
	out := make([]ssa.Instruction, 0, len(*refs))
	for _, inst := range *refs {
		if inst == nil {
			continue
		}
		out = append(out, inst)
	}
	return out
}

// callMethodName returns the method or function name being invoked when
// `recv` is the receiver of a Call. Returns "" when the call is not a
// recognized method invocation on recv.
func callMethodName(call *ssa.Call, recv ssa.Value) string {
	if call == nil {
		return ""
	}
	common := call.Common()
	if common == nil {
		return ""
	}
	// Method on a concrete type: recv is Args[0] of an invoked function.
	if common.Method != nil {
		if common.Value == recv {
			return common.Method.Name()
		}
		return ""
	}
	// Direct function call: the receiver may show up as the first arg of
	// a method expression like (*multipart.FileHeader).Open(recv).
	if fn, ok := common.Value.(*ssa.Function); ok && fn != nil && fn.Signature != nil {
		// Selector-based methods land here when SSA inlines (*T).M(recv).
		if recv != nil && len(common.Args) > 0 && common.Args[0] == recv {
			return fn.Name()
		}
		// Also handle the case where recv is bound as the Value of a Call
		// constructed from a selector like recv.Method(...) — this shows up
		// as common.Value being the function and recv being the first arg.
	}
	return ""
}

// returnValueProducers returns the SSA values that flow into the given
// return-slot index for the function. Walks every Return instruction and,
// when the slot value is a UnOp(*ssa.Alloc) load, follows the Stores into
// that Alloc and records each Store's stored value (skipping nil stores —
// those are the error-path zero values). The returned producers are the
// concrete defining instructions (e.g. *ssa.Call for bytes.NewReader).
func returnValueProducers(fn *ssa.Function, resultIndex int) []ssa.Value {
	if fn == nil {
		return nil
	}
	var out []ssa.Value
	seen := map[ssa.Value]bool{}
	add := func(v ssa.Value) {
		if v == nil || seen[v] {
			return
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, inst := range block.Instrs {
			ret, ok := inst.(*ssa.Return)
			if !ok {
				continue
			}
			if resultIndex >= len(ret.Results) {
				continue
			}
			for _, prod := range resolveProducers(ret.Results[resultIndex]) {
				add(prod)
			}
		}
	}
	return out
}

// resolveProducers walks the SSA def chain to surface the concrete
// instruction(s) that produced the value `v`. The interesting case is when
// SSA materializes return-slot temporaries as Alloc/Store/UnOp(load) — we
// follow the Alloc's referrers to find the Store(s) and recurse on the
// stored value. Constant nil stores are filtered (those are error-path
// zeros). Other shapes (Phi, Extract) are flattened into their operands.
func resolveProducers(v ssa.Value) []ssa.Value {
	if v == nil {
		return nil
	}
	switch op := v.(type) {
	case *ssa.UnOp:
		// Dereference of an Alloc: follow stores into the alloc.
		if alloc, ok := op.X.(*ssa.Alloc); ok {
			return storesIntoAlloc(alloc)
		}
	case *ssa.Phi:
		var out []ssa.Value
		for _, edge := range op.Edges {
			out = append(out, resolveProducers(edge)...)
		}
		return out
	case *ssa.Extract:
		// Extract from a tuple; the producer is whatever the tuple's call is.
		return []ssa.Value{op.Tuple}
	}
	return []ssa.Value{v}
}

// storesIntoAlloc returns the non-nil values stored into the given Alloc.
// A nil store is treated as an error-path zero and excluded so the
// pattern's producer check sees only "real" returns.
func storesIntoAlloc(alloc *ssa.Alloc) []ssa.Value {
	if alloc == nil {
		return nil
	}
	refs := alloc.Referrers()
	if refs == nil {
		return nil
	}
	var out []ssa.Value
	for _, inst := range *refs {
		store, ok := inst.(*ssa.Store)
		if !ok {
			continue
		}
		if store.Addr != alloc {
			continue
		}
		if isConstNil(store.Val) {
			continue
		}
		out = append(out, resolveProducers(store.Val)...)
	}
	return out
}

// isConstNil reports whether v is the typed constant nil.
func isConstNil(v ssa.Value) bool {
	c, ok := v.(*ssa.Const)
	if !ok {
		return false
	}
	return c.IsNil()
}

// isBytesNewReaderCall returns true when v is a *ssa.Call whose target is
// bytes.NewReader. Const nil and other constructors return false. This is
// the canonical rehydration shape for the bytes_reader_return pattern.
func isBytesNewReaderCall(v ssa.Value) bool {
	call, ok := v.(*ssa.Call)
	if !ok {
		return false
	}
	common := call.Common()
	if common == nil {
		return false
	}
	fn, ok := common.Value.(*ssa.Function)
	if !ok || fn == nil {
		return false
	}
	if fn.Name() != "NewReader" {
		return false
	}
	pkg := fn.Pkg
	if pkg == nil || pkg.Pkg == nil {
		return false
	}
	return pkg.Pkg.Path() == "bytes"
}

// typeIsByteSliceFlow reports whether the given value's type is []byte. Used
// by adapter_pass.go to confirm that bytes.NewReader is producing from a
// finite byte slice rather than (e.g.) an io.ReadSeeker.
func typeIsByteSliceFlow(v ssa.Value) bool {
	if v == nil || v.Type() == nil {
		return false
	}
	slice, ok := types.Unalias(v.Type()).(*types.Slice)
	if !ok {
		return false
	}
	basic, ok := types.Unalias(slice.Elem()).(*types.Basic)
	return ok && basic.Kind() == types.Uint8
}

// inputBodyRewriter is implemented by input patterns whose normalized helper
// body differs structurally from the original — they remove the awkward-input
// prologue and rewrite uses of the awkward parameter to reference the
// normalized value. Patterns that need no body surgery (the normalized input
// is used identically) do not implement this interface.
type inputBodyRewriter interface {
	rewriteInputBody(body *ast.BlockStmt, paramName, normName string) bool
}

// outputBodyRewriter is implemented by output patterns whose normalized helper
// body must rewrite return statements producing the awkward return value into
// ones producing the normalized value.
type outputBodyRewriter interface {
	rewriteOutputBody(body *ast.BlockStmt) bool
}

// findReceiverMethodAssignment finds a top-level statement of the form
// `<lhs0>, <lhs1> := <recvName>.<method>()` in the block. Returns the
// statement index plus the two LHS identifier names, or (-1, "", "").
func findReceiverMethodAssignment(body *ast.BlockStmt, recvName, method string) (int, string, string) {
	for i, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 2 || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != method {
			continue
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != recvName {
			continue
		}
		lhs0, ok0 := assign.Lhs[0].(*ast.Ident)
		lhs1, ok1 := assign.Lhs[1].(*ast.Ident)
		if !ok0 || !ok1 {
			continue
		}
		return i, lhs0.Name, lhs1.Name
	}
	return -1, "", ""
}

// isErrNilGuard reports whether stmt is `if <errVar> != nil { ... }` with no
// init clause.
func isErrNilGuard(stmt ast.Stmt, errVar string) bool {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Init != nil {
		return false
	}
	bin, ok := ifStmt.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return false
	}
	left, ok := bin.X.(*ast.Ident)
	if !ok || left.Name != errVar {
		return false
	}
	right, ok := bin.Y.(*ast.Ident)
	return ok && right.Name == "nil"
}

// findDeferClose returns the index of a top-level `defer <recvName>.Close()`
// statement, or -1.
func findDeferClose(body *ast.BlockStmt, recvName string) int {
	for i, stmt := range body.List {
		def, ok := stmt.(*ast.DeferStmt)
		if !ok {
			continue
		}
		sel, ok := def.Call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Close" {
			continue
		}
		if recv, ok := sel.X.(*ast.Ident); ok && recv.Name == recvName {
			return i
		}
	}
	return -1
}

// replaceIdentUses replaces every *ast.Ident named name within root with a
// freshly-built expression. Returns true if at least one was replaced.
func replaceIdentUses(root ast.Node, name string, build func() ast.Expr) bool {
	replaced := false
	astutil.Apply(root, func(c *astutil.Cursor) bool {
		id, ok := c.Node().(*ast.Ident)
		if !ok || id.Name != name {
			return true
		}
		c.Replace(build())
		replaced = true
		return true
	}, nil)
	return replaced
}

// bytesNewReaderArg returns the single argument X when expr is the call
// bytes.NewReader(X), reporting ok=false otherwise.
func bytesNewReaderArg(expr ast.Expr) (ast.Expr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NewReader" {
		return nil, false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "bytes" {
		return nil, false
	}
	return call.Args[0], true
}

// describeType returns a short type description for error messages.
func describeType(typ types.Type) string {
	if typ == nil {
		return "<nil>"
	}
	s := types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		// Use just the package name for readability (e.g. "multipart.FileHeader").
		return pkg.Name()
	})
	return strings.TrimSpace(s)
}
