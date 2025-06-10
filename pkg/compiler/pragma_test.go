package compiler

import (
	"go/ast"
	"reflect"
	"testing"
)

func TestParsePragmaLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected *Pragma
	}{
		{
			name: "valid pragma with multiple attributes",
			line: "// @monolift trigger=CPU threshold=0.5 name=test",
			expected: &Pragma{
				Raw:        "// @monolift trigger=CPU threshold=0.5 name=test",
				Attributes: map[string]string{"trigger": "CPU", "threshold": "0.5", "name": "test"},
			},
		},
		{
			name: "valid pragma with single attribute",
			line: "// @monolift service=users",
			expected: &Pragma{
				Raw:        "// @monolift service=users",
				Attributes: map[string]string{"service": "users"},
			},
		},
		{
			name: "valid pragma with boolean flag",
			line: "// @monolift async",
			expected: &Pragma{
				Raw:        "// @monolift async",
				Attributes: map[string]string{"async": "true"},
			},
		},
		{
			name: "valid pragma with no attributes",
			line: "// @monolift",
			expected: &Pragma{
				Raw:        "// @monolift",
				Attributes: map[string]string{},
			},
		},
		{
			name: "valid pragma with extra spaces",
			line: "// @monolift  key = value  another= true ",
			expected: &Pragma{
				Raw:        "// @monolift  key = value  another= true ",
				Attributes: map[string]string{"key": "value", "another": "true"},
			},
		},
		{
			name:     "invalid prefix",
			line:     "// @otherdirective key=value",
			expected: nil,
		},
		{
			name: "malformed attribute (no equals)",
			line: "// @monolift keyvalue",
			expected: &Pragma{
				Raw:        "// @monolift keyvalue",
				Attributes: map[string]string{"keyvalue": "true"},
			},
		},
		{
			name: "malformed attribute (empty key)",
			line: "// @monolift =value",
			expected: &Pragma{ // Current behavior: treats "=value" as a key with value "true"
				Raw:        "// @monolift =value",
				Attributes: map[string]string{"=value": "true"},
			},
		},
		{
			name:     "empty line",
			line:     "",
			expected: nil,
		},
		{
			name:     "just a comment",
			line:     "// This is a normal comment",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePragmaLine(tt.line)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parsePragmaLine() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetPragmasFromCommentGroup(t *testing.T) {
	tests := []struct {
		name     string
		cg       *ast.CommentGroup
		expected []*Pragma
	}{
		{
			name:     "nil comment group",
			cg:       nil,
			expected: nil,
		},
		{
			name:     "empty comment group",
			cg:       &ast.CommentGroup{List: []*ast.Comment{}},
			expected: []*Pragma{},
		},
		{
			name: "comment group with no monolift pragmas",
			cg: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "// regular comment"},
				{Text: "/* block comment */"},
			}},
			expected: []*Pragma{},
		},
		{
			name: "comment group with one monolift pragma",
			cg: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "// @monolift key=value"},
			}},
			expected: []*Pragma{
				{Raw: "// @monolift key=value", Attributes: map[string]string{"key": "value"}},
			},
		},
		{
			name: "comment group with multiple monolift pragmas",
			cg: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "// @monolift first=1"},
				{Text: "// @monolift second=2 boolflag"},
			}},
			expected: []*Pragma{
				{Raw: "// @monolift first=1", Attributes: map[string]string{"first": "1"}},
				{Raw: "// @monolift second=2 boolflag", Attributes: map[string]string{"second": "2", "boolflag": "true"}},
			},
		},
		{
			name: "comment group with mixed comments",
			cg: &ast.CommentGroup{List: []*ast.Comment{
				{Text: "// Some leading comment"},
				{Text: "// @monolift target=api"},
				{Text: "/* A block comment in between */"},
				{Text: "// @monolift version=v1"},
				{Text: "// Trailing comment"},
			}},
			expected: []*Pragma{
				{Raw: "// @monolift target=api", Attributes: map[string]string{"target": "api"}},
				{Raw: "// @monolift version=v1", Attributes: map[string]string{"version": "v1"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPragmasFromCommentGroup(tt.cg)
			if len(got) == 0 && len(tt.expected) == 0 { // Handle empty slices being non-nil vs nil
				return
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetPragmasFromCommentGroup() got = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetFuncDeclPragmas(t *testing.T) {
	// Re-use GetPragmasFromCommentGroup logic, just test the Doc extraction
	pragmaComment := &ast.Comment{Text: "// @monolift func_attr=true"}
	docWithPragma := &ast.CommentGroup{List: []*ast.Comment{pragmaComment}}

	tests := []struct {
		name     string
		funcDecl *ast.FuncDecl
		expected []*Pragma
	}{
		{"nil FuncDecl", nil, nil},
		{"FuncDecl with no Doc", &ast.FuncDecl{Name: ast.NewIdent("testFunc")}, nil},
		{"FuncDecl with Doc and pragma", &ast.FuncDecl{Name: ast.NewIdent("testFunc"), Doc: docWithPragma}, []*Pragma{{Raw: pragmaComment.Text, Attributes: map[string]string{"func_attr": "true"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFuncDeclPragmas(tt.funcDecl); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetFuncDeclPragmas() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGetTypeSpecPragmas(t *testing.T) {
	// Re-use GetPragmasFromCommentGroup logic
	pragmaComment := &ast.Comment{Text: "// @monolift type_attr=interface"}
	docWithPragma := &ast.CommentGroup{List: []*ast.Comment{pragmaComment}}

	tests := []struct {
		name     string
		typeSpec *ast.TypeSpec
		expected []*Pragma
	}{
		{"nil TypeSpec", nil, nil},
		{"TypeSpec with no Doc", &ast.TypeSpec{Name: ast.NewIdent("MyType")}, nil},
		{"TypeSpec with Doc and pragma", &ast.TypeSpec{Name: ast.NewIdent("MyType"), Doc: docWithPragma}, []*Pragma{{Raw: pragmaComment.Text, Attributes: map[string]string{"type_attr": "interface"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetTypeSpecPragmas(tt.typeSpec); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetTypeSpecPragmas() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsInterface(t *testing.T) {
	tests := []struct {
		name     string
		typeSpec *ast.TypeSpec
		expected bool
	}{
		{"nil TypeSpec", nil, false},
		{"TypeSpec is Interface", &ast.TypeSpec{Type: &ast.InterfaceType{}}, true},
		{"TypeSpec is Struct", &ast.TypeSpec{Type: &ast.StructType{}}, false},
		{"TypeSpec is Ident", &ast.TypeSpec{Type: ast.NewIdent("int")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInterface(tt.typeSpec); got != tt.expected {
				t.Errorf("IsInterface() = %v, want %v", got, tt.expected)
			}
		})
	}
}
