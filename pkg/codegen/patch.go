package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

func PatchCallsite(plan *Plan) (string, error) {
	if plan == nil {
		return "", errors.New("codegen: nil plan")
	}
	if plan.Incoming.File == "" || plan.Incoming.Line == 0 {
		return "", errors.New("codegen: incoming callsite position is required")
	}
	pkg, astFile, err := loadCallsitePackage(plan)
	if err != nil {
		return "", err
	}

	dec := decorator.NewDecorator(pkg.Fset)
	dstFile, err := dec.DecorateFile(astFile)
	if err != nil {
		return "", fmt.Errorf("decorate callsite file: %w", err)
	}
	match, err := findCallsiteMatch(plan, pkg, dec, dstFile)
	if err != nil {
		return "", err
	}
	if !match.AlreadyPatched {
		if err := rewriteCall(match.Call, stubFuncName(plan)); err != nil {
			return "", err
		}
	}

	var out bytes.Buffer
	restorer := decorator.NewRestorer()
	if err := restorer.Fprint(&out, dstFile); err != nil {
		return "", fmt.Errorf("restore patched callsite: %w", err)
	}

	path := absoluteIncomingFile(plan)
	original, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(original, out.Bytes()) {
		if err := writeAtomic(path, out.Bytes(), 0644); err != nil {
			return "", err
		}
	}
	if err := verifyPatchedPackageBuild(plan, pkg); err != nil {
		if !bytes.Equal(original, out.Bytes()) {
			_ = writeAtomic(path, original, 0644)
		}
		return "", err
	}
	return path, nil
}

type callsiteMatch struct {
	Call           *dst.CallExpr
	AlreadyPatched bool
	Position       token.Position
}

func loadCallsitePackage(plan *Plan) (*packages.Package, *ast.File, error) {
	path := absoluteIncomingFile(plan)
	cfg := &packages.Config{
		Dir: filepath.Dir(path),
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedModule,
		Fset: token.NewFileSet(),
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, nil, err
	}
	if len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("codegen: package containing %s not found", path)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, nil, fmt.Errorf("codegen: package containing %s has load errors: %s", path, packageErrorString(pkg.Errors))
	}
	for i, file := range pkg.GoFiles {
		if sameSourceFile(file, path) && i < len(pkg.Syntax) {
			return pkg, pkg.Syntax[i], nil
		}
	}
	for i, file := range pkg.CompiledGoFiles {
		if sameSourceFile(file, path) && i < len(pkg.Syntax) {
			return pkg, pkg.Syntax[i], nil
		}
	}
	return nil, nil, fmt.Errorf("codegen: parsed syntax for %s not found", path)
}

func findCallsiteMatch(plan *Plan, pkg *packages.Package, dec *decorator.Decorator, file *dst.File) (callsiteMatch, error) {
	var matches []callsiteMatch
	dst.Inspect(file, func(node dst.Node) bool {
		call, ok := node.(*dst.CallExpr)
		if !ok {
			return true
		}
		astCall, ok := dec.Ast.Nodes[call].(*ast.CallExpr)
		if !ok {
			return true
		}
		start := pkg.Fset.Position(astCall.Pos())
		end := pkg.Fset.Position(astCall.End())
		if !sameSourceFile(start.Filename, absoluteIncomingFile(plan)) || !containsIncoming(start, end, plan.Incoming) {
			return true
		}
		fn := resolvedCallFunction(pkg.TypesInfo, astCall.Fun)
		if fn == nil {
			return true
		}
		switch {
		case matchesCutFunction(plan, fn):
			matches = append(matches, callsiteMatch{Call: call, Position: start})
		case matchesStubFunction(plan, fn):
			matches = append(matches, callsiteMatch{Call: call, AlreadyPatched: true, Position: start})
		}
		return true
	})
	if len(matches) == 0 {
		return callsiteMatch{}, fmt.Errorf("codegen: no call to %s.%s found at %s:%d", plan.CutPoint.PackagePath, plan.CutPoint.FuncName, plan.Incoming.File, plan.Incoming.Line)
	}
	if len(matches) > 1 {
		var positions []string
		for _, match := range matches {
			positions = append(positions, fmt.Sprintf("%s:%d:%d", match.Position.Filename, match.Position.Line, match.Position.Column))
		}
		return callsiteMatch{}, fmt.Errorf("codegen: ambiguous callsite at %s:%d; matches: %s", plan.Incoming.File, plan.Incoming.Line, strings.Join(positions, ", "))
	}
	return matches[0], nil
}

func rewriteCall(call *dst.CallExpr, name string) error {
	switch fun := call.Fun.(type) {
	case *dst.Ident:
		fun.Name = name
		return nil
	case *dst.SelectorExpr:
		if fun.Sel == nil {
			return errors.New("codegen: selector call has nil selector")
		}
		fun.Sel.Name = name
		return nil
	case *dst.ParenExpr:
		wrapped := &dst.CallExpr{Fun: fun.X}
		if err := rewriteCall(wrapped, name); err != nil {
			return err
		}
		fun.X = wrapped.Fun
		return nil
	default:
		return fmt.Errorf("codegen: unsupported call expression %T", call.Fun)
	}
}

func resolvedCallFunction(info *types.Info, fun ast.Expr) *types.Func {
	if info == nil || fun == nil {
		return nil
	}
	switch expr := fun.(type) {
	case *ast.Ident:
		fn, _ := info.Uses[expr].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		if sel := info.Selections[expr]; sel != nil {
			fn, _ := sel.Obj().(*types.Func)
			return fn
		}
		fn, _ := info.Uses[expr.Sel].(*types.Func)
		return fn
	case *ast.ParenExpr:
		return resolvedCallFunction(info, expr.X)
	default:
		return nil
	}
}

func matchesCutFunction(plan *Plan, fn *types.Func) bool {
	return matchesFunction(plan, fn, plan.CutPoint.FuncName)
}

func matchesStubFunction(plan *Plan, fn *types.Func) bool {
	return matchesFunction(plan, fn, stubFuncName(plan))
}

func matchesFunction(plan *Plan, fn *types.Func, name string) bool {
	if fn == nil || fn.Name() != name {
		return false
	}
	if plan.CutPoint.PackagePath == "" {
		return true
	}
	pkg := fn.Pkg()
	return pkg != nil && pkg.Path() == plan.CutPoint.PackagePath
}

func containsIncoming(start, end token.Position, incoming IncomingCall) bool {
	if incoming.Line < start.Line || incoming.Line > end.Line {
		return false
	}
	if incoming.Column == 0 {
		return true
	}
	if start.Line == end.Line {
		return start.Column <= incoming.Column && incoming.Column <= end.Column
	}
	if incoming.Line == start.Line {
		return incoming.Column >= start.Column
	}
	if incoming.Line == end.Line {
		return incoming.Column <= end.Column
	}
	return true
}

func verifyPatchedPackageBuild(plan *Plan, pkg *packages.Package) error {
	dir := plan.SourceModuleRoot
	if dir == "" {
		dir = filepath.Dir(absoluteIncomingFile(plan))
	}
	target := "."
	if pkg != nil && pkg.PkgPath != "" && pkg.PkgPath != "command-line-arguments" {
		target = pkg.PkgPath
	}
	cmd := exec.Command("go", "test", "-run=^$", "-count=1", target)
	cmd.Dir = dir
	cmd.Env = withEnvValue(os.Environ(), "GOCACHE", "/tmp/monolift-gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("codegen: patched package build failed: %w\n%s", err, out)
	}
	return nil
}

func absoluteIncomingFile(plan *Plan) string {
	file := plan.Incoming.File
	if filepath.IsAbs(file) || plan.SourceModuleRoot == "" {
		return filepath.Clean(file)
	}
	return filepath.Join(plan.SourceModuleRoot, file)
}

func sameSourceFile(a, b string) bool {
	absA, err := filepath.Abs(a)
	if err == nil {
		a = absA
	}
	absB, err := filepath.Abs(b)
	if err == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func packageErrorString(errs []packages.Error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Msg)
	}
	return strings.Join(parts, "; ")
}

func withEnvValue(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
