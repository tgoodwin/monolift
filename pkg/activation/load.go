package activation

import (
	"context"
	"fmt"
	"go/token"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// PackageLoadError aggregates go/packages load errors for miss reporting.
type PackageLoadError struct {
	Errors []packages.Error
}

func (e *PackageLoadError) Error() string {
	if e == nil || len(e.Errors) == 0 {
		return "package load failed"
	}
	parts := make([]string, 0, len(e.Errors))
	for _, loadErr := range e.Errors {
		parts = append(parts, loadErr.Error())
	}
	return "package load failed: " + strings.Join(parts, "; ")
}

// LoadProgram loads the configured package patterns using go/packages.
func (c Config) LoadProgram() (*Program, error) {
	patterns := c.Packages
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	fset := token.NewFileSet()
	ctx := c.Context
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := &packages.Config{
		Context:    ctx,
		Dir:        c.Dir,
		Env:        append(os.Environ(), c.Env...),
		Fset:       fset,
		BuildFlags: c.BuildFlags,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypes |
			packages.NeedSyntax |
			packages.NeedTypesInfo |
			packages.NeedModule,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load package patterns %v: %w", patterns, err)
	}
	var loadErrs []packages.Error
	seen := map[*packages.Package]bool{}
	for _, pkg := range pkgs {
		loadErrs = appendPackageErrors(loadErrs, pkg, seen)
	}
	if len(loadErrs) > 0 {
		return nil, &PackageLoadError{Errors: loadErrs}
	}
	return &Program{Fset: fset, Packages: pkgs}, nil
}

func appendPackageErrors(dst []packages.Error, pkg *packages.Package, seen map[*packages.Package]bool) []packages.Error {
	if pkg == nil {
		return dst
	}
	if seen[pkg] {
		return dst
	}
	seen[pkg] = true
	dst = append(dst, pkg.Errors...)
	for _, imp := range pkg.Imports {
		dst = appendPackageErrors(dst, imp, seen)
	}
	return dst
}
