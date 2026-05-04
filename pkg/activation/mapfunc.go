package activation

import (
	"fmt"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

type mapFuncKey struct {
	PackagePath string
	GlobalName  string
	Type        string
}

func (k mapFuncKey) String() string {
	if k.PackagePath != "" || k.GlobalName != "" {
		return k.PackagePath + "." + k.GlobalName + ":" + k.Type
	}
	return k.Type
}

type mapFuncStore struct {
	Func     *ssa.Function
	Position Position
}

type mapParamStore struct {
	Key        mapFuncKey
	ParamIndex int
	Position   Position
}

type mapFuncIndex struct {
	Stores      map[mapFuncKey][]mapFuncStore
	ParamStores map[*ssa.Function][]mapParamStore
}

func newMapFuncIndex() *mapFuncIndex {
	return &mapFuncIndex{
		Stores:      map[mapFuncKey][]mapFuncStore{},
		ParamStores: map[*ssa.Function][]mapParamStore{},
	}
}

// AugmentMapFuncValues connects calls through map-indexed function values to
// functions stored into compatible maps, including simple registration wrappers.
func AugmentMapFuncValues(graph *Graph, program *Program) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	index := buildMapFuncIndex(program)
	connectMapFuncLookups(graph, program, index)
	return nil
}

func buildMapFuncIndex(program *Program) *mapFuncIndex {
	index := newMapFuncIndex()
	scanMapFuncStores(program, index)
	propagateMapParamStores(program, index)
	return index
}

func scanMapFuncStores(program *Program, index *mapFuncIndex) {
	for _, fn := range sortedFunctions(program.SSAProgram) {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				update, ok := instr.(*ssa.MapUpdate)
				if !ok {
					continue
				}
				key, ok := mapFuncKeyForValue(update.Map)
				if !ok || !hasFunctionValueType(update.Value) {
					continue
				}
				pos := positionFor(program, update.Pos())
				for _, stored := range resolveStoredCallables(update.Value) {
					index.addStore(key, mapFuncStore{Func: closureTarget(stored.Func), Position: pos})
				}
				if paramIndex, ok := mapStoredParam(fn, update.Value); ok {
					index.addParamStore(fn, mapParamStore{Key: key, ParamIndex: paramIndex, Position: pos})
				}
			}
		}
	}
}

func propagateMapParamStores(program *Program, index *mapFuncIndex) {
	if program == nil || index == nil {
		return
	}
	for changed := true; changed; {
		changed = false
		for _, caller := range sortedFunctions(program.SSAProgram) {
			if caller == nil {
				continue
			}
			for _, block := range caller.Blocks {
				for _, instr := range block.Instrs {
					call, ok := instr.(ssa.CallInstruction)
					if !ok || call.Common() == nil {
						continue
					}
					common := call.Common()
					callee := common.StaticCallee()
					if callee == nil {
						continue
					}
					for _, paramStore := range append([]mapParamStore(nil), index.ParamStores[callee]...) {
						if paramStore.ParamIndex < 0 || paramStore.ParamIndex >= len(common.Args) {
							continue
						}
						arg := common.Args[paramStore.ParamIndex]
						for _, stored := range resolveStoredCallables(arg) {
							changed = index.addStore(paramStore.Key, mapFuncStore{
								Func:     closureTarget(stored.Func),
								Position: positionFor(program, common.Pos()),
							}) || changed
						}
						if callerParam, ok := parameterIndexForValue(caller, arg); ok {
							changed = index.addParamStore(caller, mapParamStore{
								Key:        paramStore.Key,
								ParamIndex: callerParam,
								Position:   positionFor(program, common.Pos()),
							}) || changed
						}
					}
				}
			}
		}
	}
}

func connectMapFuncLookups(graph *Graph, program *Program, index *mapFuncIndex) {
	if graph == nil || program == nil || index == nil {
		return
	}
	for _, fn := range sortedFunctions(program.SSAProgram) {
		from := graph.nodeByFunction(fn)
		if from == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
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
				for _, store := range index.Stores[key] {
					if store.Func == nil || hasGenericContext(store.Func) {
						continue
					}
					to := graph.AddNode(FunctionKeyForSSA(store.Func), store.Func)
					if to == nil {
						continue
					}
					graph.AddEdge(from.ID, to.ID, MapFuncValue, positionFor(program, call.Common().Pos()), fmt.Sprintf("map function dispatch %s", key.String()))
				}
			}
		}
	}
}

func (i *mapFuncIndex) addStore(key mapFuncKey, store mapFuncStore) bool {
	if i == nil || key.Type == "" || store.Func == nil {
		return false
	}
	for _, existing := range i.Stores[key] {
		if existing.Func == store.Func {
			return false
		}
	}
	i.Stores[key] = append(i.Stores[key], store)
	sort.SliceStable(i.Stores[key], func(a, b int) bool {
		return FunctionKeyForSSA(i.Stores[key][a].Func).String() < FunctionKeyForSSA(i.Stores[key][b].Func).String()
	})
	return true
}

func (i *mapFuncIndex) addParamStore(fn *ssa.Function, store mapParamStore) bool {
	if i == nil || fn == nil || store.Key.Type == "" || store.ParamIndex < 0 {
		return false
	}
	for _, existing := range i.ParamStores[fn] {
		if existing.Key == store.Key && existing.ParamIndex == store.ParamIndex {
			return false
		}
	}
	i.ParamStores[fn] = append(i.ParamStores[fn], store)
	sort.SliceStable(i.ParamStores[fn], func(a, b int) bool {
		if i.ParamStores[fn][a].Key.String() != i.ParamStores[fn][b].Key.String() {
			return i.ParamStores[fn][a].Key.String() < i.ParamStores[fn][b].Key.String()
		}
		return i.ParamStores[fn][a].ParamIndex < i.ParamStores[fn][b].ParamIndex
	})
	return true
}

func mapStoredParam(fn *ssa.Function, value ssa.Value) (int, bool) {
	if idx, ok := parameterIndexForValue(fn, value); ok {
		return idx, true
	}
	closure, ok := unwrapTransparentValue(value).(*ssa.MakeClosure)
	if !ok {
		return 0, false
	}
	closureFn, ok := closure.Fn.(*ssa.Function)
	if !ok {
		return 0, false
	}
	freeVar := singleCalledFreeVar(closureFn)
	if freeVar == nil {
		return 0, false
	}
	freeIndex := -1
	for i, candidate := range closureFn.FreeVars {
		if candidate == freeVar {
			freeIndex = i
			break
		}
	}
	if freeIndex < 0 || freeIndex >= len(closure.Bindings) {
		return 0, false
	}
	return parameterIndexForValue(fn, closure.Bindings[freeIndex])
}

func parameterIndexForValue(fn *ssa.Function, value ssa.Value) (int, bool) {
	if fn == nil || value == nil {
		return 0, false
	}
	if param, ok := unwrapTransparentValue(value).(*ssa.Parameter); ok {
		for i, candidate := range fn.Params {
			if candidate == param {
				return i, true
			}
		}
	}
	if alloc, ok := unwrapTransparentValue(value).(*ssa.Alloc); ok {
		param := wrapperParamForBinding(fn, alloc)
		if param == nil {
			return 0, false
		}
		for i, candidate := range fn.Params {
			if candidate == param {
				return i, true
			}
		}
	}
	if unop, ok := unwrapTransparentValue(value).(*ssa.UnOp); ok {
		return parameterIndexForValue(fn, unop.X)
	}
	return 0, false
}

func lookupForCalledValue(value ssa.Value) (*ssa.Lookup, bool) {
	value = unwrapTransparentValue(value)
	if lookup, ok := value.(*ssa.Lookup); ok {
		return lookup, true
	}
	extract, ok := value.(*ssa.Extract)
	if !ok || extract.Index != 0 {
		return nil, false
	}
	lookup, ok := unwrapTransparentValue(extract.Tuple).(*ssa.Lookup)
	return lookup, ok
}

func mapFuncKeyForValue(value ssa.Value) (mapFuncKey, bool) {
	if global, ok := loadedGlobal(value); ok {
		mapType, ok := functionMapType(globalValueType(global))
		if !ok {
			return mapFuncKey{}, false
		}
		pkg := ""
		if global.Pkg != nil && global.Pkg.Pkg != nil {
			pkg = global.Pkg.Pkg.Path()
		}
		return mapFuncKey{
			PackagePath: pkg,
			GlobalName:  global.Name(),
			Type:        types.TypeString(mapType, packagePathQualifier),
		}, true
	}
	mapType, ok := functionMapType(unwrapTransparentValue(value).Type())
	if !ok {
		return mapFuncKey{}, false
	}
	return mapFuncKey{Type: types.TypeString(mapType, packagePathQualifier)}, true
}

func functionMapType(t types.Type) (types.Type, bool) {
	if t == nil {
		return nil, false
	}
	t = types.Unalias(t)
	if named, ok := t.(*types.Named); ok {
		t = named.Underlying()
	}
	mapType, ok := t.(*types.Map)
	if !ok || !hasFunctionSignature(mapType.Elem()) {
		return nil, false
	}
	return mapType, true
}
