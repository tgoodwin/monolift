package activation

import (
	"fmt"

	gocallgraph "golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
)

// BuildRTAGraph runs Rapid Type Analysis from the supplied entrypoints and
// converts the resulting call graph into activation graph nodes and edges.
func BuildRTAGraph(program *Program, entrypoints []*ssa.Function) (*Graph, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	if len(entrypoints) == 0 {
		return nil, fmt.Errorf("no entrypoints")
	}
	result := rta.Analyze(entrypoints, true)
	return convertCallGraph(program, result.CallGraph), nil
}

func convertCallGraph(program *Program, cg *gocallgraph.Graph) *Graph {
	if cg == nil {
		return &Graph{Out: map[int][]*Edge{}, In: map[int][]*Edge{}}
	}
	funcs := callGraphFunctions(cg)

	graph := &Graph{
		Nodes: make([]*Node, 0, len(funcs)),
		Out:   map[int][]*Edge{},
		In:    map[int][]*Edge{},
	}
	nodeByFunc := make(map[*ssa.Function]*Node, len(funcs))
	for id, fn := range funcs {
		node := nodeForFunction(id, program, fn)
		graph.Nodes = append(graph.Nodes, node)
		nodeByFunc[fn] = node
	}

	for _, cgEdge := range callGraphEdges(cg, funcs, program) {
		from := nodeByFunc[cgEdge.Caller.Func]
		to := nodeByFunc[cgEdge.Callee.Func]
		if from == nil || to == nil {
			continue
		}
		edge := &Edge{
			ID:          len(graph.Edges),
			From:        from.ID,
			To:          to.ID,
			Kind:        classifyRTAEdge(cgEdge.Site),
			Position:    positionFor(program, cgEdge.Pos()),
			Description: cgEdge.Description(),
		}
		graph.Edges = append(graph.Edges, edge)
		graph.Out[edge.From] = append(graph.Out[edge.From], edge)
		graph.In[edge.To] = append(graph.In[edge.To], edge)
	}
	return graph
}

func callGraphEdgeLess(a, b *gocallgraph.Edge, program *Program) bool {
	ak := FunctionKeyForSSA(a.Caller.Func).String()
	bk := FunctionKeyForSSA(b.Caller.Func).String()
	if ak != bk {
		return ak < bk
	}
	at := FunctionKeyForSSA(a.Callee.Func).String()
	bt := FunctionKeyForSSA(b.Callee.Func).String()
	if at != bt {
		return at < bt
	}
	ap := positionFor(program, a.Pos())
	bp := positionFor(program, b.Pos())
	if ap.File != bp.File {
		return ap.File < bp.File
	}
	if ap.Line != bp.Line {
		return ap.Line < bp.Line
	}
	if ap.Column != bp.Column {
		return ap.Column < bp.Column
	}
	return a.Description() < b.Description()
}

func classifyRTAEdge(site ssa.CallInstruction) EdgeKind {
	if site == nil || site.Common() == nil {
		return Unsupported
	}
	common := site.Common()
	if common.IsInvoke() {
		return InterfaceDispatch
	}
	if callee := common.StaticCallee(); callee != nil {
		if callee.Signature != nil && callee.Signature.Recv() != nil {
			return ConcreteMethodCall
		}
		return DirectCall
	}
	return Unsupported
}
