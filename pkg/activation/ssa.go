package activation

import (
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// BuildSSA builds SSA for the loaded package graph if it has not already been
// built.
func (p *Program) BuildSSA() {
	if p == nil || p.SSAProgram != nil {
		return
	}
	prog, pkgs := ssautil.AllPackages(p.Packages, ssa.InstantiateGenerics)
	prog.Build()
	p.SSAProgram = prog
	p.SSAPackages = pkgs
}

// FunctionKeyForSSA returns the canonical comparison key for an SSA function.
func FunctionKeyForSSA(fn *ssa.Function) FunctionKey {
	if fn == nil {
		return FunctionKey{}
	}
	var pkgPath string
	if fn.Pkg != nil && fn.Pkg.Pkg != nil {
		pkgPath = fn.Pkg.Pkg.Path()
	}
	var receiver string
	if fn.Signature != nil && fn.Signature.Recv() != nil {
		qual := types.RelativeTo(fn.Signature.Recv().Pkg())
		if fn.Pkg != nil && fn.Pkg.Pkg != nil {
			qual = types.RelativeTo(fn.Pkg.Pkg)
		}
		receiver = types.TypeString(fn.Signature.Recv().Type(), qual)
	}
	return FunctionKey{
		PackagePath: pkgPath,
		Receiver:    receiver,
		FuncName:    fn.Name(),
	}
}

func positionFor(p *Program, pos token.Pos) Position {
	if p == nil || p.Fset == nil || !pos.IsValid() {
		return Position{}
	}
	place := p.Fset.Position(pos)
	return Position{File: place.Filename, Line: place.Line, Column: place.Column}
}

func sortedFunctions(prog *ssa.Program) []*ssa.Function {
	if prog == nil {
		return nil
	}
	funcs := make([]*ssa.Function, 0)
	for fn := range ssautil.AllFunctions(prog) {
		funcs = append(funcs, fn)
	}
	sort.Slice(funcs, func(i, j int) bool {
		ki := FunctionKeyForSSA(funcs[i])
		kj := FunctionKeyForSSA(funcs[j])
		if ki.PackagePath != kj.PackagePath {
			return ki.PackagePath < kj.PackagePath
		}
		if ki.Receiver != kj.Receiver {
			return ki.Receiver < kj.Receiver
		}
		if ki.FuncName != kj.FuncName {
			return ki.FuncName < kj.FuncName
		}
		return funcs[i].String() < funcs[j].String()
	})
	return funcs
}

func sameSourceFile(posFile, targetFile, dir string) bool {
	if posFile == "" || targetFile == "" {
		return false
	}
	posClean := filepath.Clean(posFile)
	targetClean := filepath.Clean(targetFile)
	if filepath.IsAbs(targetClean) {
		return posClean == targetClean
	}
	if dir != "" {
		if abs, err := filepath.Abs(filepath.Join(dir, targetClean)); err == nil && filepath.Clean(abs) == posClean {
			return true
		}
	}
	posSlash := filepath.ToSlash(posClean)
	targetSlash := filepath.ToSlash(targetClean)
	return strings.HasSuffix(posSlash, "/"+targetSlash) || posSlash == targetSlash
}
