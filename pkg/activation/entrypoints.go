package activation

import (
	"fmt"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// FindEntrypoints discovers main.main functions in loaded command packages.
func (c Config) FindEntrypoints(program *Program) ([]*ssa.Function, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	var entrypoints []*ssa.Function
	for _, pkg := range program.SSAPackages {
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Name() != "main" {
			continue
		}
		if mainFn := pkg.Func("main"); mainFn != nil {
			entrypoints = append(entrypoints, mainFn)
		}
	}
	sort.Slice(entrypoints, func(i, j int) bool {
		return FunctionKeyForSSA(entrypoints[i]).String() < FunctionKeyForSSA(entrypoints[j]).String()
	})
	if len(entrypoints) == 0 {
		return nil, fmt.Errorf("no main.main entrypoints found in loaded command packages")
	}
	return entrypoints, nil
}
