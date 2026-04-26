package extract

import (
	"fmt"
	"sync"
	"sync/atomic"

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
	rtaOnce     sync.Once
	RTAGraph    *callgraph.Graph
}

type callgraphBuildCounters struct {
	cha int64
	rta int64
}

var (
	programCallgraphsMu sync.Mutex
	programCallgraphs   = map[*ssa.Program]*callgraph.Graph{}
	chaBuildCount       atomic.Int64
	rtaBuildCount       atomic.Int64
)

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
		CHAGraph:    callGraphForProgram(program),
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
	built.rtaOnce.Do(func() {
		rtaBuildCount.Add(1)
		result := rta.Analyze(roots, true)
		if result != nil && result.CallGraph != nil {
			built.RTAGraph = result.CallGraph
			return
		}
		built.RTAGraph = built.CHAGraph
	})
	if built.RTAGraph != nil {
		return built.RTAGraph
	}
	return built.CHAGraph
}

func callGraphForProgram(program *ssa.Program) *callgraph.Graph {
	if program == nil {
		return nil
	}
	programCallgraphsMu.Lock()
	defer programCallgraphsMu.Unlock()
	if graph := programCallgraphs[program]; graph != nil {
		return graph
	}
	chaBuildCount.Add(1)
	graph := cha.CallGraph(program)
	programCallgraphs[program] = graph
	return graph
}

func resetCallgraphBuildCounters() {
	chaBuildCount.Store(0)
	rtaBuildCount.Store(0)
}

func snapshotCallgraphBuildCounters() callgraphBuildCounters {
	return callgraphBuildCounters{
		cha: chaBuildCount.Load(),
		rta: rtaBuildCount.Load(),
	}
}
