package activation

import (
	"fmt"
	"sort"
	"strings"

	gocallgraph "golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

// ExploreCallees runs RTA from newly discovered roots and merges their call
// trees into graph.
func ExploreCallees(graph *Graph, program *Program, roots []*ssa.Function) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	roots = sortedUniqueFunctions(roots)
	var skipped []*ssa.Function
	roots, skipped = rtaCompatibleRoots(roots)
	recordSkippedRTARoots(graph, skipped)
	if len(roots) == 0 {
		return nil
	}
	program.BuildSSA()
	result := rta.Analyze(roots, true)
	mergeCallGraph(graph, program, result.CallGraph)
	return nil
}

func sortedUniqueFunctions(funcs []*ssa.Function) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	out := make([]*ssa.Function, 0, len(funcs))
	for _, fn := range funcs {
		if fn == nil || seen[fn] {
			continue
		}
		seen[fn] = true
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool {
		ki := FunctionKeyForSSA(out[i]).String()
		kj := FunctionKeyForSSA(out[j]).String()
		if ki != kj {
			return ki < kj
		}
		return out[i].String() < out[j].String()
	})
	return out
}

func mergeCallGraph(graph *Graph, program *Program, cg *gocallgraph.Graph) {
	if graph == nil || cg == nil {
		return
	}
	funcs := callGraphFunctions(cg)
	nodeByFunc := make(map[*ssa.Function]*Node, len(funcs))
	for _, fn := range funcs {
		nodeByFunc[fn] = graph.AddNode(FunctionKeyForSSA(fn), fn)
	}

	edges := callGraphEdges(cg, funcs, program)
	for _, cgEdge := range edges {
		from := nodeByFunc[cgEdge.Caller.Func]
		if from == nil {
			from = graph.AddNode(FunctionKeyForSSA(cgEdge.Caller.Func), cgEdge.Caller.Func)
		}
		to := nodeByFunc[cgEdge.Callee.Func]
		if to == nil {
			to = graph.AddNode(FunctionKeyForSSA(cgEdge.Callee.Func), cgEdge.Callee.Func)
		}
		if from == nil || to == nil {
			continue
		}
		graph.AddEdge(from.ID, to.ID, classifyRTAEdge(cgEdge.Site), positionFor(program, cgEdge.Pos()), cgEdge.Description())
	}
}

func rtaCompatibleRoots(roots []*ssa.Function) ([]*ssa.Function, []*ssa.Function) {
	compatible := make([]*ssa.Function, 0, len(roots))
	var skipped []*ssa.Function
	for _, root := range roots {
		if hasGenericContext(root) {
			skipped = append(skipped, root)
			continue
		}
		compatible = append(compatible, root)
	}
	return compatible, skipped
}

func hasGenericContext(fn *ssa.Function) bool {
	for current := fn; current != nil; current = current.Parent() {
		if params := current.TypeParams(); params != nil && params.Len() > 0 {
			return true
		}
	}
	return false
}

func recordSkippedRTARoots(graph *Graph, skipped []*ssa.Function) {
	if graph == nil || len(skipped) == 0 {
		return
	}
	names := make([]string, 0, len(skipped))
	for i, fn := range skipped {
		if i >= 5 {
			break
		}
		names = append(names, FunctionKeyForSSA(fn).String())
	}
	message := fmt.Sprintf("skipped %d augmentation root(s) in generic function contexts unsupported by RTA", len(skipped))
	if len(names) > 0 {
		message += ": " + strings.Join(names, ", ")
		if len(skipped) > len(names) {
			message += ", ..."
		}
	}
	graph.AugmentDiagnostics = append(graph.AugmentDiagnostics, Diagnostic{
		Severity: "warning",
		Phase:    "augment",
		Message:  message,
	})
}

func callGraphFunctions(cg *gocallgraph.Graph) []*ssa.Function {
	if cg == nil {
		return nil
	}
	funcs := make([]*ssa.Function, 0, len(cg.Nodes))
	for fn := range cg.Nodes {
		if fn != nil {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool {
		ki := FunctionKeyForSSA(funcs[i]).String()
		kj := FunctionKeyForSSA(funcs[j]).String()
		if ki != kj {
			return ki < kj
		}
		return funcs[i].String() < funcs[j].String()
	})
	return funcs
}

func callGraphEdges(cg *gocallgraph.Graph, funcs []*ssa.Function, program *Program) []*gocallgraph.Edge {
	if cg == nil {
		return nil
	}
	var edges []*gocallgraph.Edge
	for _, fn := range funcs {
		cgNode := cg.Nodes[fn]
		if cgNode == nil {
			continue
		}
		edges = append(edges, cgNode.Out...)
	}
	sort.Slice(edges, func(i, j int) bool {
		return callGraphEdgeLess(edges[i], edges[j], program)
	})
	return edges
}
