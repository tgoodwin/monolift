package compiler

import (
	"go/ast"
	"strings"
)

const (
	// MonoliftPragmaPrefix is the prefix for monolift specific directives.
	MonoliftPragmaPrefix = "// @monolift"
)

// Pragma represents a parsed monolift directive.
// Attributes will store key-value pairs from the pragma string.
type Pragma struct {
	Raw        string
	Attributes map[string]string
}

// parsePragmaLine attempts to parse a single comment line that starts with MonoliftPragmaPrefix.
// It expects attributes in the format: key1=value1 key2=value2 ...
func parsePragmaLine(line string) *Pragma {
	if !strings.HasPrefix(line, MonoliftPragmaPrefix) {
		return nil
	}

	trimmedLine := strings.TrimSpace(strings.TrimPrefix(line, MonoliftPragmaPrefix))
	if trimmedLine == "" {
		return &Pragma{Raw: line, Attributes: make(map[string]string)}
	}

	attributes := make(map[string]string)
	parts := strings.Fields(trimmedLine) // Split by whitespace
	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			attributes[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		} else if len(kv) == 1 && kv[0] != "" { // Handle boolean flags or valueless keys
			attributes[strings.TrimSpace(kv[0])] = "true" // Default to "true" if no value
		}
	}
	return &Pragma{Raw: line, Attributes: attributes}
}

// GetPragmasFromCommentGroup extracts all monolift pragmas from a comment group.
func GetPragmasFromCommentGroup(cg *ast.CommentGroup) []*Pragma {
	if cg == nil {
		return nil
	}
	var pragmas []*Pragma
	for _, comment := range cg.List {
		if p := parsePragmaLine(comment.Text); p != nil {
			pragmas = append(pragmas, p)
		}
	}
	return pragmas
}

// GetFuncDeclPragmas extracts monolift pragmas associated with a function declaration.
func GetFuncDeclPragmas(funcDecl *ast.FuncDecl) []*Pragma {
	if funcDecl == nil || funcDecl.Doc == nil {
		return nil
	}
	return GetPragmasFromCommentGroup(funcDecl.Doc)
}

// GetTypeSpecPragmas extracts monolift pragmas associated with a type specification.
// This is useful for interfaces, structs, etc.
func GetTypeSpecPragmas(typeSpec *ast.TypeSpec) []*Pragma {
	if typeSpec == nil || typeSpec.Doc == nil {
		return nil
	}
	return GetPragmasFromCommentGroup(typeSpec.Doc)
}

// IsInterface checks if an ast.TypeSpec represents an interface type.
func IsInterface(typeSpec *ast.TypeSpec) bool {
	if typeSpec == nil {
		return false
	}
	_, ok := typeSpec.Type.(*ast.InterfaceType)
	return ok
}
