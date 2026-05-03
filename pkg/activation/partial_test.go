package activation

import "testing"

func TestFindPartialPathStopsAtFirstGap(t *testing.T) {
	graph := &Graph{Out: map[int][]*Edge{}, In: map[int][]*Edge{}}
	mainNode := graph.addTestNode(FunctionKey{PackagePath: "p", FuncName: "main"})
	aNode := graph.addTestNode(FunctionKey{PackagePath: "p", FuncName: "A"})
	bNode := graph.addTestNode(FunctionKey{PackagePath: "p", FuncName: "B"})
	graph.addTestNode(FunctionKey{PackagePath: "p", FuncName: "C"})
	graph.AddEdge(mainNode.ID, aNode.ID, DirectCall, Position{File: "main.go", Line: 1}, "")
	graph.AddEdge(aNode.ID, bNode.ID, DirectCall, Position{File: "main.go", Line: 2}, "")

	partial := FindPartialPath(graph, []ExpectedStep{
		{Step: 0, Key: FunctionKey{PackagePath: "p", FuncName: "main"}, RawEdge: "entrypoint", EdgeKind: DirectCall},
		{Step: 1, Key: FunctionKey{PackagePath: "p", FuncName: "A"}, RawEdge: "direct-function-call", EdgeKind: DirectCall},
		{Step: 2, Key: FunctionKey{PackagePath: "p", FuncName: "B"}, RawEdge: "direct-function-call", EdgeKind: DirectCall},
		{Step: 3, Key: FunctionKey{PackagePath: "p", FuncName: "C"}, RawEdge: "direct-function-call", EdgeKind: DirectCall},
	})
	if partial == nil || partial.Prefix == nil {
		t.Fatalf("partial path missing: %+v", partial)
	}
	if got := len(partial.Prefix.Steps); got != 3 {
		t.Fatalf("prefix steps = %d, want 3", got)
	}
	if got := partial.Prefix.Steps[2].Node.Key.FuncName; got != "B" {
		t.Fatalf("last prefix node = %s, want B", got)
	}
	if partial.Gap.AfterStep != 2 || partial.Gap.ExpectedEdge != "direct-function-call" {
		t.Fatalf("gap = %+v", partial.Gap)
	}
}

func (g *Graph) addTestNode(key FunctionKey) *Node {
	node := &Node{
		ID:      len(g.Nodes),
		Key:     key,
		Name:    key.String(),
		Package: key.PackagePath,
	}
	g.Nodes = append(g.Nodes, node)
	return node
}
