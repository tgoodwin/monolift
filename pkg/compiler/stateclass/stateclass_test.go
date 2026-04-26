package stateclass

import (
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"go/types"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type sharedFixtureEnv struct {
	once      sync.Once
	loaded    *extract.LoadedModule
	program   *ssa.Program
	functions map[*ssa.Function]bool
	callGraph *callgraph.Graph
	err       error
}

var (
	sharedFixtureEnvMu sync.Mutex
	sharedFixtureEnvs  = map[string]*sharedFixtureEnv{}
)

func TestInferExternalClientType(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("SQLRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 8))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassExternalizedDurable {
		t.Fatalf("items=%v want externalized-durable", result.Items)
	}
}

func TestInferSyncGuardedGlobal(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("SyncStore", extract.SurfaceFunction, nil, 15))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassSharedMutableAcross {
		t.Fatalf("items=%v want shared-mutable-across-callers", result.Items)
	}
}

func TestInferChannelLoop(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("Worker", extract.SurfaceStruct, map[string]string{"methods": "Start"}, 20))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassSingletonMutable {
		t.Fatalf("items=%v want singleton-mutable", result.Items)
	}
}

func TestInferMutationFreeCapturedConfig(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("ConfigRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 27))
	if len(result.Items) != 1 || result.Items[0].Classes[0] != ClassImmutableCapturedConfig {
		t.Fatalf("items=%v want immutable-captured-config", result.Items)
	}
}

func TestInferUnknownState(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("MutatingField", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 32))
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_STATE_UNKNOWN" {
		t.Fatalf("diagnostics=%v want MLV2_STATE_UNKNOWN", result.Diagnostics)
	}
}

func TestInferDeveloperDeclaredNarrowing(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("NoState", extract.SurfaceFunction, map[string]string{"state": "singleton"}, 35))
	if len(result.Items) != 1 || !result.Items[0].DeveloperDeclared {
		t.Fatalf("items=%v want developer-declared state", result.Items)
	}
}

func TestInferDeveloperDeclaredConflict(t *testing.T) {
	t.Parallel()

	result := inferFixture(t, fixturePragma("GlobalStore", extract.SurfaceFunction, map[string]string{"state": "stateless"}, 12))
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_STATE_DECL_CONFLICT" {
		t.Fatalf("diagnostics=%v want MLV2_STATE_DECL_CONFLICT", result.Diagnostics)
	}
}

func TestInferDeterministic(t *testing.T) {
	t.Parallel()

	req := fixturePragma("ConfigRoot", extract.SurfaceStruct, map[string]string{"methods": "Handle"}, 27)
	first := inferFixture(t, req)
	second := inferFixture(t, req)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("infer result differs between runs\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestInferCompositeEmbeddedDBRule(t *testing.T) {
	t.Parallel()

	previous := embeddedDBAppRootMethodThreshold
	embeddedDBAppRootMethodThreshold = 2
	defer func() { embeddedDBAppRootMethodThreshold = previous }()

	result := inferFixture(t, fixturePragma("DBApp", extract.SurfaceStruct, nil, 37))
	gotCodes := []string{}
	for _, diagnostic := range result.Diagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	if !reflect.DeepEqual(gotCodes, []string{"MLV2_CLOSURE_TOO_LARGE", "MLV2_EMBEDDED_DB_APP_ROOT"}) && !reflect.DeepEqual(gotCodes, []string{"MLV2_EMBEDDED_DB_APP_ROOT", "MLV2_CLOSURE_TOO_LARGE"}) {
		t.Fatalf("diagnostics=%v want embedded-db composite codes", gotCodes)
	}
	if len(result.Items) != 1 || result.Items[0].Disposition != "refused" {
		t.Fatalf("items=%v want refused externalized-durable row", result.Items)
	}
}

func inferFixture(t *testing.T, req extract.Request) Result {
	t.Helper()

	env := sharedFixtureEnvForRequest(req)
	env.once.Do(func() {
		env.loaded, env.err = extract.LoadModule(req)
		if env.err != nil {
			return
		}
		env.program, env.err = extract.BuildProgram(env.loaded)
		if env.err != nil {
			return
		}
		env.functions = ssautil.AllFunctions(env.program)
		env.callGraph = extract.CallGraphForProgram(env.program)
	})
	if env.err != nil {
		t.Fatalf("load shared fixture env: %v", env.err)
	}

	loaded, err := extract.RebindLoadedModule(env.loaded, req)
	if err != nil {
		t.Fatalf("RebindLoadedModule: %v", err)
	}
	root := extract.ResolveRoot(loaded)
	reachable := reachableFunctionsForRoot(loaded, env, root)
	result, err := Infer(loaded, env.program, reachable, root, &loaded.RootPragma)
	if err != nil {
		t.Fatalf("Infer: %v", err)
	}
	return result
}

func sharedFixtureEnvForRequest(req extract.Request) *sharedFixtureEnv {
	key := req.Sources[0]
	if abs, err := filepath.Abs(key); err == nil {
		key = abs
	}
	sharedFixtureEnvMu.Lock()
	defer sharedFixtureEnvMu.Unlock()
	env := sharedFixtureEnvs[key]
	if env == nil {
		env = &sharedFixtureEnv{}
		sharedFixtureEnvs[key] = env
	}
	return env
}

func reachableFunctionsForRoot(loaded *extract.LoadedModule, env *sharedFixtureEnv, root reportv2.Root) []*ssa.Function {
	queue := rootFunctionsForReachability(env.functions, root)
	visited := map[*ssa.Function]bool{}

	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if fn == nil || visited[fn] {
			continue
		}
		visited[fn] = true
		for _, anon := range fn.AnonFuncs {
			if isInternalFixtureFunction(loaded, anon) {
				queue = append(queue, anon)
			}
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				switch typed := instr.(type) {
				case *ssa.MakeClosure:
					if callee, ok := typed.Fn.(*ssa.Function); ok && isInternalFixtureFunction(loaded, callee) {
						queue = append(queue, callee)
					}
				case ssa.CallInstruction:
					for _, callee := range resolveReachableCallees(env.callGraph, fn, typed) {
						if isInternalFixtureFunction(loaded, callee) {
							queue = append(queue, callee)
						}
					}
				}
				for _, operand := range instr.Operands(nil) {
					if operand == nil || *operand == nil {
						continue
					}
					if callee, ok := (*operand).(*ssa.Function); ok && isInternalFixtureFunction(loaded, callee) {
						queue = append(queue, callee)
					}
				}
			}
		}
	}

	out := make([]*ssa.Function, 0, len(visited))
	for fn := range visited {
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool {
		return compareFunctionIdentity(out[i], out[j]) < 0
	})
	return out
}

func rootFunctionsForReachability(functions map[*ssa.Function]bool, root reportv2.Root) []*ssa.Function {
	symbols := append([]reportv2.SymbolIdentity(nil), root.ExposedOperations...)
	if len(symbols) == 0 {
		symbols = append(symbols, root.Identity)
	}
	out := make([]*ssa.Function, 0, len(symbols))
	seen := map[*ssa.Function]bool{}
	for _, symbol := range symbols {
		for fn := range functions {
			if fn == nil || fn.Package() == nil || fn.Package().Pkg == nil {
				continue
			}
			if fn.Package().Pkg.Path() != symbol.PackagePath {
				continue
			}
			if functionObjectName(fn) != symbol.ObjectName {
				continue
			}
			if !seen[fn] {
				seen[fn] = true
				out = append(out, fn)
			}
		}
	}
	return out
}

func resolveReachableCallees(graph *callgraph.Graph, caller *ssa.Function, call ssa.CallInstruction) []*ssa.Function {
	common := call.Common()
	if callee := common.StaticCallee(); callee != nil {
		return []*ssa.Function{callee}
	}
	if !common.IsInvoke() || graph == nil {
		return nil
	}
	node := graph.Nodes[caller]
	if node == nil {
		return nil
	}
	out := make([]*ssa.Function, 0, len(node.Out))
	seen := map[*ssa.Function]bool{}
	for _, edge := range node.Out {
		if edge.Site != call || edge.Callee == nil || edge.Callee.Func == nil || seen[edge.Callee.Func] {
			continue
		}
		seen[edge.Callee.Func] = true
		out = append(out, edge.Callee.Func)
	}
	return out
}

func isInternalFixtureFunction(loaded *extract.LoadedModule, fn *ssa.Function) bool {
	if fn == nil || fn.Package() == nil || fn.Package().Pkg == nil {
		return false
	}
	for _, pkg := range loaded.Packages {
		if pkg.PkgPath != fn.Package().Pkg.Path() || pkg.Module == nil || loaded.RootPkg.Module == nil {
			continue
		}
		return pkg.Module.Path == loaded.RootPkg.Module.Path
	}
	return false
}

func compareFunctionIdentity(left, right *ssa.Function) int {
	leftID := reportv2.SymbolIdentity{}
	rightID := reportv2.SymbolIdentity{}
	if left != nil && left.Package() != nil && left.Package().Pkg != nil {
		leftID.PackagePath = left.Package().Pkg.Path()
		leftID.ObjectName = functionObjectName(left)
		leftID.Kind = functionKind(left)
	}
	if right != nil && right.Package() != nil && right.Package().Pkg != nil {
		rightID.PackagePath = right.Package().Pkg.Path()
		rightID.ObjectName = functionObjectName(right)
		rightID.Kind = functionKind(right)
	}
	if leftID.PackagePath != rightID.PackagePath {
		return strings.Compare(leftID.PackagePath, rightID.PackagePath)
	}
	if leftID.ObjectName != rightID.ObjectName {
		return strings.Compare(leftID.ObjectName, rightID.ObjectName)
	}
	return strings.Compare(leftID.Kind, rightID.Kind)
}

func functionObjectName(fn *ssa.Function) string {
	if recv := fn.Signature.Recv(); recv != nil {
		return receiverName(recv.Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func functionKind(fn *ssa.Function) string {
	if fn.Signature.Recv() != nil {
		return "method"
	}
	return "function"
}

func receiverName(typ types.Type) string {
	switch typed := typ.(type) {
	case *types.Pointer:
		return "(*" + receiverName(typed.Elem()) + ")"
	case *types.Named:
		return typed.Obj().Name()
	default:
		return types.TypeString(typ, func(*types.Package) string { return "" })
	}
}

func fixturePragma(decl string, surface extract.Surface, options map[string]string, line int) extract.Request {
	rootFile := filepath.Join("testdata", "fixtures", "root.go")
	return extract.Request{
		Sources: []string{filepath.Join("testdata", "fixtures")},
		Pragmas: []extract.Pragma{{
			Name:     decl,
			Surface:  surface,
			DeclName: decl,
			DeclKind: string(surface),
			Options:  options,
			Span: extract.Span{
				Filename: rootFile,
				Line:     line,
				EndLine:  line,
			},
		}},
	}
}
