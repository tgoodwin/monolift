package lift

import (
	"fmt"
	"go/ast"
	"strings"
)

// InstantiationKind defines how a dependency is created.
type InstantiationKind int

const (
	// VariableDeclaration indicates this dependency is instantiated via a `:=` or `var =` statement.
	VariableDeclaration InstantiationKind = iota
	// RelevantStatement indicates this is another statement (e.g., if, function call)
	// that is a direct consequence of a VariableDeclaration.
	RelevantStatement
)

// Dependency represents a single variable and how it's instantiated.
// This is a node in our dependency graph.
type Dependency struct {
	VarName    string            // The name of the variable (e.g., "dbStore"). Only relevant for VariableDeclaration kind.
	Kind       InstantiationKind // How this dependency is created (VariableDeclaration or InlinedExpression).
	ProviderID string            // A unique ID for the provider of this dependency (e.g., AST position).

	// The full string of the statement to be copied into the generated code.
	RenderedForm string

	// The original AST node for the statement.
	// This is used internally during resolution to extract arguments.
	OriginalStmt ast.Stmt
}

// String provides a simple string representation for debugging.
func (d *Dependency) String() string {
	if d.VarName != "" {
		return fmt.Sprintf("Dep<%s>", d.VarName)
	}
	// For relevant statements that don't declare a var, use the first line of the statement.
	return fmt.Sprintf("Dep<%q>", strings.Split(strings.TrimSpace(d.RenderedForm), "\n")[0])
}

// InstantiationPlan is the final, ordered list of dependencies to be generated in code.
// The root service is the last element in the slice.
type InstantiationPlan struct {
	Steps          []*Dependency
	RootDependency *Dependency
}
