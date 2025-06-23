package lift

import (
	"go/types"
)

// InstantiationKind defines how a dependency is created.
type InstantiationKind int

const (
	// A dependency created by calling a constructor function.
	ConstructorCall InstantiationKind = iota
	// A dependency created by a struct literal.
	StructLiteral
	// A dependency sourced from an environment variable.
	EnvVar
	// A dependency that is a basic literal (string, int, bool).
	Literal
	// A special case for a known global, like context.Background().
	KnownGlobal
)

// Dependency represents a single variable and how it's instantiated.
// This is a node in our dependency graph.
type Dependency struct {
	VarName    string            // The name of the variable (e.g., "dbStore").
	VarType    types.Type        // The Go type of the variable.
	Kind       InstantiationKind // How this dependency is created.
	ProviderID string            // A unique ID for the provider of this dependency (e.g., the function call's source position, or the env var name).
	IsPointer  bool              // True if the variable is a pointer type (e.g., *MyStruct)

	// Details for each kind of instantiation. Only one will be non-nil based on Kind.
	CtorCallData    *ConstructorCallData
	StructLitData   *StructLiteralData
	EnvVarData      *EnvVarData
	LiteralData     *LiteralData
	KnownGlobalData *KnownGlobalData
}

// ConstructorCallData holds info for dependencies created via function calls.
type ConstructorCallData struct {
	PkgPath  string   // Import path of the constructor's package.
	PkgName  string   // Name of the constructor's package.
	FuncName string   // Name of the constructor function (e.g., "NewRedisStore").
	Args     []string // The arguments passed to the constructor, as source code strings.
}

// StructLiteralData holds info for dependencies created via struct literals.
type StructLiteralData struct {
	PkgPath  string            // Import path of the struct's package.
	PkgName  string            // Name of the struct's package.
	TypeName string            // Name of the struct type.
	Fields   map[string]string // Field initializers. Key is field name, value is source code string.
}

// EnvVarData holds info for dependencies from environment variables.
type EnvVarData struct {
	Name         string // The environment variable name (e.g., "REDIS_ADDRESS").
	DefaultValue string // The fallback value, if any.
}

// LiteralData holds info for literal value dependencies.
type LiteralData struct {
	Value string // The string representation of the literal (e.g., "0", "\"some_string\"").
}

// KnownGlobalData holds info for well-known global variables.
type KnownGlobalData struct {
	PkgPath string // e.g., "context"
	Name    string // e.g., "Background"
}

// InstantiationPlan is the final, ordered list of dependencies to be generated in code.
// The root service is the last element in the slice.
type InstantiationPlan struct {
	Steps          []*Dependency
	RootDependency *Dependency
}
