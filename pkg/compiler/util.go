package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/types"
	"sort"

	"github.com/tgoodwin/monolift/pkg/lift"
	"golang.org/x/tools/go/packages"
)

// stmtToString converts an AST statement back into its Go source code representation.
func (c *Compiler) stmtToString(stmt ast.Stmt) (string, error) {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, c.Fset, stmt)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// exprToString converts an AST expression back into its Go source code representation.
func (c *Compiler) exprToString(expr ast.Expr) (string, error) {
	var buf bytes.Buffer
	err := printer.Fprint(&buf, c.Fset, expr)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// exprToStringDebug converts an AST expression back into its Go source code representation for debugging.
func (c *Compiler) exprToStringDebug(expr ast.Expr) string {
	s, _ := c.exprToString(expr)
	return s
}

// topologicalSort performs a topological sort on the given dependencies.
// It returns an ordered list of dependencies such that each dependency appears
// before any other dependency that relies on it.
func topologicalSort(nodes []*lift.Dependency, graph map[*lift.Dependency][]*lift.Dependency) ([]*lift.Dependency, error) {
	// Kahn's algorithm implementation.
	// `graph` maps a dependency to its *prerequisites*.
	// So, `inDegree[node]` is the count of its prerequisites.
	inDegree := make(map[*lift.Dependency]int)
	for _, node := range nodes {
		inDegree[node] = len(graph[node]) // Number of prerequisites
	}

	queue := []*lift.Dependency{}
	for _, node := range nodes {
		if inDegree[node] == 0 {
			queue = append(queue, node)
		}
	}

	result := []*lift.Dependency{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		result = append(result, n)

		// Find nodes that *depend on* 'n' (i.e., 'n' is a prerequisite for them)
		// and decrement their in-degree.
		for _, m := range nodes {
			if m == n {
				continue
			}
			for _, prereq := range graph[m] {
				if prereq == n {
					inDegree[m]--
					if inDegree[m] == 0 {
						queue = append(queue, m)
					}
					break // Found 'n' as a prerequisite for 'm', move to next 'm'
				}
			}
		}
	}

	if len(result) != len(nodes) {
		if debugDependencyResolution {
			fmt.Println("--- CIRCULAR DEPENDENCY DETECTED ---")
			fmt.Println("Remaining nodes with unmet dependencies:")
			for node, degree := range inDegree {
				if degree > 0 {
					fmt.Printf("  - Node: %s, In-Degree: %d\n", node, degree)
				}
			}
			fmt.Println("------------------------------------")
		}
		return nil, fmt.Errorf("circular dependency detected in dependency graph")
	}

	return result, nil
}

// getExecutableStmtsInOrder finds all statements from init() and main() functions in the package.
// It returns them as a single slice, sorted by source position.
func getExecutableStmtsInOrder(pkg *packages.Package) ([]ast.Stmt, error) {
	if pkg.Name != "main" {
		return nil, fmt.Errorf("package %s is not a main package", pkg.PkgPath)
	}

	var allStmts []ast.Stmt

	for _, fileAST := range pkg.Syntax {
		for _, declNode := range fileAST.Decls {
			if funcDecl, ok := declNode.(*ast.FuncDecl); ok {
				// A function is considered an executable entry point if it's `main` or `init`
				// and it's not a method (has no receiver).
				isMain := funcDecl.Name.Name == "main" && funcDecl.Recv == nil
				isInit := funcDecl.Name.Name == "init" && funcDecl.Recv == nil

				if (isMain || isInit) && funcDecl.Body != nil {
					allStmts = append(allStmts, funcDecl.Body.List...)
				}
			}
		}
	}

	// Sort statements by their position in the source code. This ensures a deterministic
	// processing order, which is a good approximation of execution order (init functions
	// are executed based on file dependencies, then filename order; main is last).
	sort.Slice(allStmts, func(i, j int) bool {
		return allStmts[i].Pos() < allStmts[j].Pos()
	})

	return allStmts, nil
}

// findVarIdentsUsedInStatement returns a slice of *ast.Ident for all variables used in a statement.
func findVarIdentsUsedInStatement(stmt ast.Stmt, pkg *packages.Package) []*ast.Ident {
	var used []*ast.Ident
	ast.Inspect(stmt, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if _, ok := pkg.TypesInfo.Uses[ident].(*types.Var); ok {
				used = append(used, ident)
			}
		}
		return true
	})
	return used
}

// isPackageScope determines if a given statement is a package-level declaration.
func isPackageScope(pkg *packages.Package, targetStmt ast.Stmt) bool {
	// An assignment statement (:=) can only be inside a function.
	if _, ok := targetStmt.(*ast.AssignStmt); ok {
		return false
	}

	// A declaration statement (var, const) can be at package or function level.
	declStmt, ok := targetStmt.(*ast.DeclStmt)
	if !ok {
		// Not a `var` or `const` declaration statement.
		return false
	}

	// Check if it's a top-level declaration in any of the package's files.
	for _, fileAST := range pkg.Syntax {
		for _, topLevelDecl := range fileAST.Decls {
			if topLevelDecl == declStmt.Decl {
				return true
			}
		}
	}
	return false
}
