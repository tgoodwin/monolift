package extract

import (
	"fmt"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

const builderMode = ssa.InstantiateGenerics

type builtProgram struct {
	Program     *ssa.Program
	AllPackages []*ssa.Package
	RootPackage *ssa.Package
	Functions   map[*ssa.Function]bool
	CHAGraph    *callgraph.Graph
}

func buildProgram(loaded *loadedModule) (*builtProgram, error) {
	program, ssaPkgs := ssautil.AllPackages(loaded.Packages, builderMode)
	program.Build()

	rootPackage := findSSAPackage(ssaPkgs, loaded.RootPkg.PkgPath)
	if rootPackage == nil {
		return nil, fmt.Errorf("SSA package for %s was not built", loaded.RootPkg.PkgPath)
	}

	return &builtProgram{
		Program:     program,
		AllPackages: ssaPkgs,
		RootPackage: rootPackage,
		Functions:   ssautil.AllFunctions(program),
		CHAGraph:    cha.CallGraph(program),
	}, nil
}

func findSSAPackage(pkgs []*ssa.Package, pkgPath string) *ssa.Package {
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Pkg == nil {
			continue
		}
		if pkg.Pkg.Path() == pkgPath {
			return pkg
		}
	}
	return nil
}

func dispatchGraph(built *builtProgram, roots []*ssa.Function, registryKey *string) *callgraph.Graph {
	if registryKey == nil || len(roots) == 0 {
		return built.CHAGraph
	}
	result := rta.Analyze(roots, true)
	if result != nil && result.CallGraph != nil {
		return result.CallGraph
	}
	return built.CHAGraph
}
