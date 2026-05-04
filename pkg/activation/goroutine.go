package activation

import (
	"fmt"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// AugmentGoroutine adds edges from reachable functions to goroutine bodies.
func AugmentGoroutine(graph *Graph, program *Program) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	nodes := append([]*Node(nil), graph.Nodes...)
	sort.Slice(nodes, func(i, j int) bool {
		return nodeLess(nodes[i], nodes[j])
	})
	for _, node := range nodes {
		if node == nil || node.Func == nil {
			continue
		}
		for _, block := range node.Func.Blocks {
			for _, instr := range block.Instrs {
				goInstr, ok := instr.(*ssa.Go)
				if !ok || goInstr.Common() == nil {
					continue
				}
				target := goroutineTarget(goInstr.Common())
				if target == nil {
					continue
				}
				if hasGenericContext(target) {
					continue
				}
				to := graph.AddNode(FunctionKeyForSSA(target), target)
				if to == nil {
					continue
				}
				graph.AddEdge(node.ID, to.ID, GoroutineLaunch, positionFor(program, goInstr.Common().Pos()), goInstr.Common().Description())
			}
		}
	}
	return nil
}

func goroutineTarget(common *ssa.CallCommon) *ssa.Function {
	if common == nil {
		return nil
	}
	if fn, ok := common.Value.(*ssa.Function); ok {
		return fn
	}
	if closure, ok := common.Value.(*ssa.MakeClosure); ok {
		if fn, ok := closure.Fn.(*ssa.Function); ok {
			return closureTarget(fn)
		}
	}
	return closureTarget(common.StaticCallee())
}
