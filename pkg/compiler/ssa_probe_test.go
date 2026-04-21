package compiler

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/cha"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestCaddySSAProbe(t *testing.T) {
	if os.Getenv("MONOLIFT_SSA_PROBE") != "1" {
		t.Skip("MONOLIFT_SSA_PROBE=1 required")
	}

	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax | packages.NeedModule,
		Dir:  filepath.Join("..", "..", "evaluation", "caddy"),
		Env:  probeEnv(),
	}

	pkgs, err := packages.Load(cfg, "./modules/caddyhttp/reverseproxy")
	if err != nil {
		t.Fatalf("packages.Load: %v", err)
	}
	if count := packages.PrintErrors(pkgs); count != 0 {
		t.Fatalf("packages.Load returned %d package errors", count)
	}

	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	graph := cha.CallGraph(prog)

	fnCount := len(ssautil.AllFunctions(prog))
	nodeCount := len(graph.Nodes)
	root := findFunction(ssautil.AllFunctions(prog), "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy", "(*Handler).ServeHTTP")
	if root == nil {
		t.Fatalf("reverseproxy root function not found in SSA program; candidates=%v", functionNames(ssautil.AllFunctions(prog), "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy", "ServeHTTP"))
	}

	fanout := interfaceInvokeFanout(graph, root)
	keys := make([]string, 0, len(fanout))
	for key := range fanout {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	t.Logf("caddy probe: ssaFunctions=%d chaNodes=%d root=%s", fnCount, nodeCount, root.String())
	for _, key := range keys {
		t.Logf("caddy probe: invoke=%s fanout=%d", key, fanout[key])
	}
}

func probeEnv() []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	if toolchain := os.Getenv("MONOLIFT_SSA_PROBE_TOOLCHAIN"); toolchain != "" {
		env = append(env, "GOTOOLCHAIN="+toolchain)
	}
	return env
}

func findFunction(functions map[*ssa.Function]bool, pkgPath, suffix string) *ssa.Function {
	candidates := make([]*ssa.Function, 0, 1)
	for fn := range functions {
		if fn == nil || fn.Pkg == nil || fn.Pkg.Pkg == nil {
			continue
		}
		if fn.Pkg.Pkg.Path() != pkgPath {
			continue
		}
		if strings.HasSuffix(fn.String(), suffix) || strings.Contains(fn.String(), strings.TrimPrefix(suffix, "(*")) {
			candidates = append(candidates, fn)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b *ssa.Function) int {
		return strings.Compare(a.String(), b.String())
	})
	return candidates[0]
}

func functionNames(functions map[*ssa.Function]bool, pkgPath, name string) []string {
	out := []string{}
	for fn := range functions {
		if fn == nil || fn.Pkg == nil || fn.Pkg.Pkg == nil {
			continue
		}
		if fn.Pkg.Pkg.Path() != pkgPath {
			continue
		}
		if strings.Contains(fn.String(), name) {
			out = append(out, fn.String())
		}
	}
	sort.Strings(out)
	return out
}

func interfaceInvokeFanout(graph *callgraph.Graph, fn *ssa.Function) map[string]int {
	out := map[string]int{}
	node := graph.Nodes[fn]
	if node == nil {
		return out
	}

	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok || !call.Common().IsInvoke() {
				continue
			}
			label := fmt.Sprintf("%s @ %s", call.Common().Method.Name(), fn.Prog.Fset.Position(instr.Pos()))
			for _, edge := range node.Out {
				if edge.Site == call {
					out[label]++
				}
			}
		}
	}

	return out
}
