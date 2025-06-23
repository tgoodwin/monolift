package lift

import (
	"go/ast"
)

// InstantiationKind defines how a dependency is created.
type InstantiationKind int

const (
	// VariableDeclaration indicates this dependency is instantiated via a `:=` or `var =` statement.
	VariableDeclaration InstantiationKind = iota
	// InlinedExpression indicates this dependency is an expression that should be inlined directly
	// as an argument to another function/struct, and does not require its own variable declaration.
	InlinedExpression
)

// Dependency represents a single variable and how it's instantiated.
// This is a node in our dependency graph.
type Dependency struct {
	VarName    string            // The name of the variable (e.g., "dbStore"). Only relevant for VariableDeclaration kind.
	Kind       InstantiationKind // How this dependency is created (VariableDeclaration or InlinedExpression).
	ProviderID string            // A unique ID for the provider of this dependency (e.g., AST position).

	// For VariableDeclaration kind, this is the full string of the assignment statement (e.g., "dbStore, err := database.NewRedisStore(...)").
	// For InlinedExpression kind, this is the string of the expression itself (e.g., "context.Background()", `"my-string"`).
	RenderedForm string

	// For VariableDeclaration kind, this stores the AST node of the original assignment.
	// This is used internally during resolution to extract arguments.
	OriginalAssignStmt ast.Stmt
}

// InstantiationPlan is the final, ordered list of dependencies to be generated in code.
// The root service is the last element in the slice.
type InstantiationPlan struct {
	Steps          []*Dependency
	RootDependency *Dependency
}
