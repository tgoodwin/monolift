package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestStubCompilerFixturesValidate(t *testing.T) {
	for _, target := range []string{"caddy", "pocketbase", "miniflux"} {
		t.Run(target, func(t *testing.T) {
			if target == "caddy" && testing.Short() {
				t.Skip("SSA-heavy; load real evaluation corpus")
			}
			out := t.TempDir()
			args := []string{"run", ".", "--target=" + target, "--output=" + out}
			switch target {
			case "caddy":
				args = append(args, "--source=../../../evaluation/caddy")
			case "pocketbase":
				args = append(args, "--source=../../../evaluation/pocketbase")
			}
			cmd := exec.Command("go", args...)
			data, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("stubcompiler failed: %v\n%s", err, data)
			}
			reportData, err := os.ReadFile(filepath.Join(out, "closure-report.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := reportv2.Validate(reportData); err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if target == "caddy" {
				if _, err := os.Stat(filepath.Join(out, "lifted", "manifests", "caddy-lifted-deployment.yaml")); err != nil {
					t.Fatalf("expected lifted deployment artifact: %v", err)
				}
			}
		})
	}
}

func TestEmitsLiftedTreeForCaddy(t *testing.T) {
	before := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	out := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--target=caddy", "--output="+out, "--source=../../../evaluation/caddy")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stubcompiler failed: %v\n%s", err, data)
	}
	after := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	if before != after {
		t.Fatalf("evaluation/caddy hash changed: before=%s after=%s", before, after)
	}

	lifted := filepath.Join(out, "lifted")
	original := filepath.Join(repoRoot(), "evaluation", "caddy", "modules", "caddyhttp", "caddyhttp.go")
	patched := filepath.Join(lifted, "host-patch", "modules", "caddyhttp", "caddyhttp.go")
	assertCleanPathOnlyPatch(t, original, patched)

	clientPath := filepath.Join(lifted, "host-patch", "modules", "caddyhttp", "monolift_lift_cleanpath.go")
	clientFile := parseFile(t, clientPath)
	if !hasValue(clientFile, "monoliftLiftClient") || !hasConst(clientFile, "monoliftLiftFailureSentinel") {
		t.Fatal("lift client missing http client var or failure sentinel")
	}

	extractedMain := parseFile(t, filepath.Join(lifted, "extracted-cleanpath", "main.go"))
	if !hasSelectorCall(extractedMain, "caddyhttp", "CleanPath") {
		t.Fatal("extracted main.go does not call caddyhttp.CleanPath")
	}

	manifestData, err := os.ReadFile(filepath.Join(lifted, "host-patch", "modules", "caddyhttp", "LIFTPATCH.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		OriginalSHA256 string `json:"original_sha256"`
		PatchedSHA256  string `json:"patched_sha256"`
		FunctionName   string `json:"function_name"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.FunctionName != "CleanPath" || manifest.OriginalSHA256 == "" || manifest.PatchedSHA256 == "" {
		t.Fatalf("bad LIFTPATCH manifest: %+v", manifest)
	}
	if _, err := os.ReadFile(filepath.Join(lifted, "MANIFEST.json")); err != nil {
		t.Fatal(err)
	}

	goBuild(t, filepath.Join(lifted, "host-patch"), "./...")
	goBuild(t, filepath.Join(lifted, "extracted-cleanpath"), "-mod=mod", "./...")
}

func TestCaddySourceTreeUntouched(t *testing.T) {
	before := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	out := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--target=caddy", "--output="+out, "--source=../../../evaluation/caddy")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stubcompiler failed: %v\n%s", err, data)
	}
	after := hashTree(t, filepath.Join(repoRoot(), "evaluation", "caddy"))
	if before != after {
		t.Fatalf("evaluation/caddy hash changed: before=%s after=%s", before, after)
	}
}

func hashTree(t *testing.T, root string) string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	sum := sha256.New()
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		sum.Write([]byte(filepath.ToSlash(rel)))
		sum.Write([]byte{0})
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(sum, file); err != nil {
			file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func assertCleanPathOnlyPatch(t *testing.T, originalPath, patchedPath string) {
	t.Helper()
	original := parseFile(t, originalPath)
	patched := parseFile(t, patchedPath)
	if !sameImports(original, patched) {
		t.Fatal("patched caddyhttp.go changed imports")
	}
	patchedCleanPath := findFunc(t, patched, "CleanPath")
	if len(patchedCleanPath.Body.List) == 0 {
		t.Fatal("patched CleanPath body is empty")
	}
	ifStmt, ok := patchedCleanPath.Body.List[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("first CleanPath statement = %T, want *ast.IfStmt", patchedCleanPath.Body.List[0])
	}
	ident, ok := ifStmt.Cond.(*ast.Ident)
	if !ok || ident.Name != "monoliftLiftEnabled" {
		t.Fatalf("CleanPath prelude condition = %#v", ifStmt.Cond)
	}

	originalClone := cloneWithoutCleanPathBody(t, originalPath)
	patchedClone := cloneWithoutCleanPathBody(t, patchedPath)
	originalBytes := formatNode(t, originalClone)
	patchedBytes := formatNode(t, patchedClone)
	if !bytes.Equal(originalBytes, patchedBytes) {
		t.Fatal("patched caddyhttp.go differs outside CleanPath body")
	}
}

func cloneWithoutCleanPathBody(t *testing.T, path string) *ast.File {
	t.Helper()
	file := parseFile(t, path)
	findFunc(t, file, "CleanPath").Body = nil
	return file
}

func parseFile(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func findFunc(t *testing.T, file *ast.File, name string) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == name {
			return fn
		}
	}
	t.Fatalf("%s not found", name)
	return nil
}

func formatNode(t *testing.T, node any) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), node); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func sameImports(a, b *ast.File) bool {
	if len(a.Imports) != len(b.Imports) {
		return false
	}
	for i := range a.Imports {
		if a.Imports[i].Path.Value != b.Imports[i].Path.Value {
			return false
		}
	}
	return true
}

func hasSelectorCall(file *ast.File, pkg, name string) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != name {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == pkg {
			found = true
			return false
		}
		return true
	})
	return found
}

func hasValue(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for _, ident := range value.Names {
				if ident.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func hasConst(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value := spec.(*ast.ValueSpec)
			for _, ident := range value.Names {
				if ident.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func goBuild(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"build"}, args...)...)
	cmd.Dir = dir
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build %s: %v\n%s", dir, err, data)
	}
}
