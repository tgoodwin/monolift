package activation

import (
	"sort"

	"golang.org/x/tools/go/ssa"
)

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

// FunctionSet snapshots the SSA functions currently present in the graph.
func (g *Graph) FunctionSet() map[*ssa.Function]bool {
	out := map[*ssa.Function]bool{}
	if g == nil {
		return out
	}
	for _, node := range g.Nodes {
		if node != nil && node.Func != nil {
			out[node.Func] = true
		}
	}
	return out
}

// NewFunctionsSince returns functions added after the supplied snapshot.
func (g *Graph) NewFunctionsSince(before map[*ssa.Function]bool) []*ssa.Function {
	if g == nil {
		return nil
	}
	seen := map[*ssa.Function]bool{}
	var out []*ssa.Function
	for _, node := range g.Nodes {
		if node == nil || node.Func == nil || before[node.Func] || seen[node.Func] {
			continue
		}
		seen[node.Func] = true
		out = append(out, node.Func)
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
