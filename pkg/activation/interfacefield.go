package activation

import (
	"fmt"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

type interfaceFieldStore struct {
	Key          StructFieldKey
	ConcreteType types.Type
	Position     Position
}

type interfaceFieldIndex struct {
	Stores map[StructFieldKey][]interfaceFieldStore
}

func newInterfaceFieldIndex() *interfaceFieldIndex {
	return &interfaceFieldIndex{Stores: map[StructFieldKey][]interfaceFieldStore{}}
}

// AugmentInterfaceFields connects interface-typed struct field invokes to
// concrete result types returned by map-indexed factory calls.
func AugmentInterfaceFields(graph *Graph, program *Program, mapIndex *mapFuncIndex) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	index := newInterfaceFieldIndex()
	results := mapFactoryResultTypes(program, mapIndex)
	scanInterfaceFieldStores(program, index, results)
	connectInterfaceFieldReads(graph, program, index)
	return nil
}

func mapFactoryResultTypes(program *Program, mapIndex *mapFuncIndex) map[ssa.Value][]types.Type {
	out := map[ssa.Value][]types.Type{}
	if mapIndex == nil {
		mapIndex = buildMapFuncIndex(program)
	}
	for _, fn := range program.Functions() {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(*ssa.Call)
				if !ok || call.Common() == nil || call.Common().IsInvoke() || call.Common().StaticCallee() != nil {
					continue
				}
				lookup, ok := lookupForCalledValue(call.Common().Value)
				if !ok {
					continue
				}
				key, ok := mapFuncKeyForValue(lookup.X)
				if !ok {
					continue
				}
				for _, store := range mapIndex.Stores[key] {
					for _, resultType := range concreteResultTypes(store.Func) {
						out[call] = appendUniqueType(out[call], resultType)
					}
				}
			}
		}
	}
	return out
}

func scanInterfaceFieldStores(program *Program, index *interfaceFieldIndex, results map[ssa.Value][]types.Type) {
	for _, fn := range program.Functions() {
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
				key, _, fieldType, ok := interfaceFieldInfo(fieldAddr)
				if !ok {
					continue
				}
				for _, concreteType := range concreteTypesForValue(store.Val, results, map[ssa.Value]bool{}) {
					if !types.AssignableTo(concreteType, fieldType) {
						continue
					}
					index.addStore(interfaceFieldStore{
						Key:          key,
						ConcreteType: concreteType,
						Position:     positionFor(program, store.Pos()),
					})
				}
			}
		}
	}
}

func connectInterfaceFieldReads(graph *Graph, program *Program, index *interfaceFieldIndex) {
	if graph == nil || program == nil || index == nil {
		return
	}
	for _, fn := range program.Functions() {
		from := graph.nodeByFunction(fn)
		if from == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok || call.Common() == nil || !call.Common().IsInvoke() {
					continue
				}
				fieldAddr, ok := loadedFieldAddr(call.Common().Value)
				if !ok {
					continue
				}
				key, _, _, ok := interfaceFieldInfo(fieldAddr)
				if !ok {
					continue
				}
				for _, store := range index.Stores[key] {
					target := interfaceFieldMethod(program, store.ConcreteType, call.Common().Method)
					if target == nil || hasGenericContext(target) {
						continue
					}
					to := graph.AddNode(FunctionKeyForSSA(target), target)
					if to == nil {
						continue
					}
					graph.AddEdge(from.ID, to.ID, InterfaceDispatch, positionFor(program, call.Common().Pos()), fmt.Sprintf("interface field dispatch %s", key.String()))
				}
			}
		}
	}
}

func (i *interfaceFieldIndex) addStore(store interfaceFieldStore) {
	if i == nil || store.Key.Signature == "" || store.ConcreteType == nil {
		return
	}
	for _, existing := range i.Stores[store.Key] {
		if types.Identical(existing.ConcreteType, store.ConcreteType) {
			return
		}
	}
	i.Stores[store.Key] = append(i.Stores[store.Key], store)
	sort.SliceStable(i.Stores[store.Key], func(a, b int) bool {
		return types.TypeString(i.Stores[store.Key][a].ConcreteType, packagePathQualifier) <
			types.TypeString(i.Stores[store.Key][b].ConcreteType, packagePathQualifier)
	})
}

func concreteResultTypes(fn *ssa.Function) []types.Type {
	if fn == nil || fn.Signature == nil || fn.Signature.Results() == nil {
		return nil
	}
	var out []types.Type
	for i := 0; i < fn.Signature.Results().Len(); i++ {
		t := fn.Signature.Results().At(i).Type()
		if t == nil || isInterfaceType(t) || hasFunctionSignature(t) {
			continue
		}
		out = appendUniqueType(out, t)
	}
	return out
}

func concreteTypesForValue(value ssa.Value, results map[ssa.Value][]types.Type, seen map[ssa.Value]bool) []types.Type {
	if value == nil || seen[value] {
		return nil
	}
	seen[value] = true
	if typesForValue := results[value]; len(typesForValue) > 0 {
		return typesForValue
	}
	unwrapped := unwrapTransparentValue(value)
	if unwrapped != value {
		return concreteTypesForValue(unwrapped, results, seen)
	}
	switch v := unwrapped.(type) {
	case *ssa.MakeInterface:
		if v.X != nil && v.X.Type() != nil && !isInterfaceType(v.X.Type()) && !hasFunctionSignature(v.X.Type()) {
			return []types.Type{v.X.Type()}
		}
		return concreteTypesForValue(v.X, results, seen)
	case *ssa.Phi:
		var out []types.Type
		for _, edge := range v.Edges {
			for _, t := range concreteTypesForValue(edge, results, seen) {
				out = appendUniqueType(out, t)
			}
		}
		return out
	default:
		return nil
	}
}

func appendUniqueType(typesList []types.Type, t types.Type) []types.Type {
	if t == nil {
		return typesList
	}
	for _, existing := range typesList {
		if types.Identical(existing, t) {
			return typesList
		}
	}
	return append(typesList, t)
}

func interfaceFieldInfo(addr *ssa.FieldAddr) (StructFieldKey, *types.Named, types.Type, bool) {
	if addr == nil || addr.X == nil {
		return StructFieldKey{}, nil, nil, false
	}
	named, st, ok := namedStructType(addr.X.Type())
	if !ok || addr.Field < 0 || addr.Field >= st.NumFields() {
		return StructFieldKey{}, nil, nil, false
	}
	field := st.Field(addr.Field)
	fieldType := field.Type()
	if !isInterfaceType(fieldType) {
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
		Signature:   types.TypeString(fieldType, packagePathQualifier),
	}, named, fieldType, true
}

func interfaceFieldMethod(program *Program, concrete types.Type, method *types.Func) (target *ssa.Function) {
	if program == nil || program.SSAProgram == nil || concrete == nil || method == nil {
		return nil
	}
	defer func() {
		if recover() != nil {
			target = nil
		}
	}()
	return program.SSAProgram.LookupMethod(concrete, method.Pkg(), method.Name())
}
