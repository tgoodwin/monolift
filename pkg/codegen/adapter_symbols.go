package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"strconv"
	"strings"
)

// helperBodyRefs records the external symbols a (rewritten) helper body
// depends on so the adapter renderer can re-emit the right imports and copy
// referenced package-level constants. The scan is target-agnostic: it names
// no package and no symbol.
type helperBodyRefs struct {
	// pkgRefs holds local names used as the X in a selector X.Sel — candidate
	// imported-package qualifiers. Over-collection is harmless: the names are
	// later intersected with the cut file's actual import set.
	pkgRefs map[string]bool
	// valueRefs holds identifiers used in value position that are not bound
	// within the body. Over-collection is harmless for constants (an unused
	// copied const still compiles) and only risks a spurious free-var refusal.
	valueRefs map[string]bool
}

// scanHelperBodyRefs walks a rewritten helper body collecting package
// qualifiers and free value identifiers, excluding anything in bound.
func scanHelperBodyRefs(body *ast.BlockStmt, bound map[string]bool) helperBodyRefs {
	refs := helperBodyRefs{pkgRefs: map[string]bool{}, valueRefs: map[string]bool{}}
	skip := map[*ast.Ident]bool{}
	var all []*ast.Ident
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := node.X.(*ast.Ident); ok {
				refs.pkgRefs[x.Name] = true
				skip[x] = true
			}
			skip[node.Sel] = true
		case *ast.Ident:
			all = append(all, node)
		}
		return true
	})
	for _, id := range all {
		if skip[id] || bound[id.Name] {
			continue
		}
		switch id.Name {
		case "_", "nil", "true", "false", "iota":
			continue
		}
		refs.valueRefs[id.Name] = true
	}
	return refs
}

// helperBoundNames collects identifiers that are locally bound in the helper
// (so they are not mistaken for free symbols): the normalized parameters and
// any names introduced inside the body.
func helperBoundNames(plan *Plan, body *ast.BlockStmt) map[string]bool {
	bound := map[string]bool{}
	norm := normalizedAdapterPlan(plan)
	for _, p := range norm.BoundaryParams {
		bound[p.Name] = true
	}
	for _, rp := range norm.ReconstructedParams {
		bound[rp.Param.Name] = true
	}
	if norm.ReceiverParam != nil {
		bound["recv"] = true
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				bindIdents(bound, node.Lhs)
			}
		case *ast.ValueSpec:
			for _, name := range node.Names {
				bound[name.Name] = true
			}
		case *ast.RangeStmt:
			bindIdents(bound, []ast.Expr{node.Key, node.Value})
		case *ast.TypeSwitchStmt:
			if as, ok := node.Assign.(*ast.AssignStmt); ok {
				bindIdents(bound, as.Lhs)
			}
		case *ast.FuncLit:
			if node.Type != nil && node.Type.Params != nil {
				for _, f := range node.Type.Params.List {
					for _, name := range f.Names {
						bound[name.Name] = true
					}
				}
			}
		}
		return true
	})
	return bound
}

func bindIdents(bound map[string]bool, exprs []ast.Expr) {
	for _, expr := range exprs {
		if id, ok := expr.(*ast.Ident); ok {
			bound[id.Name] = true
		}
	}
}

// cutFileImportsFor returns the cut file's import specs whose local name is
// referenced by the rewritten body. The intersection guarantees the helper's
// import block is exactly what it uses — gofmt does not add or drop imports.
func cutFileImportsFor(file *ast.File, pkgRefs map[string]bool) []importSpec {
	var out []importSpec
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		local := importLocalName(imp, path)
		if local == "_" || local == "." {
			continue
		}
		if !pkgRefs[local] {
			continue
		}
		spec := importSpec{Path: path}
		if imp.Name != nil && imp.Name.Name != importPathBase(path) {
			spec.Alias = imp.Name.Name
		}
		out = append(out, spec)
	}
	return uniqueImports(out)
}

func importLocalName(imp *ast.ImportSpec, path string) string {
	if imp.Name != nil {
		return imp.Name.Name
	}
	return importPathBase(path)
}

func importPathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

// cutFileFreeConsts renders the package-level constant declarations the helper
// body references so they can be copied into an extracted service where the
// cut package is not in scope. A referenced package-level var is conservatively
// refused: snapshotting a var that the monolith mutates at runtime would
// silently diverge, and distinguishing a never-written init-time var (safe to
// copy, including non-const-able values like slices or regexp.MustCompile)
// from mutable shared state needs write-analysis on the *ssa.Global that this
// render-time scan does not have. Until that analysis lands the lift fails
// closed. See the SPRINT-0052 follow-up non-goal.
func cutFileFreeConsts(fset *token.FileSet, file *ast.File, valueRefs map[string]bool) ([]string, error) {
	var consts []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		switch gd.Tok {
		case token.CONST:
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || !specDefinesAny(vs, valueRefs) {
					continue
				}
				rendered, err := renderConstSpec(fset, vs)
				if err != nil {
					return nil, err
				}
				consts = append(consts, rendered)
			}
		case token.VAR:
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if valueRefs[name.Name] {
						return nil, fmt.Errorf("codegen: adapter helper references package-level var %q; conservatively refused — distinguishing an immutable init-time var from mutable shared state needs write-analysis (deferred, see SPRINT-0052 follow-up)", name.Name)
					}
				}
			}
		}
	}
	return consts, nil
}

func specDefinesAny(vs *ast.ValueSpec, names map[string]bool) bool {
	for _, n := range vs.Names {
		if names[n.Name] {
			return true
		}
	}
	return false
}

func renderConstSpec(fset *token.FileSet, vs *ast.ValueSpec) (string, error) {
	if len(vs.Values) == 0 {
		return "", fmt.Errorf("codegen: free const %s has no explicit value (iota-derived constants cannot be copied)", identNames(vs.Names))
	}
	typ := ""
	if vs.Type != nil {
		t, err := printNode(fset, vs.Type)
		if err != nil {
			return "", err
		}
		typ = " " + t
	}
	vals := make([]string, len(vs.Values))
	for i, v := range vs.Values {
		s, err := printNode(fset, v)
		if err != nil {
			return "", err
		}
		vals[i] = s
	}
	return "const " + identNames(vs.Names) + typ + " = " + strings.Join(vals, ", "), nil
}

func identNames(idents []*ast.Ident) string {
	parts := make([]string, len(idents))
	for i, id := range idents {
		parts[i] = id.Name
	}
	return strings.Join(parts, ", ")
}

func printNode(fset *token.FileSet, n ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}
