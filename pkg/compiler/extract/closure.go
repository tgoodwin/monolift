package extract

import (
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type closureResult struct {
	Closure           reportv2.Closure
	ExternalDeps      []reportv2.ExternalDep
	PrecisionTriggers []string
	ReachableFuncs    []*ssa.Function
}

type dispatchPlan struct {
	SelectedGraph *callgraph.Graph
	CHAGraph      *callgraph.Graph
	RegistryKey   *string
	triggers      map[string]bool
}

func buildClosure(loaded *loadedModule, built *builtProgram, root reportv2.Root) closureResult {
	included := map[string]reportv2.SymbolEntry{}
	excluded := map[string]reportv2.SymbolEntry{}
	externalDeps := map[string]reportv2.ExternalDep{}
	rootEntry := symbolEntryForIdentity(root.Identity, reportSpan(loaded.RootPragma.Span, loaded.ModuleRoot))
	addIncludedEntry(included, rootEntry)

	queue := resolveRootFunctions(built, root)
	rootFunctions := append([]*ssa.Function(nil), queue...)
	dispatch := buildDispatchPlan(built, queue, root.RegistryKey)
	visitedFunctions := map[*ssa.Function]bool{}
	visitedPackages := map[string]bool{}

	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if fn == nil || visitedFunctions[fn] {
			continue
		}
		visitedFunctions[fn] = true
		addIncludedEntry(included, symbolEntryForFunction(loaded, fn))

		if fn.Package() != nil && fn.Package().Pkg != nil && !visitedPackages[fn.Package().Pkg.Path()] {
			visitedPackages[fn.Package().Pkg.Path()] = true
			for _, entry := range packageConstantEntries(loaded, fn.Package()) {
				addIncludedEntry(included, entry)
			}
		}
		for _, anon := range fn.AnonFuncs {
			if shouldIncludeFunction(loaded, anon) {
				queue = append(queue, anon)
			}
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch value := instr.(type) {
				case *ssa.MakeClosure:
					if callee, ok := value.Fn.(*ssa.Function); ok && shouldIncludeFunction(loaded, callee) {
						queue = append(queue, callee)
					}
				case ssa.CallInstruction:
					for _, callee := range resolveCallCallees(dispatch, fn, value) {
						if shouldIncludeFunction(loaded, callee) {
							queue = append(queue, callee)
						} else if callee != nil {
							entry := symbolEntryForFunction(loaded, callee)
							addExcludedEntry(excluded, entry)
							addExternalDep(externalDeps, externalDepForEntry(root, entry))
						}
					}
				}

				for _, operand := range instr.Operands(nil) {
					if operand == nil || *operand == nil {
						continue
					}
					value := *operand
					switch typed := value.(type) {
					case *ssa.Function:
						if shouldIncludeFunction(loaded, typed) {
							queue = append(queue, typed)
						}
					case *ssa.Global:
						addIncludedEntry(included, symbolEntryForGlobal(loaded, typed))
					}
					for _, entry := range namedTypeEntriesFromType(loaded, value.Type(), value.Pos()) {
						addIncludedEntry(included, entry)
					}
				}
			}
		}
	}

	return closureResult{
		Closure: reportv2.Closure{
			IncludedSymbols: sortedEntries(included),
			ExcludedSymbols: sortedEntries(excluded),
			WiringPaths:     buildWiringPaths(loaded, rootEntry, rootFunctions),
		},
		ExternalDeps:      sortedExternalDeps(externalDeps),
		PrecisionTriggers: sortedTriggerKeys(dispatch.triggers),
		ReachableFuncs:    sortedFunctions(visitedFunctions),
	}
}

func resolveRootFunctions(built *builtProgram, root reportv2.Root) []*ssa.Function {
	var symbols []reportv2.SymbolIdentity
	if len(root.ExposedOperations) > 0 {
		symbols = append(symbols, root.ExposedOperations...)
	} else {
		symbols = append(symbols, root.Identity)
	}

	var out []*ssa.Function
	seen := map[*ssa.Function]bool{}
	for _, symbol := range symbols {
		fn := lookupFunction(built, symbol)
		if fn != nil && !seen[fn] {
			seen[fn] = true
			out = append(out, fn)
		}
	}
	return out
}

func lookupFunction(built *builtProgram, symbol reportv2.SymbolIdentity) *ssa.Function {
	for fn := range built.Functions {
		if fn == nil || fn.Package() == nil || fn.Package().Pkg == nil {
			continue
		}
		if fn.Package().Pkg.Path() != symbol.PackagePath {
			continue
		}
		if functionObjectName(fn) == symbol.ObjectName {
			return fn
		}
	}
	return nil
}

func resolveCallCallees(plan dispatchPlan, caller *ssa.Function, call ssa.CallInstruction) []*ssa.Function {
	common := call.Common()
	if callee := common.StaticCallee(); callee != nil {
		return []*ssa.Function{callee}
	}
	if !common.IsInvoke() || plan.SelectedGraph == nil {
		return nil
	}

	node := plan.SelectedGraph.Nodes[caller]
	if node == nil {
		return nil
	}

	var callees []*ssa.Function
	seen := map[*ssa.Function]bool{}
	for _, edge := range node.Out {
		if edge.Site != call || edge.Callee == nil || edge.Callee.Func == nil || seen[edge.Callee.Func] {
			continue
		}
		seen[edge.Callee.Func] = true
		callees = append(callees, edge.Callee.Func)
	}
	if len(callees) > 1 {
		plan.triggers["dispatch-growth:"+dispatchTriggerID(caller, common.Method.Name())] = true
	}
	if plan.RegistryKey != nil && plan.CHAGraph != nil {
		chaCount := len(resolveGraphCallees(plan.CHAGraph, caller, call))
		if chaCount > len(callees) {
			plan.triggers["rta-escape:"+functionObjectName(caller)] = true
		}
	}
	return callees
}

func resolveGraphCallees(graph *callgraph.Graph, caller *ssa.Function, call ssa.CallInstruction) []*ssa.Function {
	if graph == nil {
		return nil
	}
	node := graph.Nodes[caller]
	if node == nil {
		return nil
	}
	var callees []*ssa.Function
	seen := map[*ssa.Function]bool{}
	for _, edge := range node.Out {
		if edge.Site != call || edge.Callee == nil || edge.Callee.Func == nil || seen[edge.Callee.Func] {
			continue
		}
		seen[edge.Callee.Func] = true
		callees = append(callees, edge.Callee.Func)
	}
	return callees
}

func shouldIncludeFunction(loaded *loadedModule, fn *ssa.Function) bool {
	if fn == nil || fn.Package() == nil || fn.Package().Pkg == nil {
		return false
	}
	return isInternalPackage(loaded, fn.Package().Pkg.Path())
}

func isInternalPackage(loaded *loadedModule, pkgPath string) bool {
	for _, pkg := range loaded.Packages {
		if pkg.PkgPath != pkgPath || pkg.Module == nil || loaded.RootPkg.Module == nil {
			continue
		}
		return pkg.Module.Path == loaded.RootPkg.Module.Path
	}
	return false
}

func symbolEntryForFunction(loaded *loadedModule, fn *ssa.Function) reportv2.SymbolEntry {
	modulePath, packagePath := functionModuleAndPackage(loaded, fn)
	return reportv2.SymbolEntry{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  functionObjectName(fn),
			Kind:        functionKind(fn),
		},
		Span:    functionSpan(loaded, fn.Pos()),
		RuleIDs: []string{},
	}
}

func symbolEntryForGlobal(loaded *loadedModule, global *ssa.Global) reportv2.SymbolEntry {
	modulePath, packagePath := packageModuleAndPath(loaded, global.Pkg)
	return reportv2.SymbolEntry{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  global.Name(),
			Kind:        "variable",
		},
		Span:    functionSpan(loaded, global.Pos()),
		RuleIDs: []string{},
	}
}

func symbolEntryForNamedConst(loaded *loadedModule, constant *ssa.NamedConst) reportv2.SymbolEntry {
	modulePath, packagePath := packageModuleAndPath(loaded, constant.Package())
	return reportv2.SymbolEntry{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  modulePath,
			PackagePath: packagePath,
			ObjectName:  constant.Name(),
			Kind:        "constant",
		},
		Span:    functionSpan(loaded, constant.Pos()),
		RuleIDs: []string{},
	}
}

func namedTypeEntriesFromType(loaded *loadedModule, typ types.Type, pos token.Pos) []reportv2.SymbolEntry {
	if typ == nil {
		return nil
	}
	var entries []reportv2.SymbolEntry
	seen := map[string]bool{}
	visitedTypes := map[types.Type]bool{}

	var visit func(types.Type)
	visit = func(current types.Type) {
		if current == nil || visitedTypes[current] {
			return
		}
		visitedTypes[current] = true
		switch t := current.(type) {
		case *types.Named:
			obj := t.Obj()
			if obj == nil || obj.Pkg() == nil {
				return
			}
			key := obj.Pkg().Path() + ":" + obj.Name()
			if seen[key] {
				return
			}
			seen[key] = true
			entries = append(entries, reportv2.SymbolEntry{
				Identity: reportv2.SymbolIdentity{
					ModulePath:  modulePathForTypesPackage(loaded, obj.Pkg()),
					PackagePath: obj.Pkg().Path(),
					ObjectName:  obj.Name(),
					Kind:        typeKind(t),
				},
				Span:    functionSpan(loaded, pos),
				RuleIDs: []string{},
			})
			visit(t.Underlying())
		case *types.Pointer:
			visit(t.Elem())
		case *types.Slice:
			visit(t.Elem())
		case *types.Array:
			visit(t.Elem())
		case *types.Map:
			visit(t.Key())
			visit(t.Elem())
		case *types.Chan:
			visit(t.Elem())
		case *types.Signature:
			if recv := t.Recv(); recv != nil {
				visit(recv.Type())
			}
			for i := 0; i < t.Params().Len(); i++ {
				visit(t.Params().At(i).Type())
			}
			for i := 0; i < t.Results().Len(); i++ {
				visit(t.Results().At(i).Type())
			}
		case *types.Struct:
			for i := 0; i < t.NumFields(); i++ {
				visit(t.Field(i).Type())
			}
		case *types.Interface:
			for i := 0; i < t.NumMethods(); i++ {
				visit(t.Method(i).Type())
			}
		}
	}
	visit(typ)

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Identity.PackagePath == entries[j].Identity.PackagePath {
			return entries[i].Identity.ObjectName < entries[j].Identity.ObjectName
		}
		return entries[i].Identity.PackagePath < entries[j].Identity.PackagePath
	})
	return entries
}

func packageConstantEntries(loaded *loadedModule, pkg *ssa.Package) []reportv2.SymbolEntry {
	if pkg == nil || pkg.Pkg == nil {
		return nil
	}
	modulePath, packagePath := packageModuleAndPath(loaded, pkg)
	var entries []reportv2.SymbolEntry
	for _, member := range pkg.Members {
		constant, ok := member.(*ssa.NamedConst)
		if !ok {
			continue
		}
		entries = append(entries, reportv2.SymbolEntry{
			Identity: reportv2.SymbolIdentity{
				ModulePath:  modulePath,
				PackagePath: packagePath,
				ObjectName:  constant.Name(),
				Kind:        "constant",
			},
			Span:    functionSpan(loaded, constant.Pos()),
			RuleIDs: []string{},
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Identity.ObjectName < entries[j].Identity.ObjectName
	})
	return entries
}

func functionSpan(loaded *loadedModule, pos token.Pos) reportv2.SourceSpan {
	position := loaded.Fset.Position(pos)
	if position.Filename == "" {
		return reportv2.SourceSpan{
			FileRelativePath: "unknown",
			ByteOffsetStart:  0,
			ByteOffsetEnd:    1,
			LineStart:        1,
			LineEnd:          1,
		}
	}
	return reportSpan(Span{Filename: position.Filename, Line: position.Line, EndLine: position.Line}, loaded.ModuleRoot)
}

func functionObjectName(fn *ssa.Function) string {
	if recv := fn.Signature.Recv(); recv != nil {
		return receiverName(recv.Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func receiverName(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Pointer:
		return "(*" + receiverName(t.Elem()) + ")"
	case *types.Named:
		return t.Obj().Name()
	default:
		return types.TypeString(typ, func(*types.Package) string { return "" })
	}
}

func functionKind(fn *ssa.Function) string {
	if fn.Signature.Recv() != nil {
		return "method"
	}
	return "function"
}

func typeKind(named *types.Named) string {
	if _, ok := named.Underlying().(*types.Interface); ok {
		return "interface"
	}
	return "type"
}

func functionModuleAndPackage(loaded *loadedModule, fn *ssa.Function) (string, string) {
	if fn.Package() == nil || fn.Package().Pkg == nil {
		return loaded.RootPkg.Module.Path, loaded.RootPkg.PkgPath
	}
	return packageModuleAndPath(loaded, fn.Package())
}

func packageModuleAndPath(loaded *loadedModule, pkg *ssa.Package) (string, string) {
	if pkg == nil || pkg.Pkg == nil {
		return loaded.RootPkg.Module.Path, loaded.RootPkg.PkgPath
	}
	return modulePathForTypesPackage(loaded, pkg.Pkg), pkg.Pkg.Path()
}

func modulePathForTypesPackage(loaded *loadedModule, pkg *types.Package) string {
	for _, candidate := range loaded.Packages {
		if candidate.Types == nil || pkg == nil || candidate.Types.Path() != pkg.Path() {
			continue
		}
		if candidate.Module != nil && candidate.Module.Path != "" {
			return candidate.Module.Path
		}
	}
	if loaded.RootPkg.Module != nil {
		return loaded.RootPkg.Module.Path
	}
	return loaded.RootPkg.PkgPath
}

func symbolEntryForIdentity(identity reportv2.SymbolIdentity, span reportv2.SourceSpan) reportv2.SymbolEntry {
	return reportv2.SymbolEntry{Identity: identity, Span: span, RuleIDs: []string{}}
}

func addIncludedEntry(entries map[string]reportv2.SymbolEntry, entry reportv2.SymbolEntry) {
	if entry.Identity.ObjectName == "" || strings.HasPrefix(entry.Identity.PackagePath, "unsafe") || entry.Identity.PackagePath == "" {
		return
	}
	key := entry.Identity.PackagePath + "|" + entry.Identity.ObjectName + "|" + entry.Identity.Kind
	entries[key] = entry
}

func addExcludedEntry(entries map[string]reportv2.SymbolEntry, entry reportv2.SymbolEntry) {
	if entry.Identity.ObjectName == "" || entry.Identity.PackagePath == "" {
		return
	}
	key := entry.Identity.PackagePath + "|" + entry.Identity.ObjectName + "|" + entry.Identity.Kind
	entries[key] = entry
}

func sortedEntries(entries map[string]reportv2.SymbolEntry) []reportv2.SymbolEntry {
	out := make([]reportv2.SymbolEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Identity.PackagePath == out[j].Identity.PackagePath {
			if out[i].Identity.ObjectName == out[j].Identity.ObjectName {
				return out[i].Identity.Kind < out[j].Identity.Kind
			}
			return out[i].Identity.ObjectName < out[j].Identity.ObjectName
		}
		return out[i].Identity.PackagePath < out[j].Identity.PackagePath
	})
	return out
}

func buildWiringPaths(loaded *loadedModule, rootEntry reportv2.SymbolEntry, roots []*ssa.Function) []reportv2.WiringPath {
	paths := map[string]reportv2.WiringPath{}
	if len(roots) == 0 {
		paths[wiringPathKey(rootEntry.Identity)] = reportv2.WiringPath{
			Target: rootEntry.Identity,
			Steps:  []reportv2.SymbolEntry{rootEntry},
		}
	}
	for _, fn := range roots {
		if fn == nil {
			continue
		}
		targetEntry := symbolEntryForFunction(loaded, fn)
		steps := []reportv2.SymbolEntry{rootEntry}
		if !sameIdentity(rootEntry.Identity, targetEntry.Identity) {
			steps = append(steps, targetEntry)
		}
		paths[wiringPathKey(targetEntry.Identity)] = reportv2.WiringPath{
			Target: targetEntry.Identity,
			Steps:  steps,
		}
	}

	out := make([]reportv2.WiringPath, 0, len(paths))
	for _, path := range paths {
		out = append(out, path)
	}
	sort.Slice(out, func(i, j int) bool {
		return compareIdentities(out[i].Target, out[j].Target) < 0
	})
	return out
}

func wiringPathKey(identity reportv2.SymbolIdentity) string {
	return identity.PackagePath + "|" + identity.ObjectName + "|" + identity.Kind
}

func sameIdentity(left, right reportv2.SymbolIdentity) bool {
	return left.ModulePath == right.ModulePath &&
		left.PackagePath == right.PackagePath &&
		left.ObjectName == right.ObjectName &&
		left.Kind == right.Kind
}

func compareIdentities(left, right reportv2.SymbolIdentity) int {
	if left.PackagePath != right.PackagePath {
		return strings.Compare(left.PackagePath, right.PackagePath)
	}
	if left.ObjectName != right.ObjectName {
		return strings.Compare(left.ObjectName, right.ObjectName)
	}
	if left.Kind != right.Kind {
		return strings.Compare(left.Kind, right.Kind)
	}
	return strings.Compare(left.ModulePath, right.ModulePath)
}

func buildDispatchPlan(built *builtProgram, roots []*ssa.Function, registryKey *string) dispatchPlan {
	triggers := map[string]bool{}
	if registryKey != nil {
		triggers["registry-key:"+*registryKey] = true
	}
	return dispatchPlan{
		SelectedGraph: dispatchGraph(built, roots, registryKey),
		CHAGraph:      built.CHAGraph,
		RegistryKey:   registryKey,
		triggers:      triggers,
	}
}

func sortedTriggerKeys(triggers map[string]bool) []string {
	out := make([]string, 0, len(triggers))
	for trigger := range triggers {
		out = append(out, trigger)
	}
	sort.Strings(out)
	return out
}

func dispatchTriggerID(caller *ssa.Function, method string) string {
	if caller == nil || caller.Package() == nil || caller.Package().Pkg == nil {
		return method
	}
	return caller.Package().Pkg.Path() + ":" + functionObjectName(caller) + ":" + method
}

func externalDepForEntry(root reportv2.Root, entry reportv2.SymbolEntry) reportv2.ExternalDep {
	dep := reportv2.ExternalDep{
		Identity:            entry.Identity,
		AccessPath:          entry.Identity.PackagePath,
		ConfigurationSource: "",
		StateEffectSummary:  []string{},
	}
	if root.RegistryKey != nil {
		dep.ConfigurationSource = "registry:" + *root.RegistryKey
	}
	return dep
}

func sortedFunctions(seen map[*ssa.Function]bool) []*ssa.Function {
	out := make([]*ssa.Function, 0, len(seen))
	for fn := range seen {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool {
		left := reportv2.SymbolIdentity{PackagePath: "", ObjectName: ""}
		right := reportv2.SymbolIdentity{PackagePath: "", ObjectName: ""}
		if out[i] != nil && out[i].Package() != nil && out[i].Package().Pkg != nil {
			left.PackagePath = out[i].Package().Pkg.Path()
			left.ObjectName = functionObjectName(out[i])
			left.Kind = functionKind(out[i])
		}
		if out[j] != nil && out[j].Package() != nil && out[j].Package().Pkg != nil {
			right.PackagePath = out[j].Package().Pkg.Path()
			right.ObjectName = functionObjectName(out[j])
			right.Kind = functionKind(out[j])
		}
		return compareIdentities(left, right) < 0
	})
	return out
}

func addExternalDep(deps map[string]reportv2.ExternalDep, dep reportv2.ExternalDep) {
	if dep.AccessPath == "" {
		return
	}
	deps[dep.AccessPath] = dep
}

func sortedExternalDeps(deps map[string]reportv2.ExternalDep) []reportv2.ExternalDep {
	out := make([]reportv2.ExternalDep, 0, len(deps))
	for _, dep := range deps {
		out = append(out, dep)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AccessPath < out[j].AccessPath
	})
	return out
}
