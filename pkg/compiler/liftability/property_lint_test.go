package liftability

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const allowStringLiteralsDirective = "liftability:allow-string-literals"

var propertyLiteralPattern = regexp.MustCompile(`^(boundary|contract|effects|lifecycle|transport|state)\.[a-z]+(\.[a-z_-]+)*$`)

func TestNoBarePropertyIDStringLiterals(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", ".."))
	for _, dir := range []string{"pkg", "cmd"} {
		walkRoot := filepath.Join(root, dir)
		if err := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || filepath.Base(path) == "property.go" {
				return nil
			}
			checkPropertyLiterals(t, path)
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", walkRoot, err)
		}
	}
}

func checkPropertyLiterals(t *testing.T, path string) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if hasAllowStringLiteralsDirective(file) {
		return
	}

	allowed := map[token.Pos]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.ValueSpec:
			if typed.Names != nil && typed.Values != nil && typed.Doc == nil {
				for _, value := range typed.Values {
					markBasicLits(value, allowed)
				}
			}
		case *ast.Field:
			if typed.Tag != nil {
				allowed[typed.Tag.Pos()] = true
			}
		}
		return true
	})

	ast.Inspect(file, func(node ast.Node) bool {
		lit, ok := node.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING || allowed[lit.Pos()] {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if propertyLiteralPattern.MatchString(value) {
			pos := fset.Position(lit.Pos())
			t.Errorf("%s: bare property-ID string literal %q; use a liftability.PropertyID constant", pos, value)
		}
		return true
	})
}

func hasAllowStringLiteralsDirective(file *ast.File) bool {
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.Contains(comment.Text, allowStringLiteralsDirective) {
				return true
			}
		}
	}
	return false
}

func markBasicLits(expr ast.Expr, allowed map[token.Pos]bool) {
	ast.Inspect(expr, func(node ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			allowed[lit.Pos()] = true
		}
		return true
	})
}
