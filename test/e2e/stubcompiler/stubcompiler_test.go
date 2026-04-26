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
			case "miniflux":
				args = append(args, "--source=../../../evaluation/miniflux")
			case "pocketbase":
				args = append(args, "--source=../../../evaluation/pocketbase")
			}
			cmd := exec.Command("go", args...)
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.0")
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
	hostPatch := filepath.Join(lifted, "host-patch")
	if _, err := os.Stat(filepath.Join(lifted, "upstream")); !os.IsNotExist(err) {
		t.Fatalf("lifted/upstream exists or could not be checked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lifted, "extracted-cleanpath")); !os.IsNotExist(err) {
		t.Fatalf("legacy extracted-cleanpath tree exists or could not be checked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(lifted, "extracted-sanitizemethod")); !os.IsNotExist(err) {
		t.Fatalf("legacy extracted-sanitizemethod tree exists or could not be checked: %v", err)
	}

	assertFunctionOnlyPatch(t,
		filepath.Join(repoRoot(), "evaluation", "caddy", "modules", "caddyhttp", "caddyhttp.go"),
		filepath.Join(hostPatch, "modules", "caddyhttp", "caddyhttp.go"),
		"CleanPath",
	)
	assertFunctionOnlyPatch(t,
		filepath.Join(repoRoot(), "evaluation", "caddy", "internal", "metrics", "metrics.go"),
		filepath.Join(hostPatch, "internal", "metrics", "metrics.go"),
		"SanitizeMethod",
	)

	for _, tc := range []struct {
		path string
		pkg  string
		name string
	}{
		{
			path: filepath.Join(hostPatch, "modules", "caddyhttp", "monolift_lift_cleanpath.go"),
			pkg:  "caddyhttp",
			name: "CleanPath",
		},
		{
			path: filepath.Join(hostPatch, "internal", "metrics", "monolift_lift_sanitizemethod.go"),
			pkg:  "metrics",
			name: "SanitizeMethod",
		},
	} {
		clientFile := parseFile(t, tc.path)
		if clientFile.Name.Name != tc.pkg {
			t.Fatalf("%s package = %s, want %s", tc.path, clientFile.Name.Name, tc.pkg)
		}
		if !hasValue(clientFile, "monoliftLiftClient") || !hasConst(clientFile, "monoliftLiftFailureSentinel") {
			t.Fatalf("%s lift client missing http client var or failure sentinel", tc.name)
		}
	}

	for _, tc := range []struct {
		path string
		pkg  string
		name string
	}{
		{
			path: filepath.Join(hostPatch, "cmd", "monolift-extracted-cleanpath", "main.go"),
			pkg:  "caddyhttp",
			name: "CleanPath",
		},
		{
			path: filepath.Join(hostPatch, "cmd", "monolift-extracted-sanitizemethod", "main.go"),
			pkg:  "metrics",
			name: "SanitizeMethod",
		},
	} {
		extractedMain := parseFile(t, tc.path)
		if !hasSelectorCall(extractedMain, tc.pkg, tc.name) {
			t.Fatalf("extracted main.go does not call %s.%s", tc.pkg, tc.name)
		}
	}

	for _, tc := range []struct {
		path string
		name string
	}{
		{path: filepath.Join(hostPatch, "modules", "caddyhttp", "LIFTPATCH.json"), name: "CleanPath"},
		{path: filepath.Join(hostPatch, "internal", "metrics", "LIFTPATCH.json"), name: "SanitizeMethod"},
	} {
		manifestData, err := os.ReadFile(tc.path)
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
		if manifest.FunctionName != tc.name || manifest.OriginalSHA256 == "" || manifest.PatchedSHA256 == "" {
			t.Fatalf("bad LIFTPATCH manifest: %+v", manifest)
		}
	}
	if _, err := os.ReadFile(filepath.Join(lifted, "MANIFEST.json")); err != nil {
		t.Fatal(err)
	}

	goBuild(t, hostPatch, "-mod=mod", "./cmd/...")
	makeVerifyEvaluationUntouched(t)
}

func TestEmitsLiftedTreeForMiniflux(t *testing.T) {
	before := hashTree(t, filepath.Join(repoRoot(), "evaluation", "miniflux"))
	out := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--target=miniflux", "--output="+out, "--source=../../../evaluation/miniflux")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.0")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stubcompiler failed: %v\n%s", err, data)
	}
	after := hashTree(t, filepath.Join(repoRoot(), "evaluation", "miniflux"))
	if before != after {
		t.Fatalf("evaluation/miniflux hash changed: before=%s after=%s", before, after)
	}

	lifted := filepath.Join(out, "lifted")
	hostPatch := filepath.Join(lifted, "host-patch")
	assertFunctionOnlyPatch(t,
		filepath.Join(repoRoot(), "evaluation", "miniflux", "internal", "reader", "readingtime", "readingtime.go"),
		filepath.Join(hostPatch, "internal", "reader", "readingtime", "readingtime.go"),
		"EstimateReadingTime",
	)

	clientFile := parseFile(t, filepath.Join(hostPatch, "internal", "reader", "readingtime", "monolift_lift_estimatereadingtime.go"))
	if clientFile.Name.Name != "readingtime" {
		t.Fatalf("lift client package = %s, want readingtime", clientFile.Name.Name)
	}
	if !hasValue(clientFile, "monoliftLiftClient") || !hasConst(clientFile, "monoliftLiftFailureSentinel") {
		t.Fatal("miniflux lift client missing http client var or failure sentinel")
	}

	extractedMain := parseFile(t, filepath.Join(hostPatch, "cmd", "monolift-extracted-estimatereadingtime", "main.go"))
	if !hasSelectorCall(extractedMain, "readingtime", "EstimateReadingTime") {
		t.Fatal("extracted main.go does not call readingtime.EstimateReadingTime")
	}
	oracleMain := parseFile(t, filepath.Join(hostPatch, "cmd", "monolift-oracle-estimatereadingtime", "main.go"))
	if !hasSelectorCall(oracleMain, "readingtime", "EstimateReadingTime") {
		t.Fatal("oracle main.go does not call readingtime.EstimateReadingTime")
	}

	manifestData, err := os.ReadFile(filepath.Join(hostPatch, "internal", "reader", "readingtime", "LIFTPATCH.json"))
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
	if manifest.FunctionName != "EstimateReadingTime" || manifest.OriginalSHA256 == "" || manifest.PatchedSHA256 == "" {
		t.Fatalf("bad LIFTPATCH manifest: %+v", manifest)
	}

	for _, path := range []string{
		"Dockerfile.host",
		"Dockerfile.extracted-estimatereadingtime",
		"Dockerfile.oracle-estimatereadingtime",
		"manifests/miniflux-lifted-deployment.yaml",
		"manifests/miniflux-lifted-service.yaml",
		"manifests/extracted-estimatereadingtime-deployment.yaml",
		"manifests/extracted-estimatereadingtime-service.yaml",
		"manifests/oracle-estimatereadingtime-deployment.yaml",
		"manifests/oracle-estimatereadingtime-service.yaml",
		"MANIFEST.json",
	} {
		if _, err := os.ReadFile(filepath.Join(lifted, filepath.FromSlash(path))); err != nil {
			t.Fatalf("%s missing: %v", path, err)
		}
	}

	goBuild(t, hostPatch, "-mod=mod", ".")
	goBuild(t, hostPatch, "-mod=mod", "./cmd/monolift-extracted-estimatereadingtime")
	goBuild(t, hostPatch, "-mod=mod", "./cmd/monolift-oracle-estimatereadingtime")
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

func TestMinifluxSourceTreeUntouched(t *testing.T) {
	before := hashTree(t, filepath.Join(repoRoot(), "evaluation", "miniflux"))
	out := t.TempDir()
	cmd := exec.Command("go", "run", ".", "--target=miniflux", "--output="+out, "--source=../../../evaluation/miniflux")
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.0")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("stubcompiler failed: %v\n%s", err, data)
	}
	after := hashTree(t, filepath.Join(repoRoot(), "evaluation", "miniflux"))
	if before != after {
		t.Fatalf("evaluation/miniflux hash changed: before=%s after=%s", before, after)
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
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			sum.Write([]byte("symlink:"))
			sum.Write([]byte(target))
			sum.Write([]byte{0})
			continue
		}
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

func assertFunctionOnlyPatch(t *testing.T, originalPath, patchedPath, funcName string) {
	t.Helper()
	original := parseFile(t, originalPath)
	patched := parseFile(t, patchedPath)
	if !sameImports(original, patched) {
		t.Fatalf("patched %s changed imports", patchedPath)
	}
	patchedFunc := findFunc(t, patched, funcName)
	if len(patchedFunc.Body.List) == 0 {
		t.Fatalf("patched %s body is empty", funcName)
	}
	ifStmt, ok := patchedFunc.Body.List[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("first %s statement = %T, want *ast.IfStmt", funcName, patchedFunc.Body.List[0])
	}
	ident, ok := ifStmt.Cond.(*ast.Ident)
	if !ok || ident.Name != "monoliftLiftEnabled" {
		t.Fatalf("%s prelude condition = %#v", funcName, ifStmt.Cond)
	}
	originalFunc := findFunc(t, original, funcName)
	if len(patchedFunc.Body.List) != len(originalFunc.Body.List)+1 {
		t.Fatalf("patched %s body has %d statements, want prepended sentinel plus original %d statements", funcName, len(patchedFunc.Body.List), len(originalFunc.Body.List))
	}
	originalBody := formatStmtList(t, originalFunc.Body.List)
	patchedTail := formatStmtList(t, patchedFunc.Body.List[1:])
	if !bytes.Equal(originalBody, patchedTail) {
		t.Fatalf("patched %s does not preserve original %s body after sentinel prelude", patchedPath, funcName)
	}

	originalClone := cloneWithoutFunctionBody(t, originalPath, funcName)
	patchedClone := cloneWithoutFunctionBody(t, patchedPath, funcName)
	originalBytes := formatNode(t, originalClone)
	patchedBytes := formatNode(t, patchedClone)
	if !bytes.Equal(originalBytes, patchedBytes) {
		t.Fatalf("patched %s differs outside %s body", patchedPath, funcName)
	}
}

func cloneWithoutFunctionBody(t *testing.T, path, funcName string) *ast.File {
	t.Helper()
	file := parseFile(t, path)
	findFunc(t, file, funcName).Body = nil
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

func formatStmtList(t *testing.T, stmts []ast.Stmt) []byte {
	t.Helper()
	return formatNode(t, &ast.BlockStmt{List: stmts})
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
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=go1.26.0")
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build %s: %v\n%s", dir, err, data)
	}
}

func makeVerifyEvaluationUntouched(t *testing.T) {
	t.Helper()
	cmd := exec.Command("make", "verify-evaluation-untouched")
	cmd.Dir = repoRoot()
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make verify-evaluation-untouched: %v\n%s", err, data)
	}
}
