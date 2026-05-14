package liftability

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

type Detector interface {
	ID() PropertyID
	Evaluate(ctx *Context, op Operation) (Verdict, []Evidence, error)
}

type Operation struct {
	Identity  reportv2.SymbolIdentity
	Signature *types.Signature
	Function  *ssa.Function
}

type Context struct {
	Loaded    *extract.LoadedModule
	Program   *ssa.Program
	CallGraph *callgraph.Graph
	cache     *contextCache
}

type contextCache struct {
	mu             sync.RWMutex
	factCache      map[*ssa.Function]*functionFacts
	callgraphCache map[*ssa.Function]callgraphFacts
}

func NewContext(loaded *extract.LoadedModule, program *ssa.Program) (*Context, error) {
	return NewContextWithCallgraph(loaded, program, extract.CallGraphForProgram(program))
}

func NewContextWithCallgraph(loaded *extract.LoadedModule, program *ssa.Program, graph *callgraph.Graph) (*Context, error) {
	if loaded == nil {
		return nil, fmt.Errorf("liftability: loaded module is nil")
	}
	if program == nil {
		return nil, fmt.Errorf("liftability: program is nil")
	}
	return &Context{
		Loaded:    loaded,
		Program:   program,
		CallGraph: graph,
		cache: &contextCache{
			factCache:      map[*ssa.Function]*functionFacts{},
			callgraphCache: map[*ssa.Function]callgraphFacts{},
		},
	}, nil
}

func Classify(loaded *extract.LoadedModule, program *ssa.Program, root reportv2.Root) (Result, error) {
	ctx, err := NewContext(loaded, program)
	if err != nil {
		return Result{}, err
	}
	return ClassifyWithContext(ctx, root)
}

func ClassifyWithContext(ctx *Context, root reportv2.Root) (Result, error) {
	if ctx == nil {
		return Result{}, fmt.Errorf("liftability: context is nil")
	}
	loaded := ctx.Loaded
	if loaded == nil {
		return Result{}, fmt.Errorf("liftability: loaded module is nil")
	}
	ops, err := exposedOperations(loaded, root)
	if err != nil {
		return Result{}, err
	}
	registry := DefaultRegistry()
	perOperation := make([]Classification, 0, len(ops))
	diagnostics := make([]extract.Diagnostic, 0, len(ops))
	for _, identity := range ops {
		op, err := resolveOperation(loaded, ctx.Program, identity)
		if err != nil {
			return Result{}, err
		}
		classification, opDiagnostics, err := evaluateOperation(ctx, registry, op)
		if err != nil {
			return Result{}, err
		}
		perOperation = append(perOperation, classification)
		if emitOperationDiagnostics(loaded) {
			diagnostics = append(diagnostics, opDiagnostics...)
		}
	}
	sort.Slice(perOperation, func(i, j int) bool {
		return identityKey(perOperation[i].Operation) < identityKey(perOperation[j].Operation)
	})
	rootClassification, rootDiagnostics := aggregateRoot(loaded, root, perOperation)
	diagnostics = append(diagnostics, rootDiagnostics...)
	sortExtractDiagnostics(diagnostics)
	return Result{
		Root:         rootClassification,
		PerOperation: perOperation,
		Diagnostics:  diagnostics,
	}, nil
}

func evaluateOperation(ctx *Context, registry []Detector, op Operation) (Classification, []extract.Diagnostic, error) {
	properties := make([]Evidence, 0, len(registry))
	for _, detector := range registry {
		_, evidence, err := detector.Evaluate(ctx, op)
		if err != nil {
			return Classification{}, nil, err
		}
		properties = append(properties, evidence...)
	}
	sortEvidence(properties)
	classification, diagnostics := decideAdmission(ctx.Loaded, op, properties)
	return classification, diagnostics, nil
}

func ForExtract(loaded *extract.LoadedModule, program *ssa.Program, root reportv2.Root) (extract.LiftabilityResult, error) {
	ctx, err := NewContext(loaded, program)
	if err != nil {
		return extract.LiftabilityResult{}, err
	}
	return ForExtractWithContext(ctx, root)
}

func ForExtractWithContext(ctx *Context, root reportv2.Root) (extract.LiftabilityResult, error) {
	result, err := ClassifyWithContext(ctx, root)
	if err != nil {
		return extract.LiftabilityResult{}, err
	}
	return toExtractResult(result), nil
}

func (ctx *Context) WithLoaded(loaded *extract.LoadedModule) (*Context, error) {
	if ctx == nil {
		return nil, fmt.Errorf("liftability: context is nil")
	}
	if loaded == nil {
		return nil, fmt.Errorf("liftability: loaded module is nil")
	}
	if ctx.cache == nil {
		return nil, fmt.Errorf("liftability: context cache is nil")
	}
	return &Context{
		Loaded:    loaded,
		Program:   ctx.Program,
		CallGraph: ctx.CallGraph,
		cache:     ctx.cache,
	}, nil
}

func (ctx *Context) factFor(fn *ssa.Function) (*functionFacts, bool) {
	if ctx == nil || ctx.cache == nil {
		return nil, false
	}
	ctx.cache.mu.RLock()
	defer ctx.cache.mu.RUnlock()
	facts, ok := ctx.cache.factCache[fn]
	return facts, ok
}

func (ctx *Context) storeFact(fn *ssa.Function, facts *functionFacts) {
	if ctx == nil || ctx.cache == nil {
		return
	}
	ctx.cache.mu.Lock()
	defer ctx.cache.mu.Unlock()
	if _, ok := ctx.cache.factCache[fn]; !ok {
		ctx.cache.factCache[fn] = facts
	}
}

func (ctx *Context) callgraphFactFor(fn *ssa.Function) (callgraphFacts, bool) {
	if ctx == nil || ctx.cache == nil {
		return callgraphFacts{}, false
	}
	ctx.cache.mu.RLock()
	defer ctx.cache.mu.RUnlock()
	facts, ok := ctx.cache.callgraphCache[fn]
	return facts, ok
}

func (ctx *Context) storeCallgraphFact(fn *ssa.Function, facts callgraphFacts) {
	if ctx == nil || ctx.cache == nil {
		return
	}
	ctx.cache.mu.Lock()
	defer ctx.cache.mu.Unlock()
	if _, ok := ctx.cache.callgraphCache[fn]; !ok {
		ctx.cache.callgraphCache[fn] = facts
	}
}

func toExtractResult(result Result) extract.LiftabilityResult {
	return extract.LiftabilityResult{
		Root:         toExtractClassification(result.Root),
		PerOperation: toExtractClassifications(result.PerOperation),
		Diagnostics:  append([]extract.Diagnostic(nil), result.Diagnostics...),
	}
}

func toExtractClassifications(items []Classification) []extract.LiftabilityClassification {
	out := make([]extract.LiftabilityClassification, 0, len(items))
	for _, item := range items {
		out = append(out, toExtractClassification(item))
	}
	return out
}

func toExtractClassification(item Classification) extract.LiftabilityClassification {
	return extract.LiftabilityClassification{
		Operation:   item.Operation,
		Admission:   string(item.Admission),
		Properties:  toReportEvidence(item.Properties),
		RefusalCode: item.RefusalCode,
	}
}

func exposedOperations(loaded *extract.LoadedModule, root reportv2.Root) ([]reportv2.SymbolIdentity, error) {
	if len(root.ExposedOperations) > 0 {
		out := append([]reportv2.SymbolIdentity(nil), root.ExposedOperations...)
		sort.Slice(out, func(i, j int) bool {
			return identityKey(out[i]) < identityKey(out[j])
		})
		return out, nil
	}
	if root.Identity.Kind == "method" {
		if strings.Contains(root.Identity.ObjectName, ".") {
			return []reportv2.SymbolIdentity{root.Identity}, nil
		}
		op, err := resolveDirectMethodIdentity(loaded, root.Identity)
		if err != nil {
			return nil, err
		}
		return []reportv2.SymbolIdentity{op}, nil
	}
	if root.Identity.Kind == "function" {
		return []reportv2.SymbolIdentity{root.Identity}, nil
	}
	return []reportv2.SymbolIdentity{}, nil
}

func resolveOperation(loaded *extract.LoadedModule, program *ssa.Program, op reportv2.SymbolIdentity) (Operation, error) {
	identity := op
	if op.Kind == "method" && !strings.Contains(op.ObjectName, ".") {
		resolved, err := resolveDirectMethodIdentity(loaded, op)
		if err != nil {
			return Operation{}, err
		}
		identity = resolved
	}

	switch identity.Kind {
	case "function":
		obj := loaded.RootPkg.Types.Scope().Lookup(identity.ObjectName)
		if obj == nil {
			return Operation{}, fmt.Errorf("liftability: root function %q not found", identity.ObjectName)
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			return Operation{}, fmt.Errorf("liftability: root function %q resolved to %T", identity.ObjectName, obj)
		}
		signature, ok := fn.Type().(*types.Signature)
		if !ok {
			return Operation{}, fmt.Errorf("liftability: function %q does not have a signature", identity.ObjectName)
		}
		return Operation{
			Identity:  identity,
			Signature: signature,
			Function:  lookupSSAFunction(program, loaded.RootPkg.Types, identity.ObjectName),
		}, nil
	case "method":
		selection, err := lookupMethodSelection(loaded, identity.ObjectName)
		if err != nil {
			return Operation{}, err
		}
		fn, ok := selection.Obj().(*types.Func)
		if !ok {
			return Operation{}, fmt.Errorf("liftability: method %q resolved to %T", identity.ObjectName, selection.Obj())
		}
		signature, ok := fn.Type().(*types.Signature)
		if !ok {
			return Operation{}, fmt.Errorf("liftability: method %q does not have a signature", identity.ObjectName)
		}
		return Operation{
			Identity:  identity,
			Signature: signature,
			Function:  program.MethodValue(selection),
		}, nil
	default:
		return Operation{}, fmt.Errorf("liftability: unsupported operation kind %q", identity.Kind)
	}
}

func resolveDirectMethodIdentity(loaded *extract.LoadedModule, identity reportv2.SymbolIdentity) (reportv2.SymbolIdentity, error) {
	funcDecl := findRootMethodDecl(loaded)
	if funcDecl == nil || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
		return reportv2.SymbolIdentity{}, fmt.Errorf("liftability: could not resolve receiver for root method %q", identity.ObjectName)
	}
	recvType := receiverTypeName(funcDecl.Recv.List[0].Type)
	if recvType == "" {
		return reportv2.SymbolIdentity{}, fmt.Errorf("liftability: unsupported receiver expression for root method %q", identity.ObjectName)
	}
	objName := recvType + "." + identity.ObjectName
	if _, ok := funcDecl.Recv.List[0].Type.(*ast.StarExpr); ok {
		objName = "(*" + recvType + ")." + identity.ObjectName
	}
	identity.ObjectName = objName
	return identity, nil
}

func findRootMethodDecl(loaded *extract.LoadedModule) *ast.FuncDecl {
	for _, file := range loaded.RootPkg.Syntax {
		filePath := loaded.Fset.Position(file.Pos()).Filename
		if !samePath(filePath, loaded.RootFile) {
			continue
		}
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Doc == nil || funcDecl.Name == nil || funcDecl.Name.Name != loaded.RootPragma.DeclName {
				continue
			}
			for _, comment := range funcDecl.Doc.List {
				pos := loaded.Fset.Position(comment.Slash)
				if samePath(pos.Filename, loaded.RootFile) && pos.Line == loaded.RootPragma.Span.Line {
					return funcDecl
				}
			}
		}
	}
	return nil
}

func receiverTypeName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		if ident, ok := typed.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

func lookupSSAFunction(program *ssa.Program, pkg *types.Package, name string) *ssa.Function {
	ssaPkg := program.Package(pkg)
	if ssaPkg == nil {
		return nil
	}
	member := ssaPkg.Members[name]
	fn, _ := member.(*ssa.Function)
	return fn
}

func lookupMethodSelection(loaded *extract.LoadedModule, objectName string) (*types.Selection, error) {
	typeName, methodName, pointerRecv, ok := parseMethodObjectName(objectName)
	if !ok {
		return nil, fmt.Errorf("liftability: could not parse method identity %q", objectName)
	}
	obj := loaded.RootPkg.Types.Scope().Lookup(typeName)
	if obj == nil {
		return nil, fmt.Errorf("liftability: receiver type %q not found", typeName)
	}
	typeNameObj, ok := obj.(*types.TypeName)
	if !ok {
		return nil, fmt.Errorf("liftability: receiver %q resolved to %T", typeName, obj)
	}
	named, ok := typeNameObj.Type().(*types.Named)
	if !ok {
		return nil, fmt.Errorf("liftability: receiver %q is not a named type", typeName)
	}
	if iface, ok := named.Underlying().(*types.Interface); ok {
		iface.Complete()
		for i := 0; i < iface.NumMethods(); i++ {
			method := iface.Method(i)
			if method.Name() == methodName {
				return types.NewMethodSet(named).Lookup(loaded.RootPkg.Types, methodName), nil
			}
		}
		return nil, fmt.Errorf("liftability: interface method %q not found on %s", methodName, typeName)
	}
	if pointerRecv {
		if selection := types.NewMethodSet(types.NewPointer(named)).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
			return selection, nil
		}
	}
	if selection := types.NewMethodSet(named).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
		return selection, nil
	}
	if selection := types.NewMethodSet(types.NewPointer(named)).Lookup(loaded.RootPkg.Types, methodName); selection != nil {
		return selection, nil
	}
	return nil, fmt.Errorf("liftability: method %q not found on %s", methodName, typeName)
}

func parseMethodObjectName(name string) (typeName, methodName string, pointerRecv, ok bool) {
	if strings.HasPrefix(name, "(*") {
		right := strings.Index(name, ").")
		if right < 0 {
			return "", "", false, false
		}
		return name[2:right], name[right+2:], true, true
	}
	left := strings.LastIndex(name, ".")
	if left < 0 {
		return "", "", false, false
	}
	return name[:left], name[left+1:], false, true
}

func contextContextType(loaded *extract.LoadedModule) (types.Type, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "context")
	if pkg == nil {
		return nil, false
	}
	obj := pkg.Scope().Lookup("Context")
	if obj == nil {
		return nil, false
	}
	return obj.Type(), true
}

func netHTTPTypes(loaded *extract.LoadedModule) (types.Type, types.Type, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "net/http")
	if pkg == nil {
		return nil, nil, false
	}
	responseWriter := pkg.Scope().Lookup("ResponseWriter")
	request := pkg.Scope().Lookup("Request")
	if responseWriter == nil || request == nil {
		return nil, nil, false
	}
	return responseWriter.Type(), types.NewPointer(request.Type()), true
}

func caddyServeHTTPSignature(loaded *extract.LoadedModule) (*types.Signature, bool) {
	pkg := findImportedTypesPackage(loaded.RootPkg, "github.com/caddyserver/caddy/v2/modules/caddyhttp")
	if pkg == nil {
		return nil, false
	}
	obj := pkg.Scope().Lookup("MiddlewareHandler")
	if obj == nil {
		return nil, false
	}
	named, ok := obj.Type().(*types.Named)
	if !ok {
		return nil, false
	}
	iface, ok := named.Underlying().(*types.Interface)
	if !ok {
		return nil, false
	}
	iface.Complete()
	for i := 0; i < iface.NumMethods(); i++ {
		method := iface.Method(i)
		if method.Name() == "ServeHTTP" {
			signature, ok := method.Type().(*types.Signature)
			return signature, ok
		}
	}
	return nil, false
}

func isErrorType(typ types.Type) bool {
	errorType := types.Universe.Lookup("error")
	return errorType != nil && types.Identical(typ, errorType.Type())
}

func findImportedTypesPackage(root *packages.Package, target string) *types.Package {
	seen := map[string]bool{}
	var walk func(pkg *packages.Package) *types.Package
	walk = func(pkg *packages.Package) *types.Package {
		if pkg == nil {
			return nil
		}
		key := pkg.PkgPath
		if key == "" {
			key = pkg.ID
		}
		if seen[key] {
			return nil
		}
		seen[key] = true
		if pkg.Types != nil && pkg.Types.Path() == target {
			return pkg.Types
		}
		for _, imported := range pkg.Imports {
			if found := walk(imported); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(root)
}

func identityKey(identity reportv2.SymbolIdentity) string {
	return identity.ModulePath + "|" + identity.PackagePath + "|" + identity.Kind + "|" + identity.ObjectName
}

func emitOperationDiagnostics(loaded *extract.LoadedModule) bool {
	return loaded.RootPragma.Surface == extract.SurfaceFunction ||
		loaded.RootPragma.Surface == extract.SurfaceMethod ||
		loaded.RootPragma.Options["methods"] != ""
}

func samePath(left, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}

func sortExtractDiagnostics(diags []extract.Diagnostic) {
	sort.Slice(diags, func(i, j int) bool {
		if diags[i].Code != diags[j].Code {
			return diags[i].Code < diags[j].Code
		}
		if diags[i].Span.Filename != diags[j].Span.Filename {
			return diags[i].Span.Filename < diags[j].Span.Filename
		}
		if diags[i].Span.Line != diags[j].Span.Line {
			return diags[i].Span.Line < diags[j].Span.Line
		}
		return diags[i].Message < diags[j].Message
	})
}
