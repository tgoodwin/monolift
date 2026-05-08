package codegen

import (
	"bytes"
	"errors"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dave/dst"
	"github.com/dave/dst/decorator"
	"golang.org/x/tools/go/packages"
)

func renamedOriginalFunc(plan *Plan) string {
	return "monoliftOriginal" + plan.CutPoint.FuncName
}

// PatchCutFunction renames the original function declaration and writes the
// client stub file so that the stub takes the original name. All callers
// (same-package and cross-package) automatically route through the stub.
// stubContent is the rendered client stub source produced by RenderClient.
func PatchCutFunction(plan *Plan, stubContent []byte) (string, error) {
	if plan == nil {
		return "", errors.New("codegen: nil plan")
	}

	cutFile := absoluteCutFile(plan)
	pkg, err := loadCutPackage(plan, cutFile)
	if err != nil {
		return "", err
	}

	original, err := os.ReadFile(cutFile)
	if err != nil {
		return "", fmt.Errorf("read cut file: %w", err)
	}

	renamed, alreadyRenamed, err := renameFuncDecl(pkg, cutFile, plan)
	if err != nil {
		return "", err
	}

	if !alreadyRenamed {
		if err := writeAtomic(cutFile, renamed, 0644); err != nil {
			return "", err
		}
	}

	stubPath := plan.ClientPath
	if err := writeAtomic(stubPath, stubContent, 0644); err != nil {
		if !alreadyRenamed {
			_ = writeAtomic(cutFile, original, 0644)
		}
		return "", err
	}

	if err := verifyPatchedBuild(plan, pkg); err != nil {
		if !alreadyRenamed {
			_ = writeAtomic(cutFile, original, 0644)
		}
		_ = os.Remove(stubPath)
		return "", err
	}

	return cutFile, nil
}

func absoluteCutFile(plan *Plan) string {
	file := plan.CutPoint.File
	if filepath.IsAbs(file) || plan.SourceModuleRoot == "" {
		return filepath.Clean(file)
	}
	return filepath.Join(plan.SourceModuleRoot, file)
}

func loadCutPackage(plan *Plan, cutFile string) (*packages.Package, error) {
	cfg := &packages.Config{
		Dir: filepath.Dir(cutFile),
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
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("codegen: package containing %s not found", cutFile)
	}
	pkg := pkgs[0]
	if len(pkg.Errors) > 0 {
		return nil, fmt.Errorf("codegen: package load errors: %s", packageErrorString(pkg.Errors))
	}
	return pkg, nil
}

func renameFuncDecl(pkg *packages.Package, cutFile string, plan *Plan) ([]byte, bool, error) {
	originalName := plan.CutPoint.FuncName
	newName := renamedOriginalFunc(plan)

	var astFileIdx int = -1
	for i, file := range pkg.GoFiles {
		if sameSourceFile(file, cutFile) && i < len(pkg.Syntax) {
			astFileIdx = i
			break
		}
	}
	if astFileIdx < 0 {
		for i, file := range pkg.CompiledGoFiles {
			if sameSourceFile(file, cutFile) && i < len(pkg.Syntax) {
				astFileIdx = i
				break
			}
		}
	}
	if astFileIdx < 0 {
		return nil, false, fmt.Errorf("codegen: parsed syntax for %s not found", cutFile)
	}
	astFile := pkg.Syntax[astFileIdx]

	dec := decorator.NewDecorator(pkg.Fset)
	dstFile, err := dec.DecorateFile(astFile)
	if err != nil {
		return nil, false, fmt.Errorf("decorate cut file: %w", err)
	}

	found := false
	alreadyRenamed := false
	for _, decl := range dstFile.Decls {
		fn, ok := decl.(*dst.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if fn.Name.Name == originalName {
			fn.Name.Name = newName
			found = true
			break
		}
		if fn.Name.Name == newName {
			alreadyRenamed = true
			found = true
			break
		}
	}
	if !found {
		return nil, false, fmt.Errorf("codegen: function %s not found in %s", originalName, cutFile)
	}
	if alreadyRenamed {
		return nil, true, nil
	}

	var out bytes.Buffer
	restorer := decorator.NewRestorer()
	if err := restorer.Fprint(&out, dstFile); err != nil {
		return nil, false, fmt.Errorf("restore renamed file: %w", err)
	}
	return out.Bytes(), false, nil
}

func verifyPatchedBuild(plan *Plan, pkg *packages.Package) error {
	dir := plan.SourceModuleRoot
	if dir == "" {
		dir = filepath.Dir(absoluteCutFile(plan))
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
