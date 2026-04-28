package extract

import (
	"path/filepath"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

type LoadedModule = loadedModule

func LoadModule(req Request) (*LoadedModule, error) {
	return loadModule(req)
}

func BuildProgram(loaded *LoadedModule) (*ssa.Program, error) {
	built, err := buildProgram(loaded)
	if err != nil {
		return nil, err
	}
	return built.Program, nil
}

func CallGraphForProgram(program *ssa.Program) *callgraph.Graph {
	return callGraphForProgram(program)
}

func ReachableFunctions(loaded *LoadedModule, root reportv2.Root) (*ssa.Program, []*ssa.Function, error) {
	built, err := buildProgram(loaded)
	if err != nil {
		return nil, nil, err
	}
	closure := buildRegionClosure(loaded, built, root)
	return built.Program, closure.ReachableFuncs, nil
}

func ResolveRoot(loaded *LoadedModule) reportv2.Root {
	return resolveRoot(loaded)
}

func RebindLoadedModule(loaded *LoadedModule, req Request) (*LoadedModule, error) {
	if loaded == nil {
		return nil, nil
	}
	rootRegion, rootPragma, err := selectRootRegion(req)
	if err != nil {
		return nil, err
	}

	rootFile, err := filepath.Abs(rootPragma.Span.Filename)
	if err != nil {
		return nil, err
	}

	rootPkg := findPackageForFile(loaded.Packages, rootFile)
	if rootPkg == nil {
		return nil, err
	}

	clone := *loaded
	clone.RootPragma = rootPragma
	clone.RootRegion = rootRegion
	clone.RootFile = rootFile
	clone.RootPkg = rootPkg
	return &clone, nil
}
