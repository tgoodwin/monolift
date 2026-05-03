package activation

import "golang.org/x/tools/go/ssa"

// AddEdge inserts an edge into the graph unless an edge with the same
// (from, to, kind) tuple already exists.
func (g *Graph) AddEdge(from, to int, kind EdgeKind, pos Position, desc string) *Edge {
	if g == nil || from < 0 || to < 0 || from >= len(g.Nodes) || to >= len(g.Nodes) {
		return nil
	}
	for _, edge := range g.Out[from] {
		if edge.From == from && edge.To == to && edge.Kind == kind {
			return edge
		}
	}
	if g.Out == nil {
		g.Out = map[int][]*Edge{}
	}
	if g.In == nil {
		g.In = map[int][]*Edge{}
	}
	edge := &Edge{
		ID:          len(g.Edges),
		From:        from,
		To:          to,
		Kind:        kind,
		Position:    pos,
		Description: desc,
	}
	g.Edges = append(g.Edges, edge)
	g.Out[from] = append(g.Out[from], edge)
	g.In[to] = append(g.In[to], edge)
	return edge
}

// AddNode inserts an SSA function node unless the function is already present.
func (g *Graph) AddNode(key FunctionKey, fn *ssa.Function) *Node {
	if g == nil || fn == nil {
		return nil
	}
	for _, node := range g.Nodes {
		if node.Func == fn {
			return node
		}
	}
	if key.IsZero() {
		key = FunctionKeyForSSA(fn)
	}
	node := &Node{
		ID:       len(g.Nodes),
		Key:      key,
		Name:     fn.String(),
		Package:  key.PackagePath,
		Position: positionForSSA(fn),
		Func:     fn,
	}
	g.Nodes = append(g.Nodes, node)
	if g.Out == nil {
		g.Out = map[int][]*Edge{}
	}
	if g.In == nil {
		g.In = map[int][]*Edge{}
	}
	return node
}

func positionForSSA(fn *ssa.Function) Position {
	if fn == nil || fn.Prog == nil || fn.Prog.Fset == nil || !fn.Pos().IsValid() {
		return Position{}
	}
	place := fn.Prog.Fset.Position(fn.Pos())
	return Position{File: place.Filename, Line: place.Line, Column: place.Column}
}
