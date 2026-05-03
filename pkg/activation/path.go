package activation

import (
	"sort"

	"golang.org/x/tools/go/ssa"
)

// ShortestPath finds a deterministic BFS path from any entrypoint to target.
func ShortestPath(graph *Graph, entrypoints []*ssa.Function, target *ssa.Function) (*Path, bool) {
	if graph == nil || target == nil {
		return nil, false
	}
	targetNode := graph.nodeByFunction(target)
	if targetNode == nil {
		return nil, false
	}
	starts := make([]*Node, 0, len(entrypoints))
	for _, entry := range entrypoints {
		if node := graph.nodeByFunction(entry); node != nil {
			starts = append(starts, node)
		}
	}
	sort.Slice(starts, func(i, j int) bool {
		return nodeLess(starts[i], starts[j])
	})
	seen := map[int]bool{}
	prev := map[int]*Edge{}
	queue := make([]*Node, 0, len(starts))
	for _, start := range starts {
		if seen[start.ID] {
			continue
		}
		seen[start.ID] = true
		queue = append(queue, start)
	}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node.ID == targetNode.ID {
			return buildPath(graph, targetNode, prev), true
		}
		edges := append([]*Edge(nil), graph.Out[node.ID]...)
		sort.SliceStable(edges, func(i, j int) bool {
			return edgeLess(graph, edges[i], edges[j])
		})
		for _, edge := range edges {
			if seen[edge.To] {
				continue
			}
			seen[edge.To] = true
			prev[edge.To] = edge
			queue = append(queue, graph.Nodes[edge.To])
		}
	}
	return nil, false
}

func buildPath(graph *Graph, target *Node, prev map[int]*Edge) *Path {
	var reversed []PathStep
	for node := target; node != nil; {
		edge := prev[node.ID]
		reversed = append(reversed, PathStep{Node: node, Edge: edge})
		if edge == nil {
			break
		}
		node = graph.Nodes[edge.From]
	}
	steps := make([]PathStep, 0, len(reversed))
	for i := len(reversed) - 1; i >= 0; i-- {
		steps = append(steps, reversed[i])
	}
	return &Path{Steps: steps}
}

func (g *Graph) nodeByFunction(fn *ssa.Function) *Node {
	if g == nil || fn == nil {
		return nil
	}
	for _, node := range g.Nodes {
		if node.Func == fn {
			return node
		}
	}
	return nil
}

func nodeLess(a, b *Node) bool {
	if a == nil || b == nil {
		return b != nil
	}
	if len(a.Key.PackagePath) != len(b.Key.PackagePath) {
		return len(a.Key.PackagePath) < len(b.Key.PackagePath)
	}
	if a.Key.PackagePath != b.Key.PackagePath {
		return a.Key.PackagePath < b.Key.PackagePath
	}
	if a.Key.FuncName != b.Key.FuncName {
		return a.Key.FuncName < b.Key.FuncName
	}
	if a.Key.Receiver != b.Key.Receiver {
		return a.Key.Receiver < b.Key.Receiver
	}
	return a.Name < b.Name
}

func edgeLess(graph *Graph, a, b *Edge) bool {
	if a == nil || b == nil {
		return b != nil
	}
	toA := graph.Nodes[a.To]
	toB := graph.Nodes[b.To]
	if nodeLess(toA, toB) {
		return true
	}
	if nodeLess(toB, toA) {
		return false
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if a.Position.File != b.Position.File {
		return a.Position.File < b.Position.File
	}
	if a.Position.Line != b.Position.Line {
		return a.Position.Line < b.Position.Line
	}
	if a.Position.Column != b.Position.Column {
		return a.Position.Column < b.Position.Column
	}
	return a.ID < b.ID
}
