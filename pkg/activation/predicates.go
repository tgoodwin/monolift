package activation

import (
	"fmt"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// FrameworkPredicate describes a framework-owned struct field dispatch rule.
type FrameworkPredicate struct {
	ImportPath string // e.g., "github.com/spf13/cobra"
	TypeName   string // e.g., "Command"
	FieldName  string // e.g., "RunE"
	DispatchFn string // e.g., "(*Command).execute"
}

// DefaultFrameworkPredicates returns the registered framework predicates.
func DefaultFrameworkPredicates() []FrameworkPredicate {
	return []FrameworkPredicate{
		{
			ImportPath: "github.com/spf13/cobra",
			TypeName:   "Command",
			FieldName:  "RunE",
			DispatchFn: "(*Command).execute",
		},
		{
			ImportPath: "github.com/spf13/cobra",
			TypeName:   "Command",
			FieldName:  "Run",
			DispatchFn: "(*Command).execute",
		},
		{
			ImportPath: "github.com/urfave/cli/v3",
			TypeName:   "Command",
			FieldName:  "Action",
			DispatchFn: "(*Command).Run",
		},
		{
			ImportPath: "github.com/urfave/cli/v3",
			TypeName:   "App",
			FieldName:  "Action",
			DispatchFn: "(*App).RunContext",
		},
	}
}

// ApplyPredicates adds known framework dispatch edges for function-valued
// struct fields discovered by the generic struct-field pass.
func ApplyPredicates(graph *Graph, program *Program, index *StructFieldIndex, predicates []FrameworkPredicate) error {
	if graph == nil {
		return fmt.Errorf("graph is nil")
	}
	if program == nil {
		return fmt.Errorf("program is nil")
	}
	if index == nil || len(predicates) == 0 {
		return nil
	}
	for _, predicate := range predicates {
		predicateType := findPredicateType(program, predicate)
		if predicateType == nil {
			continue
		}
		dispatch := findDispatchNode(graph, program, predicate)
		if dispatch == nil {
			index.Diagnostics = append(index.Diagnostics, fmt.Sprintf("predicate dispatch not found: %s", predicate.Description()))
			continue
		}
		for _, key := range index.sortedKeys() {
			if key.FieldName != predicate.FieldName {
				continue
			}
			for _, stored := range index.Stores[key] {
				if hasGenericContext(stored.Func) {
					continue
				}
				if !types.Identical(stored.StructType, predicateType) {
					continue
				}
				to := graph.AddNode(FunctionKeyForSSA(stored.Func), stored.Func)
				if to == nil {
					continue
				}
				graph.AddEdge(dispatch.ID, to.ID, StructFieldFuncValue, stored.Position, predicate.Description())
			}
		}
	}
	return nil
}

func (p FrameworkPredicate) Description() string {
	return fmt.Sprintf("framework predicate %s.%s -> %s", p.ImportPath, p.FieldName, p.DispatchFn)
}

func findPredicateType(program *Program, predicate FrameworkPredicate) *types.Named {
	if program != nil && program.SSAProgram != nil {
		if named := namedTypeFromSSAPackage(program.SSAProgram.ImportedPackage(predicate.ImportPath), predicate.TypeName); named != nil {
			return named
		}
		for _, pkg := range program.SSAProgram.AllPackages() {
			if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Path() != predicate.ImportPath {
				continue
			}
			if named := namedTypeFromSSAPackage(pkg, predicate.TypeName); named != nil {
				return named
			}
		}
	}
	for _, pkg := range program.SSAPackages {
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Path() != predicate.ImportPath {
			continue
		}
		if named := namedTypeFromSSAPackage(pkg, predicate.TypeName); named != nil {
			return named
		}
	}
	return nil
}

func namedTypeFromSSAPackage(pkg *ssa.Package, typeName string) *types.Named {
	if pkg == nil {
		return nil
	}
	member := pkg.Type(typeName)
	if member == nil {
		return nil
	}
	named, _ := member.Object().Type().(*types.Named)
	return named
}

func findDispatchNode(graph *Graph, program *Program, predicate FrameworkPredicate) *Node {
	if node := findDispatchInGraph(graph, predicate); node != nil {
		return node
	}
	if fn := findDispatchFunction(program, predicate); fn != nil {
		return graph.AddNode(FunctionKeyForSSA(fn), fn)
	}
	return nil
}

func findDispatchInGraph(graph *Graph, predicate FrameworkPredicate) *Node {
	receiver, funcName := parseDispatchFn(predicate.DispatchFn)
	for _, node := range graph.Nodes {
		if node.Key.PackagePath != predicate.ImportPath || node.Key.FuncName != funcName {
			continue
		}
		if receiver == "" || sameReceiverName(node.Key.Receiver, receiver) {
			return node
		}
	}
	return nil
}

func findDispatchFunction(program *Program, predicate FrameworkPredicate) *ssa.Function {
	receiver, funcName := parseDispatchFn(predicate.DispatchFn)
	for _, fn := range sortedFunctions(program.SSAProgram) {
		key := FunctionKeyForSSA(fn)
		if key.PackagePath != predicate.ImportPath || key.FuncName != funcName {
			continue
		}
		if receiver == "" || sameReceiverName(key.Receiver, receiver) {
			return fn
		}
	}
	return nil
}

func parseDispatchFn(raw string) (receiver, funcName string) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "(") {
		return "", raw
	}
	idx := strings.Index(raw, ").")
	if idx < 0 {
		return "", raw
	}
	receiver = strings.TrimPrefix(raw[1:idx], "(")
	receiver = strings.TrimSuffix(receiver, ")")
	funcName = raw[idx+2:]
	return receiver, funcName
}

func sameReceiverName(a, b string) bool {
	return strings.TrimPrefix(a, "*") == strings.TrimPrefix(b, "*")
}
