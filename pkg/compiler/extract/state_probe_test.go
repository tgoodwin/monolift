package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/ssa"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type stateProbeArtifact struct {
	Target                 string             `json:"target"`
	ModuleRoot             string             `json:"moduleRoot"`
	Root                   string             `json:"root"`
	ReachableFunctionCount int                `json:"reachableFunctionCount"`
	InterfaceMethodCount   int                `json:"interfaceMethodCount,omitempty"`
	BaseAppMethodCount     int                `json:"baseAppMethodCount,omitempty"`
	BaseAppDBFieldType     string             `json:"baseAppDbFieldType,omitempty"`
	Globals                []stateProbeSymbol `json:"globals"`
	ReceiverFields         []stateProbeSymbol `json:"receiverFields"`
}

type stateProbeSymbol struct {
	Symbol        string   `json:"symbol"`
	Type          string   `json:"type"`
	Referenced    bool     `json:"referenced"`
	StoreSites    []string `json:"storeSites"`
	SyncWitnesses []string `json:"syncWitnesses"`
}

func TestWriteCaddyStateProbe(t *testing.T) {
	if os.Getenv("MONOLIFT_SSA_PROBE") != "1" {
		t.Skip("MONOLIFT_SSA_PROBE=1 required")
	}

	loaded, built, root, reachable := loadProbeContext(t, Request{
		Sources: []string{filepath.Join("..", "..", "..", "evaluation", "caddy")},
		Pragmas: []Pragma{{
			Name:    "caddy-reverse-proxy",
			Surface: SurfaceStruct,
			Options: map[string]string{
				"name":     "caddy-reverse-proxy",
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: Span{
				Filename: filepath.Join("..", "..", "..", "evaluation", "caddy", "modules", "caddyhttp", "reverseproxy", "reverseproxy.go"),
				Line:     101,
				EndLine:  101,
			},
			DeclName: "Handler",
			DeclKind: "type",
		}},
	})

	rootNamed := mustNamedType(t, loaded.RootPkg.Types.Scope().Lookup("Handler"))
	fields, globals := probeReachableState(loaded, reachable, rootNamed)
	writeProbeArtifact(t, repoPath("docs", "research", "SPRINT-0007-caddy-state-probe.json"), stateProbeArtifact{
		Target:                 "caddy",
		ModuleRoot:             filepath.ToSlash(loaded.ModuleRoot),
		Root:                   root.Identity.PackagePath + ":" + root.Identity.ObjectName,
		ReachableFunctionCount: len(reachable),
		Globals:                globals,
		ReceiverFields:         fields,
	})

	_ = built
}

func TestWritePocketBaseStateProbe(t *testing.T) {
	if os.Getenv("MONOLIFT_SSA_PROBE") != "1" {
		t.Skip("MONOLIFT_SSA_PROBE=1 required")
	}

	loaded, built, _, _ := loadProbeContext(t, Request{
		Sources: []string{filepath.Join("..", "..", "..", "evaluation", "pocketbase")},
		Pragmas: []Pragma{{
			Name:    "pocketbase-app",
			Surface: SurfaceInterface,
			Options: map[string]string{
				"name":  "pocketbase-app",
				"state": "external",
			},
			Span: Span{
				Filename: filepath.Join("..", "..", "..", "evaluation", "pocketbase", "core", "app.go"),
				Line:     29,
				EndLine:  29,
			},
			DeclName: "App",
			DeclKind: "type",
		}},
	})

	baseAppNamed := mustNamedType(t, loaded.RootPkg.Types.Scope().Lookup("BaseApp"))
	baseAppRoot := reportv2.Root{
		Identity: reportv2.SymbolIdentity{
			ModulePath:  loaded.RootPkg.Module.Path,
			PackagePath: loaded.RootPkg.PkgPath,
			ObjectName:  "BaseApp",
			Kind:        "type",
		},
		ExposedOperations: exportedMethodSymbols(loaded, baseAppNamed),
	}
	reachable := buildClosure(loaded, built, baseAppRoot).ReachableFuncs

	fields, globals := probeReachableState(loaded, reachable, baseAppNamed)
	iface := mustInterface(t, mustNamedType(t, loaded.RootPkg.Types.Scope().Lookup("App")))
	dbFieldSummary := baseAppDBFieldSummary(baseAppNamed)
	writeProbeArtifact(t, repoPath("docs", "research", "SPRINT-0007-pocketbase-state-probe.json"), stateProbeArtifact{
		Target:                 "pocketbase",
		ModuleRoot:             filepath.ToSlash(loaded.ModuleRoot),
		Root:                   loaded.RootPkg.PkgPath + ":App",
		ReachableFunctionCount: len(reachable),
		InterfaceMethodCount:   iface.NumMethods(),
		BaseAppMethodCount:     len(baseAppRoot.ExposedOperations),
		BaseAppDBFieldType:     dbFieldSummary,
		Globals:                globals,
		ReceiverFields:         fields,
	})
}

func loadProbeContext(t *testing.T, req Request) (*loadedModule, *builtProgram, reportv2.Root, []*ssa.Function) {
	t.Helper()

	loaded, err := loadModule(req)
	if err != nil {
		t.Fatalf("loadModule: %v", err)
	}
	built, err := buildProgram(loaded)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	root := resolveRoot(loaded)
	reachable := buildClosure(loaded, built, root).ReachableFuncs
	return loaded, built, root, reachable
}

func probeReachableState(loaded *loadedModule, reachable []*ssa.Function, rootNamed *types.Named) ([]stateProbeSymbol, []stateProbeSymbol) {
	fieldFacts := map[int]*stateProbeSymbol{}
	if strct, ok := rootNamed.Underlying().(*types.Struct); ok {
		for i := 0; i < strct.NumFields(); i++ {
			field := strct.Field(i)
			fieldFacts[i] = &stateProbeSymbol{
				Symbol: rootNamed.Obj().Name() + "." + field.Name(),
				Type:   typeString(field.Type()),
			}
		}
	}
	globalFacts := map[string]*stateProbeSymbol{}

	for _, fn := range reachable {
		if fn == nil {
			continue
		}
		syncWitnesses := collectSyncWitnesses(loaded, fn)
		mutatedFields := map[int][]string{}
		mutatedGlobals := map[string][]string{}

		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if fieldIndex, ok := referencedRootField(instr, rootNamed); ok {
					entry := ensureProbeSymbol(fieldFacts[fieldIndex])
					entry.Referenced = true
					fieldFacts[fieldIndex] = entry
				}
				switch typed := instr.(type) {
				case *ssa.Store:
					if fieldIndex, ok := rootFieldMutation(typed.Addr, rootNamed); ok {
						mutatedFields[fieldIndex] = append(mutatedFields[fieldIndex], sourceSite(loaded, typed.Pos()))
					}
					if global, ok := typed.Addr.(*ssa.Global); ok {
						key := globalKey(global)
						entry := ensureGlobalProbe(globalFacts, key, global)
						mutatedGlobals[key] = append(mutatedGlobals[key], sourceSite(loaded, typed.Pos()))
						globalFacts[key] = entry
					}
				case *ssa.MapUpdate:
					if global, ok := typed.Map.(*ssa.Global); ok {
						key := globalKey(global)
						entry := ensureGlobalProbe(globalFacts, key, global)
						mutatedGlobals[key] = append(mutatedGlobals[key], sourceSite(loaded, typed.Pos()))
						globalFacts[key] = entry
					}
				}
				for _, operand := range instr.Operands(nil) {
					if operand == nil || *operand == nil {
						continue
					}
					if global, ok := (*operand).(*ssa.Global); ok {
						key := globalKey(global)
						entry := ensureGlobalProbe(globalFacts, key, global)
						entry.Referenced = true
						globalFacts[key] = entry
					}
				}
			}
		}

		for fieldIndex, sites := range mutatedFields {
			entry := ensureProbeSymbol(fieldFacts[fieldIndex])
			entry.StoreSites = append(entry.StoreSites, sites...)
			entry.SyncWitnesses = append(entry.SyncWitnesses, syncWitnesses...)
			fieldFacts[fieldIndex] = entry
		}
		for key, sites := range mutatedGlobals {
			entry := globalFacts[key]
			entry.StoreSites = append(entry.StoreSites, sites...)
			entry.SyncWitnesses = append(entry.SyncWitnesses, syncWitnesses...)
		}
	}

	return sortedProbeSymbols(fieldFacts), sortedGlobalSymbols(globalFacts)
}

func collectSyncWitnesses(loaded *loadedModule, fn *ssa.Function) []string {
	var witnesses []string
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok {
				continue
			}
			common := call.Common()
			callee := common.StaticCallee()
			if callee == nil {
				continue
			}
			pkgPath := ""
			if callee.Package() != nil && callee.Package().Pkg != nil {
				pkgPath = callee.Package().Pkg.Path()
			}
			if pkgPath == "sync/atomic" || pkgPath == "sync" {
				witnesses = append(witnesses, callee.String()+" @ "+sourceSite(loaded, instr.Pos()))
			}
		}
	}
	sort.Strings(witnesses)
	return compactStrings(witnesses)
}

func referencedRootField(instr ssa.Instruction, rootNamed *types.Named) (int, bool) {
	switch typed := instr.(type) {
	case *ssa.Field:
		if rootFieldOwner(typed.X.Type(), rootNamed) {
			return typed.Field, true
		}
	case *ssa.FieldAddr:
		if rootFieldOwner(typed.X.Type(), rootNamed) {
			return typed.Field, true
		}
	}
	return 0, false
}

func rootFieldMutation(addr ssa.Value, rootNamed *types.Named) (int, bool) {
	fieldAddr, ok := addr.(*ssa.FieldAddr)
	if !ok {
		return 0, false
	}
	if !rootFieldOwner(fieldAddr.X.Type(), rootNamed) {
		return 0, false
	}
	return fieldAddr.Field, true
}

func rootFieldOwner(typ types.Type, rootNamed *types.Named) bool {
	switch typed := typ.(type) {
	case *types.Pointer:
		return rootFieldOwner(typed.Elem(), rootNamed)
	case *types.Named:
		return typed.Obj() == rootNamed.Obj()
	default:
		return false
	}
}

func ensureGlobalProbe(facts map[string]*stateProbeSymbol, key string, global *ssa.Global) *stateProbeSymbol {
	if existing := facts[key]; existing != nil {
		return existing
	}
	entry := &stateProbeSymbol{
		Symbol: key,
		Type:   typeString(global.Type()),
	}
	facts[key] = entry
	return entry
}

func globalKey(global *ssa.Global) string {
	if global == nil || global.Package() == nil || global.Package().Pkg == nil {
		return "<unknown>"
	}
	return global.Package().Pkg.Path() + "." + global.Name()
}

func ensureProbeSymbol(symbol *stateProbeSymbol) *stateProbeSymbol {
	if symbol != nil {
		return symbol
	}
	return &stateProbeSymbol{}
}

func sortedProbeSymbols(facts map[int]*stateProbeSymbol) []stateProbeSymbol {
	var out []stateProbeSymbol
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		if fact.Symbol == "" {
			continue
		}
		if !fact.Referenced && len(fact.StoreSites) == 0 && len(fact.SyncWitnesses) == 0 {
			continue
		}
		fact.StoreSites = compactStrings(fact.StoreSites)
		fact.SyncWitnesses = compactStrings(fact.SyncWitnesses)
		out = append(out, *fact)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

func sortedGlobalSymbols(facts map[string]*stateProbeSymbol) []stateProbeSymbol {
	var out []stateProbeSymbol
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		fact.StoreSites = compactStrings(fact.StoreSites)
		fact.SyncWitnesses = compactStrings(fact.SyncWitnesses)
		out = append(out, *fact)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Symbol < out[j].Symbol
	})
	return out
}

func compactStrings(values []string) []string {
	sort.Strings(values)
	if len(values) == 0 {
		return nil
	}
	out := values[:0]
	var prev string
	for i, value := range values {
		if i == 0 || value != prev {
			out = append(out, value)
			prev = value
		}
	}
	return out
}

func sourceSite(loaded *loadedModule, pos token.Pos) string {
	position := loaded.Fset.Position(pos)
	if position.Filename == "" {
		return "unknown"
	}
	relative, err := filepath.Rel(loaded.ModuleRoot, position.Filename)
	if err != nil {
		relative = position.Filename
	}
	return filepath.ToSlash(relative) + ":" + intString(position.Line)
}

func intString(value int) string {
	return strconv.Itoa(value)
}

func typeString(typ types.Type) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Path()
	})
}

func exportedMethodSymbols(loaded *loadedModule, named *types.Named) []reportv2.SymbolIdentity {
	methodSet := types.NewMethodSet(types.NewPointer(named))
	var out []reportv2.SymbolIdentity
	for i := 0; i < methodSet.Len(); i++ {
		selection := methodSet.At(i)
		if selection == nil || selection.Obj() == nil || !selection.Obj().Exported() {
			continue
		}
		out = append(out, reportv2.SymbolIdentity{
			ModulePath:  loaded.RootPkg.Module.Path,
			PackagePath: loaded.RootPkg.PkgPath,
			ObjectName:  receiverName(types.NewPointer(named)) + "." + selection.Obj().Name(),
			Kind:        "method",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func mustNamedType(t *testing.T, obj types.Object) *types.Named {
	t.Helper()
	if obj == nil {
		t.Fatal("named type lookup returned nil")
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		t.Fatalf("object %s type = %T, want *types.Named", obj.Name(), obj.Type())
	}
	return named
}

func mustInterface(t *testing.T, named *types.Named) *types.Interface {
	t.Helper()
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		t.Fatalf("named type %s underlying = %T, want *types.Interface", named.Obj().Name(), named.Underlying())
	}
	return iface
}

func baseAppDBFieldSummary(named *types.Named) string {
	strct, ok := named.Underlying().(*types.Struct)
	if !ok {
		return "<non-struct>"
	}
	var matches []string
	for i := 0; i < strct.NumFields(); i++ {
		field := strct.Field(i)
		lower := strings.ToLower(field.Name())
		if lower == "db" || strings.Contains(lower, "db") {
			matches = append(matches, field.Name()+"="+typeString(field.Type()))
		}
	}
	sort.Strings(matches)
	if len(matches) == 0 {
		return "<absent>"
	}
	return strings.Join(matches, "; ")
}

func writeProbeArtifact(t *testing.T, path string, artifact stateProbeArtifact) {
	t.Helper()
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal probe: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func repoPath(parts ...string) string {
	prefix := []string{"..", "..", ".."}
	return filepath.Join(append(prefix, parts...)...)
}
