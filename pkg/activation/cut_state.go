package activation

import (
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

func classifyState(fn *ssa.Function) StateClass {
	if fn == nil || fn.Signature == nil {
		return SharedState
	}
	recv := fn.Signature.Recv()
	if recv == nil {
		return Stateless
	}
	return classifyReceiverState(recv.Type())
}

func classifyReceiverState(typ types.Type) StateClass {
	if class, ok := knownStateType(typ); ok {
		return class
	}

	typ = types.Unalias(typ)
	for {
		pointer, ok := typ.(*types.Pointer)
		if !ok {
			break
		}
		typ = types.Unalias(pointer.Elem())
	}

	named, _ := typ.(*types.Named)
	if named != nil {
		if class, ok := knownStateType(named); ok {
			return class
		}
		if strct, ok := types.Unalias(named.Underlying()).(*types.Struct); ok {
			class := classifyReceiverStructState(strct)
			return worseStateClass(class, fallbackStateForTypeName(named.Obj().Name(), packagePath(named.Obj().Pkg())))
		}
		return fallbackStateForTypeName(named.Obj().Name(), packagePath(named.Obj().Pkg()))
	}

	if strct, ok := typ.(*types.Struct); ok {
		return classifyReceiverStructState(strct)
	}
	return ConfigOnly
}

func classifyReceiverStructState(strct *types.Struct) StateClass {
	if strct.NumFields() == 0 {
		return ConfigOnly
	}
	worst := ConfigOnly
	for i := 0; i < strct.NumFields(); i++ {
		worst = worseStateClass(worst, classifyFieldState(strct.Field(i)))
		if worst == SharedState {
			return SharedState
		}
	}
	return worst
}

func classifyFieldState(field *types.Var) StateClass {
	if field == nil {
		return ConfigOnly
	}
	name := strings.ToLower(field.Name())
	typ := field.Type()

	if typeContainsCallback(typ, map[types.Type]bool{}) || typeContainsChannel(typ, map[types.Type]bool{}) {
		return SharedState
	}
	if class, ok := knownStateType(typ); ok {
		return class
	}
	if isSyncPrimitive(typ) {
		return SharedState
	}
	if strings.Contains(name, "cache") ||
		strings.Contains(name, "registry") ||
		strings.Contains(name, "plugin") ||
		strings.Contains(name, "session") ||
		strings.Contains(name, "worker") ||
		strings.Contains(name, "cancel") ||
		strings.Contains(name, "lifecycle") {
		return SharedState
	}
	if configBackedTypeName(cutTypeString(typ)) || configBackedTypeName(name) {
		return ClientReconstructible
	}
	if strings.Contains(name, "config") ||
		strings.Contains(name, "option") ||
		strings.Contains(name, "setting") ||
		strings.Contains(name, "logger") ||
		strings.Contains(name, "template") {
		return ConfigOnly
	}
	return ConfigOnly
}

func knownStateType(typ types.Type) (StateClass, bool) {
	named, pointer := namedType(typ)
	if named == nil || named.Obj() == nil {
		return "", false
	}
	pkgPath := packagePath(named.Obj().Pkg())
	name := named.Obj().Name()
	switch pkgPath {
	case "database/sql":
		if pointer && name == "DB" {
			return ClientReconstructible, true
		}
	case "net/http":
		if pointer && name == "Client" {
			return ClientReconstructible, true
		}
	case "log":
		if pointer && name == "Logger" {
			return ConfigOnly, true
		}
	case "html/template", "text/template":
		if pointer && name == "Template" {
			return ConfigOnly, true
		}
	}
	if pointer && configBackedTypeName(name) {
		return ClientReconstructible, true
	}
	return "", false
}

func typeContainsChannel(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true
	defer delete(seen, typ)

	switch t := typ.(type) {
	case *types.Chan:
		return true
	case *types.Pointer:
		return typeContainsChannel(t.Elem(), seen)
	case *types.Slice:
		return typeContainsChannel(t.Elem(), seen)
	case *types.Array:
		return typeContainsChannel(t.Elem(), seen)
	case *types.Map:
		return typeContainsChannel(t.Key(), seen) || typeContainsChannel(t.Elem(), seen)
	case *types.Struct:
		for i := 0; i < t.NumFields(); i++ {
			if typeContainsChannel(t.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Named:
		return typeContainsChannel(t.Underlying(), seen)
	}
	return false
}

func isSyncPrimitive(typ types.Type) bool {
	named, _ := namedType(typ)
	if named == nil || named.Obj() == nil || packagePath(named.Obj().Pkg()) != "sync" {
		return false
	}
	switch named.Obj().Name() {
	case "Mutex", "RWMutex", "WaitGroup", "Cond", "Once", "Pool", "Map":
		return true
	default:
		return false
	}
}

func fallbackStateForTypeName(name, pkgPath string) StateClass {
	lowerName := strings.ToLower(name)
	lowerPkg := strings.ToLower(pkgPath)
	if strings.Contains(lowerPkg, "mattermost") && (strings.Contains(lowerName, "app") || strings.Contains(lowerName, "server")) {
		return SharedState
	}
	if strings.Contains(lowerName, "registry") || strings.Contains(lowerName, "cache") || strings.Contains(lowerName, "plugin") {
		return SharedState
	}
	if configBackedTypeName(name) {
		return ClientReconstructible
	}
	return ConfigOnly
}

func packagePath(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}

func worseStateClass(a, b StateClass) StateClass {
	if stateClassRank(b) > stateClassRank(a) {
		return b
	}
	return a
}

func stateClassRank(class StateClass) int {
	switch class {
	case Stateless:
		return 0
	case ConfigOnly:
		return 1
	case ClientReconstructible:
		return 2
	case SharedState:
		return 3
	default:
		return 3
	}
}
