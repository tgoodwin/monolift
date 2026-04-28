package entrypath

import (
	"sync"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type graphBuild struct {
	graph      *callgraph.Graph
	diagnostic []Diagnostic
	algorithm  string
}

type reverseBFSResult struct {
	Touchpoints         []RegionTouchpoint
	TouchpointFunctions []*ssa.Function
	Diagnostics         []Diagnostic
}

var graphCache = struct {
	sync.Mutex
	byProgram map[*ssa.Program]map[string]graphBuild
}{
	byProgram: map[*ssa.Program]map[string]graphBuild{},
}

func buildApplicationCallGraph(prog *ssa.Program, mainPkg *ssa.Package) graphBuild {
	if prog == nil {
		return graphBuild{diagnostic: []Diagnostic{{Kind: "callgraph_program_missing"}}}
	}
	cacheKey := packagePath(mainPkg)
	graphCache.Lock()
	if perProgram := graphCache.byProgram[prog]; perProgram != nil {
		if cached, ok := perProgram[cacheKey]; ok {
			graphCache.Unlock()
			return cached
		}
	}
	graphCache.Unlock()

	roots := applicationRoots(prog, mainPkg)
	built := graphBuild{algorithm: "rta"}
	if len(roots) == 0 {
		built.diagnostic = append(built.diagnostic, Diagnostic{Kind: "callgraph_roots_missing"})
	} else if result := rta.Analyze(roots, true); result != nil {
		built.graph = result.CallGraph
	}
	if built.graph == nil {
		built.diagnostic = append(built.diagnostic, Diagnostic{Kind: "callgraph_unavailable"})
	} else if vtaFallbackNeeded(built.graph) {
		if graph := vta.CallGraph(ssautil.AllFunctions(prog), built.graph); graph != nil {
			built.graph = graph
			built.algorithm = "rta+vta"
			built.diagnostic = append(built.diagnostic, Diagnostic{
				Kind:   "vta_fallback_used",
				Reason: "rta_indirect_collapse",
			})
		}
	}

	graphCache.Lock()
	perProgram := graphCache.byProgram[prog]
	if perProgram == nil {
		perProgram = map[string]graphBuild{}
		graphCache.byProgram[prog] = perProgram
	}
	perProgram[cacheKey] = built
	graphCache.Unlock()
	return built
}

func vtaFallbackNeeded(graph *callgraph.Graph) bool {
	if graph == nil {
		return false
	}
	for _, node := range graph.Nodes {
		if node == nil || node.Func == nil || len(node.Out) > 0 {
			continue
		}
		if signatureAcceptsHandler(node.Func.Signature) {
			return true
		}
	}
	return false
}

func callgraphStats(prog *ssa.Program, graph *callgraph.Graph) Stats {
	stats := Stats{FunctionCount: countProgramFunctions(prog)}
	if graph != nil {
		edgeSites := map[ssa.CallInstruction]bool{}
		for _, node := range graph.Nodes {
			if node == nil {
				continue
			}
			for _, edge := range node.Out {
				if edge == nil {
					continue
				}
				if edge.Site != nil {
					edgeSites[edge.Site] = true
				}
				if isDynamicSite(edge.Site) {
					stats.DynamicEdgeCount++
				} else {
					stats.StaticEdgeCount++
				}
			}
		}
		stats.UnresolvedDynamicSiteCount = countUnresolvedDynamicSites(prog, edgeSites)
	}
	return stats
}

func countUnresolvedDynamicSites(prog *ssa.Program, edgeSites map[ssa.CallInstruction]bool) int {
	count := 0
	for fn := range ssautil.AllFunctions(prog) {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				call, ok := instr.(ssa.CallInstruction)
				if !ok || !isDynamicSite(call) || edgeSites[call] {
					continue
				}
				count++
			}
		}
	}
	return count
}

func isDynamicSite(call ssa.CallInstruction) bool {
	if call == nil || call.Common() == nil {
		return false
	}
	return call.Common().StaticCallee() == nil
}

func reverseBFS(prog *ssa.Program, graph *callgraph.Graph, roots []*ssa.Function) reverseBFSResult {
	if graph == nil {
		return reverseBFSResult{}
	}
	var result reverseBFSResult
	for _, root := range sortedUniqueFunctions(roots) {
		rootNode := graph.Nodes[root]
		if rootNode == nil {
			continue
		}
		callers, bounded := reverseReachableCallers(rootNode)
		if bounded {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Kind:     "reverse_bfs_bound_exceeded",
				Function: root.String(),
				Position: sourcePosition(prog, root.Pos()),
			})
		}
		rootTrace := traceNodeForFunction(prog, root)
		for _, caller := range callers {
			result.Touchpoints = append(result.Touchpoints, RegionTouchpoint{
				RegionRoot: rootTrace,
				Touchpoint: traceNodeForFunction(prog, caller),
				EdgeKind:   EdgeStaticCall,
			})
			result.TouchpointFunctions = append(result.TouchpointFunctions, caller)
		}
	}
	return result
}

func reverseReachableCallers(root *callgraph.Node) ([]*ssa.Function, bool) {
	const maxVisited = 4096
	visited := map[*callgraph.Node]bool{root: true}
	callerFns := map[*ssa.Function]bool{}
	queue := []*callgraph.Node{root}
	bounded := false
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, edge := range node.In {
			if edge == nil || edge.Caller == nil || edge.Caller.Func == nil {
				continue
			}
			if !callerFns[edge.Caller.Func] {
				callerFns[edge.Caller.Func] = true
			}
			if visited[edge.Caller] {
				continue
			}
			if len(visited) >= maxVisited {
				bounded = true
				continue
			}
			visited[edge.Caller] = true
			queue = append(queue, edge.Caller)
		}
	}
	out := make([]*ssa.Function, 0, len(callerFns))
	for fn := range callerFns {
		out = append(out, fn)
	}
	return sortedUniqueFunctions(out), bounded
}

func applicationRoots(prog *ssa.Program, mainPkg *ssa.Package) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	var roots []*ssa.Function
	add := func(fn *ssa.Function) {
		if fn == nil || seen[fn] {
			return
		}
		seen[fn] = true
		roots = append(roots, fn)
	}
	if mainPkg != nil {
		for _, pkg := range importedPackages(prog, mainPkg) {
			add(pkg.Func("init"))
		}
		add(mainPkg.Func("main"))
	}
	return sortedUniqueFunctions(roots)
}

func importedPackages(prog *ssa.Program, root *ssa.Package) []*ssa.Package {
	if prog == nil || root == nil || root.Pkg == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []*ssa.Package
	var visit func(*ssa.Package)
	visit = func(pkg *ssa.Package) {
		if pkg == nil || pkg.Pkg == nil || seen[pkg.Pkg.Path()] {
			return
		}
		seen[pkg.Pkg.Path()] = true
		for _, imported := range pkg.Pkg.Imports() {
			visit(prog.Package(imported))
		}
		out = append(out, pkg)
	}
	visit(root)
	return out
}

func packagePath(pkg *ssa.Package) string {
	if pkg == nil || pkg.Pkg == nil {
		return ""
	}
	return pkg.Pkg.Path()
}
