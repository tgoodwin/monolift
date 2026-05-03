package activation

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// StructFieldKey identifies a function-typed field on a concrete struct type.
type StructFieldKey struct {
	PackagePath string `json:"package_path"`
	TypeName    string `json:"type_name"`
	FieldIndex  int    `json:"field_index"`
	FieldName   string `json:"field_name"`
	Signature   string `json:"signature"`
}

func (k StructFieldKey) String() string {
	return fmt.Sprintf("%s.%s[%d].%s:%s", k.PackagePath, k.TypeName, k.FieldIndex, k.FieldName, k.Signature)
}

// StoredFunction records one function value written to a struct field.
type StoredFunction struct {
	Key         StructFieldKey
	Func        *ssa.Function
	Position    Position
	Kind        EdgeKind
	Description string
	StructType  *types.Named
	FieldType   types.Type
	ValueType   types.Type
}

// FieldRead records a call through a function-typed struct field.
type FieldRead struct {
	Key       StructFieldKey
	Caller    *ssa.Function
	Position  Position
	FieldType types.Type
}

// StructFieldIndex records function values stored into struct fields. Later
// augmentation passes use this as shared input.
type StructFieldIndex struct {
	Stores      map[StructFieldKey][]StoredFunction
	Reads       map[StructFieldKey][]FieldRead
	Diagnostics []string
}

func newStructFieldIndex() *StructFieldIndex {
	return &StructFieldIndex{
		Stores: map[StructFieldKey][]StoredFunction{},
		Reads:  map[StructFieldKey][]FieldRead{},
	}
}

// AugmentStructField scans all loaded SSA functions for function values stored
// into struct fields. Read-side connection is added by later sprint tasks.
func AugmentStructField(graph *Graph, program *Program) (*StructFieldIndex, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	index := newStructFieldIndex()
	scanStructFieldWrites(program, index)
	scanStructFieldReads(program, index)
	connectStructFieldReads(graph, index)
	return index, nil
}

func scanStructFieldWrites(program *Program, index *StructFieldIndex) {
	for _, fn := range sortedFunctions(program.SSAProgram) {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				store, ok := instr.(*ssa.Store)
				if !ok {
					continue
				}
				fieldAddr, ok := store.Addr.(*ssa.FieldAddr)
				if !ok {
					continue
				}
				key, structType, fieldType, ok := structFieldInfo(fieldAddr)
				if !ok {
					continue
				}
				for _, stored := range resolveStoredCallables(store.Val) {
					stored.Key = key
					stored.Position = positionFor(program, store.Pos())
					stored.Kind = fieldStoreKind(fieldAddr)
					stored.Description = fmt.Sprintf("struct field write %s", key.String())
					stored.StructType = structType
					stored.FieldType = fieldType
					stored.ValueType = store.Val.Type()
					index.addStore(stored)
				}
			}
		}
	}
}

func connectStructFieldReads(graph *Graph, index *StructFieldIndex) {
	if graph == nil || index == nil {
		return
	}
	for _, key := range index.sortedKeys() {
		reads := index.Reads[key]
		stores := index.Stores[key]
		for _, read := range reads {
			from := graph.AddNode(FunctionKeyForSSA(read.Caller), read.Caller)
			if from == nil {
				continue
			}
			for _, stored := range stores {
				if !storedAssignableToField(stored, read.FieldType) {
					continue
				}
				to := graph.AddNode(FunctionKeyForSSA(stored.Func), stored.Func)
				if to == nil {
					continue
				}
				kind := stored.Kind
				if kind == "" {
					kind = StructFieldFuncValue
				}
				graph.AddEdge(from.ID, to.ID, kind, read.Position, fmt.Sprintf("struct field dispatch %s", key.String()))
			}
		}
	}
}

func (i *StructFieldIndex) sortedKeys() []StructFieldKey {
	if i == nil {
		return nil
	}
	seen := map[StructFieldKey]bool{}
	for key := range i.Stores {
		seen[key] = true
	}
	for key := range i.Reads {
		seen[key] = true
	}
	keys := make([]StructFieldKey, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(a, b int) bool {
		return keys[a].String() < keys[b].String()
	})
	return keys
}

func storedAssignableToField(stored StoredFunction, fieldType types.Type) bool {
	valueType := stored.ValueType
	if valueType == nil && stored.Func != nil {
		valueType = stored.Func.Signature
	}
	if valueType == nil || fieldType == nil {
		return false
	}
	if types.AssignableTo(valueType, fieldType) {
		return true
	}
	valueSig, valueOK := functionSignature(valueType)
	fieldSig, fieldOK := functionSignature(fieldType)
	return valueOK && fieldOK && types.Identical(valueSig, fieldSig)
}

func scanStructFieldReads(program *Program, index *StructFieldIndex) {
	for _, fn := range sortedFunctions(program.SSAProgram) {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(*ssa.Call)
				if !ok || call.Common() == nil || call.Common().IsInvoke() {
					continue
				}
				fieldAddr, ok := loadedFieldAddr(call.Common().Value)
				if !ok {
					continue
				}
				key, _, fieldType, ok := structFieldInfo(fieldAddr)
				if !ok {
					continue
				}
				index.addRead(FieldRead{
					Key:       key,
					Caller:    fn,
					Position:  positionFor(program, call.Common().Pos()),
					FieldType: fieldType,
				})
			}
		}
	}
}

func (i *StructFieldIndex) addRead(read FieldRead) {
	if i == nil || read.Caller == nil {
		return
	}
	for _, existing := range i.Reads[read.Key] {
		if existing.Caller == read.Caller {
			return
		}
	}
	i.Reads[read.Key] = append(i.Reads[read.Key], read)
	sort.SliceStable(i.Reads[read.Key], func(a, b int) bool {
		return FunctionKeyForSSA(i.Reads[read.Key][a].Caller).String() < FunctionKeyForSSA(i.Reads[read.Key][b].Caller).String()
	})
}

func loadedFieldAddr(value ssa.Value) (*ssa.FieldAddr, bool) {
	unop, ok := unwrapTransparentValue(value).(*ssa.UnOp)
	if !ok || unop.Op != token.MUL {
		return nil, false
	}
	fieldAddr, ok := unwrapTransparentValue(unop.X).(*ssa.FieldAddr)
	return fieldAddr, ok
}

func fieldStoreKind(addr *ssa.FieldAddr) EdgeKind {
	alloc, ok := addr.X.(*ssa.Alloc)
	if !ok {
		return StructFieldFuncValue
	}
	comment := strings.ToLower(alloc.Comment)
	if strings.Contains(comment, "complit") || strings.Contains(comment, "literal") {
		return StructLiteralFieldAssignment
	}
	return StructFieldFuncValue
}

func (i *StructFieldIndex) addStore(stored StoredFunction) {
	if i == nil || stored.Func == nil {
		return
	}
	for _, existing := range i.Stores[stored.Key] {
		if existing.Func == stored.Func && existing.Kind == stored.Kind {
			return
		}
	}
	i.Stores[stored.Key] = append(i.Stores[stored.Key], stored)
	sort.SliceStable(i.Stores[stored.Key], func(a, b int) bool {
		return FunctionKeyForSSA(i.Stores[stored.Key][a].Func).String() < FunctionKeyForSSA(i.Stores[stored.Key][b].Func).String()
	})
}

func resolveStoredCallables(value ssa.Value) []StoredFunction {
	return resolveStoredCallablesSeen(value, map[ssa.Value]bool{})
}

func resolveStoredCallablesSeen(value ssa.Value, seen map[ssa.Value]bool) []StoredFunction {
	if value == nil || seen[value] {
		return nil
	}
	seen[value] = true
	if fn, ok := value.(*ssa.Function); ok {
		return []StoredFunction{{Func: fn}}
	}
	switch v := value.(type) {
	case *ssa.MakeInterface:
		return resolveStoredCallablesSeen(v.X, seen)
	case *ssa.ChangeType:
		return resolveStoredCallablesSeen(v.X, seen)
	case *ssa.Convert:
		return resolveStoredCallablesSeen(v.X, seen)
	case *ssa.ChangeInterface:
		return resolveStoredCallablesSeen(v.X, seen)
	case *ssa.MakeClosure:
		if fn, ok := v.Fn.(*ssa.Function); ok {
			return []StoredFunction{{Func: closureTarget(fn)}}
		}
		return resolveStoredCallablesSeen(v.Fn, seen)
	case *ssa.Call:
		return resolveWrapperCall(v, seen)
	}
	return nil
}

func resolveWrapperCall(call *ssa.Call, seen map[ssa.Value]bool) []StoredFunction {
	if call == nil || call.Common() == nil {
		return nil
	}
	wrapper := call.Common().StaticCallee()
	if wrapper == nil || wrapper.Signature == nil || wrapper.Signature.Results().Len() != 1 {
		return nil
	}
	paramIndex, ok := delegatedWrapperParam(wrapper)
	if !ok || paramIndex < 0 || paramIndex >= len(call.Common().Args) {
		return nil
	}
	return resolveStoredCallablesSeen(call.Common().Args[paramIndex], seen)
}

func delegatedWrapperParam(wrapper *ssa.Function) (int, bool) {
	closure, ok := returnedClosure(wrapper)
	if !ok {
		return 0, false
	}
	fn, ok := closure.Fn.(*ssa.Function)
	if !ok {
		return 0, false
	}
	freeVar := singleCalledFreeVar(fn)
	if freeVar == nil {
		return 0, false
	}
	freeIndex := -1
	for i, candidate := range fn.FreeVars {
		if candidate == freeVar {
			freeIndex = i
			break
		}
	}
	if freeIndex < 0 || freeIndex >= len(closure.Bindings) {
		return 0, false
	}
	param := wrapperParamForBinding(wrapper, closure.Bindings[freeIndex])
	if param == nil {
		return 0, false
	}
	for i, candidate := range wrapper.Params {
		if candidate == param {
			return i, true
		}
	}
	return 0, false
}

func returnedClosure(wrapper *ssa.Function) (*ssa.MakeClosure, bool) {
	var result ssa.Value
	var returns int
	for _, block := range wrapper.Blocks {
		for _, instr := range block.Instrs {
			ret, ok := instr.(*ssa.Return)
			if !ok {
				continue
			}
			returns++
			if len(ret.Results) != 1 {
				return nil, false
			}
			result = ret.Results[0]
		}
	}
	if returns != 1 {
		return nil, false
	}
	closure, ok := unwrapTransparentValue(result).(*ssa.MakeClosure)
	return closure, ok
}

func singleCalledFreeVar(fn *ssa.Function) *ssa.FreeVar {
	var found *ssa.FreeVar
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok || call.Common() == nil || call.Common().IsInvoke() {
				continue
			}
			freeVar := calledFreeVar(call.Common().Value)
			if freeVar == nil {
				continue
			}
			if found != nil && found != freeVar {
				return nil
			}
			found = freeVar
		}
	}
	return found
}

func calledFreeVar(value ssa.Value) *ssa.FreeVar {
	value = unwrapTransparentValue(value)
	if freeVar, ok := value.(*ssa.FreeVar); ok {
		return freeVar
	}
	unop, ok := value.(*ssa.UnOp)
	if !ok || unop.Op != token.MUL {
		return nil
	}
	freeVar, _ := unwrapTransparentValue(unop.X).(*ssa.FreeVar)
	return freeVar
}

func wrapperParamForBinding(wrapper *ssa.Function, binding ssa.Value) *ssa.Parameter {
	binding = unwrapTransparentValue(binding)
	if param, ok := binding.(*ssa.Parameter); ok {
		return param
	}
	alloc, ok := binding.(*ssa.Alloc)
	if !ok {
		return nil
	}
	var found *ssa.Parameter
	for _, block := range wrapper.Blocks {
		for _, instr := range block.Instrs {
			store, ok := instr.(*ssa.Store)
			if !ok || store.Addr != alloc {
				continue
			}
			param, ok := unwrapTransparentValue(store.Val).(*ssa.Parameter)
			if !ok {
				continue
			}
			if found != nil && found != param {
				return nil
			}
			found = param
		}
	}
	return found
}

func unwrapTransparentValue(value ssa.Value) ssa.Value {
	for {
		switch v := value.(type) {
		case *ssa.MakeInterface:
			value = v.X
		case *ssa.ChangeType:
			value = v.X
		case *ssa.Convert:
			value = v.X
		case *ssa.ChangeInterface:
			value = v.X
		default:
			return value
		}
	}
}

func closureTarget(fn *ssa.Function) *ssa.Function {
	if fn == nil {
		return nil
	}
	if strings.Contains(fn.Synthetic, "bound method wrapper") {
		if callee := singleStaticCallee(fn); callee != nil {
			return callee
		}
	}
	return fn
}

func singleStaticCallee(fn *ssa.Function) *ssa.Function {
	var found *ssa.Function
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
			if found != nil && found != callee {
				return nil
			}
			found = callee
		}
	}
	return found
}

func structFieldInfo(addr *ssa.FieldAddr) (StructFieldKey, *types.Named, types.Type, bool) {
	if addr == nil || addr.X == nil {
		return StructFieldKey{}, nil, nil, false
	}
	named, st, ok := namedStructType(addr.X.Type())
	if !ok || addr.Field < 0 || addr.Field >= st.NumFields() {
		return StructFieldKey{}, nil, nil, false
	}
	field := st.Field(addr.Field)
	fieldType := field.Type()
	sig, ok := functionSignature(fieldType)
	if !ok {
		return StructFieldKey{}, nil, nil, false
	}
	pkgPath := ""
	if obj := named.Obj(); obj != nil && obj.Pkg() != nil {
		pkgPath = obj.Pkg().Path()
	}
	return StructFieldKey{
		PackagePath: pkgPath,
		TypeName:    named.Obj().Name(),
		FieldIndex:  addr.Field,
		FieldName:   field.Name(),
		Signature:   types.TypeString(sig, packagePathQualifier),
	}, named, fieldType, true
}

func namedStructType(t types.Type) (*types.Named, *types.Struct, bool) {
	t = deref(t)
	named, ok := t.(*types.Named)
	if !ok {
		return nil, nil, false
	}
	st, ok := named.Underlying().(*types.Struct)
	return named, st, ok
}

func deref(t types.Type) types.Type {
	if ptr, ok := t.(*types.Pointer); ok {
		return ptr.Elem()
	}
	return t
}

func functionSignature(t types.Type) (*types.Signature, bool) {
	if t == nil {
		return nil, false
	}
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	sig, ok := t.(*types.Signature)
	return sig, ok
}

func hasFunctionSignature(t types.Type) bool {
	_, ok := functionSignature(t)
	return ok
}

func packagePathQualifier(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}
