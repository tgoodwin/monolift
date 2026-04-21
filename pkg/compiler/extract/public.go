package extract

import (
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
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

func ReachableFunctions(loaded *LoadedModule, root reportv2.Root) (*ssa.Program, []*ssa.Function, error) {
	built, err := buildProgram(loaded)
	if err != nil {
		return nil, nil, err
	}
	closure := buildClosure(loaded, built, root)
	return built.Program, closure.ReachableFuncs, nil
}
