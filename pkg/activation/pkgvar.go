package activation

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

type packageVarStore struct {
	Func         *ssa.Function
	ConcreteType types.Type
	ValueType    types.Type
	Position     Position
}

type packageVarIndex struct {
	Stores map[*ssa.Global][]packageVarStore
}

func newPackageVarIndex() *packageVarIndex {
	return &packageVarIndex{Stores: map[*ssa.Global][]packageVarStore{}}
}

// AugmentPackageVars connects calls through package-level function variables
// and interface globals to concrete functions stored in those globals.
func AugmentPackageVars(graph *Graph, program *Program) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	index := newPackageVarIndex()
	scanPackageVarWrites(program, index)
	connectPackageVarReads(graph, program, index)
	return nil
}

func scanPackageVarWrites(program *Program, index *packageVarIndex) {
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
				global, ok := packageGlobal(store.Addr)
				if !ok || !globalMayHoldCallable(global, store.Val) {
					continue
				}
				pos := positionFor(program, store.Pos())
				for _, stored := range resolveStoredCallables(store.Val) {
					index.addStore(global, packageVarStore{
						Func:      closureTarget(stored.Func),
						ValueType: store.Val.Type(),
						Position:  pos,
					})
				}
				if concrete := concreteStoredType(store.Val); concrete != nil && isInterfaceType(globalValueType(global)) {
					index.addStore(global, packageVarStore{
						ConcreteType: concrete,
						ValueType:    store.Val.Type(),
						Position:     pos,
					})
				}
			}
		}
	}
}

func connectPackageVarReads(graph *Graph, program *Program, index *packageVarIndex) {
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
				if !ok || call.Common() == nil {
					continue
				}
				common := call.Common()
				global, ok := loadedGlobal(common.Value)
				if !ok {
					continue
				}
				stores := index.Stores[global]
				if len(stores) == 0 {
					continue
				}
				if common.IsInvoke() {
					connectPackageVarInvoke(graph, program, from, call, common.Method, global, stores)
					continue
				}
				connectPackageVarDirectCall(graph, program, from, call, global, stores)
			}
		}
	}
}

func connectPackageVarDirectCall(graph *Graph, program *Program, from *Node, call ssa.CallInstruction, global *ssa.Global, stores []packageVarStore) {
	for _, store := range stores {
		if store.Func == nil || hasGenericContext(store.Func) {
			continue
		}
		to := graph.AddNode(FunctionKeyForSSA(store.Func), store.Func)
		if to == nil {
			continue
		}
		graph.AddEdge(from.ID, to.ID, PackageVarFuncValue, positionFor(program, call.Common().Pos()), fmt.Sprintf("package global dispatch %s", packageGlobalName(global)))
	}
}

func connectPackageVarInvoke(graph *Graph, program *Program, from *Node, call ssa.CallInstruction, method *types.Func, global *ssa.Global, stores []packageVarStore) {
	if method == nil {
		return
	}
	for _, store := range stores {
		target := packageVarMethod(program, store.ConcreteType, method)
		if target == nil || hasGenericContext(target) {
			continue
		}
		to := graph.AddNode(FunctionKeyForSSA(target), target)
		if to == nil {
			continue
		}
		graph.AddEdge(from.ID, to.ID, PackageVarFuncValue, positionFor(program, call.Common().Pos()), fmt.Sprintf("package global interface dispatch %s.%s", packageGlobalName(global), method.Name()))
	}
}

func packageVarMethod(program *Program, concrete types.Type, method *types.Func) (target *ssa.Function) {
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

func (i *packageVarIndex) addStore(global *ssa.Global, store packageVarStore) {
	if i == nil || global == nil || (store.Func == nil && store.ConcreteType == nil) {
		return
	}
	for _, existing := range i.Stores[global] {
		switch {
		case store.Func != nil && existing.Func == store.Func:
			return
		case store.ConcreteType != nil && existing.ConcreteType != nil && types.Identical(store.ConcreteType, existing.ConcreteType):
			return
		}
	}
	i.Stores[global] = append(i.Stores[global], store)
	sort.SliceStable(i.Stores[global], func(a, b int) bool {
		return packageVarStoreKey(i.Stores[global][a]) < packageVarStoreKey(i.Stores[global][b])
	})
}

func packageVarStoreKey(store packageVarStore) string {
	if store.Func != nil {
		return FunctionKeyForSSA(store.Func).String()
	}
	return types.TypeString(store.ConcreteType, packagePathQualifier)
}

func packageGlobal(value ssa.Value) (*ssa.Global, bool) {
	value = unwrapTransparentValue(value)
	global, ok := value.(*ssa.Global)
	return global, ok
}

func loadedGlobal(value ssa.Value) (*ssa.Global, bool) {
	value = unwrapTransparentValue(value)
	if global, ok := value.(*ssa.Global); ok {
		return global, true
	}
	unop, ok := value.(*ssa.UnOp)
	if !ok || unop.Op != token.MUL {
		return nil, false
	}
	return packageGlobal(unop.X)
}

func globalMayHoldCallable(global *ssa.Global, value ssa.Value) bool {
	if global == nil {
		return false
	}
	globalType := globalValueType(global)
	return hasFunctionSignature(globalType) ||
		isInterfaceType(globalType) ||
		valueHasFunctionOrInterfaceType(value)
}

func valueHasFunctionOrInterfaceType(value ssa.Value) bool {
	if value == nil {
		return false
	}
	t := value.Type()
	return hasFunctionSignature(t) || isInterfaceType(t)
}

func globalValueType(global *ssa.Global) types.Type {
	if global == nil {
		return nil
	}
	return deref(global.Type())
}

func concreteStoredType(value ssa.Value) types.Type {
	value = unwrapTransparentValue(value)
	if value == nil || value.Type() == nil {
		return nil
	}
	t := value.Type()
	if hasFunctionSignature(t) || isInterfaceType(t) {
		return nil
	}
	return t
}

func isInterfaceType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok
}

func packageGlobalName(global *ssa.Global) string {
	if global == nil {
		return ""
	}
	pkg := ""
	if global.Pkg != nil && global.Pkg.Pkg != nil {
		pkg = global.Pkg.Pkg.Path()
	}
	if pkg == "" {
		return global.Name()
	}
	return pkg + "." + global.Name()
}
