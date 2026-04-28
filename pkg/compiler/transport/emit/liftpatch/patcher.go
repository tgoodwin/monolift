package liftpatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

const collisionPrefix = "monoliftLift"

type targetDecl struct {
	path string
	file *ast.File
	fn   *ast.FuncDecl
	tags bool
}

func PatchRegion(req RegionPatchRequest) (RegionPatchResult, error) {
	if len(req.Symbols) == 0 {
		return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticTargetNotFound, Message: "region patch has no symbols"}}, nil
	}
	if refusal := validateRegionRequest(req); refusal != nil {
		return RegionPatchResult{Refused: refusal}, nil
	}

	type plannedPatch struct {
		symbol PatchSymbolRequest
		target targetDecl
		files  []targetDecl
		fset   *token.FileSet
	}
	plans := make([]plannedPatch, 0, len(req.Symbols))
	for _, symbol := range req.Symbols {
		if symbol.PackageDir == "" {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticTargetNotFound, Message: "package dir is required", Symbol: symbol}}, nil
		}
		sentinel := symbol.SentinelIdent
		if sentinel == "" {
			sentinel = regionSentinel(req.RegionName, symbol.PackageImportPath)
			symbol.SentinelIdent = sentinel
		}
		fset := token.NewFileSet()
		files, err := parsePackageFiles(fset, symbol.PackageDir)
		if err != nil {
			return RegionPatchResult{}, err
		}
		if err := scanCollisions(files); err != nil {
			if diag, ok := err.(*DiagnosticError); ok {
				return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: diag.Kind, Message: diag.Message, Symbol: symbol}}, nil
			}
			return RegionPatchResult{}, err
		}
		targets := findRegionTargets(files, symbol)
		if len(targets) == 0 {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticTargetNotFound, Message: fmt.Sprintf("symbol %s not found", symbol.FuncName), Symbol: symbol}}, nil
		}
		if len(targets) > 1 {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticAmbiguousTarget, Message: fmt.Sprintf("symbol %s found in %d files", symbol.FuncName, len(targets)), Symbol: symbol}}, nil
		}
		target := targets[0]
		if target.tags {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticUnsupportedBuildTags, Message: fmt.Sprintf("%s has build constraints", target.path), Symbol: symbol}}, nil
		}
		if symbol.ReceiverType == "" && target.fn.Recv != nil {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticMethodReceiver, Message: fmt.Sprintf("%s is a method", symbol.FuncName), Symbol: symbol}}, nil
		}
		if symbol.ReceiverType != "" && receiverTypeString(fset, target.fn) != symbol.ReceiverType {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticMethodReceiver, Message: fmt.Sprintf("receiver mismatch for %s", symbol.FuncName), Symbol: symbol}}, nil
		}
		if target.fn.Type.TypeParams != nil && len(target.fn.Type.TypeParams.List) > 0 {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticGenericFunction, Message: fmt.Sprintf("%s is generic", symbol.FuncName), Symbol: symbol}}, nil
		}
		if signature := signatureString(fset, target.fn.Type); signature != symbol.ExpectedSignature {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticSignatureMismatch, Message: fmt.Sprintf("got %q want %q", signature, symbol.ExpectedSignature), Symbol: symbol}}, nil
		}
		if hasNamedResults(target.fn.Type) && hasNakedReturn(target.fn.Body) {
			return RegionPatchResult{Refused: &RegionPatchRefusal{Kind: DiagnosticNamedNakedReturn, Message: fmt.Sprintf("%s has named results and a naked return", symbol.FuncName), Symbol: symbol}}, nil
		}
		if _, err := parsePrelude(fset, symbol.Prelude.GoSource); err != nil {
			return RegionPatchResult{}, err
		}
		plans = append(plans, plannedPatch{symbol: symbol, target: target, files: files, fset: fset})
	}

	result := RegionPatchResult{}
	for _, plan := range plans {
		fset := token.NewFileSet()
		files, err := parsePackageFiles(fset, plan.symbol.PackageDir)
		if err != nil {
			return RegionPatchResult{}, err
		}
		targets := findRegionTargets(files, plan.symbol)
		if len(targets) != 1 {
			return RegionPatchResult{}, fmt.Errorf("region patch target changed during write: %s", plan.symbol.FuncName)
		}
		target := targets[0]
		original, err := os.ReadFile(target.path)
		if err != nil {
			return RegionPatchResult{}, err
		}
		originalHash := sha256Hex(original)
		if isAlreadyApplied(target.fn, plan.symbol.SentinelIdent) {
			result.Files = append(result.Files, PatchedFileResult{
				Path:           target.path,
				OriginalSHA256: originalHash,
				PatchedSHA256:  originalHash,
				AlreadyApplied: true,
			})
			continue
		}
		prelude, err := parsePrelude(fset, plan.symbol.Prelude.GoSource)
		if err != nil {
			return RegionPatchResult{}, err
		}
		target.fn.Body.List = append(prelude, target.fn.Body.List...)
		var added []string
		for _, imp := range plan.symbol.Prelude.RequiredImports {
			if astutil.AddImport(fset, target.file, imp) {
				added = append(added, imp)
			}
		}
		sort.Strings(added)
		var formatted bytes.Buffer
		if err := format.Node(&formatted, fset, target.file); err != nil {
			return RegionPatchResult{}, err
		}
		if err := os.WriteFile(target.path, formatted.Bytes(), 0o644); err != nil {
			return RegionPatchResult{}, err
		}
		result.Files = append(result.Files, PatchedFileResult{
			Path:           target.path,
			OriginalSHA256: originalHash,
			PatchedSHA256:  sha256Hex(formatted.Bytes()),
			AddedImports:   added,
		})
	}
	for _, file := range regionGeneratedFiles(req) {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return RegionPatchResult{}, err
		}
		if err := os.WriteFile(file.Path, file.Content, 0o644); err != nil {
			return RegionPatchResult{}, err
		}
		result.GeneratedFiles = append(result.GeneratedFiles, GeneratedFileResult{Path: file.Path, SHA256: sha256Hex(file.Content)})
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	sort.Slice(result.GeneratedFiles, func(i, j int) bool { return result.GeneratedFiles[i].Path < result.GeneratedFiles[j].Path })
	return result, nil
}

func PatchSymbolBody(req PatchRequest) (PatchResult, error) {
	if req.PackageDir == "" {
		return PatchResult{}, fmt.Errorf("package dir is required")
	}
	sentinel := req.SentinelIdent
	if sentinel == "" {
		sentinel = "monoliftLiftEnabled"
	}

	fset := token.NewFileSet()
	files, err := parsePackageFiles(fset, req.PackageDir)
	if err != nil {
		return PatchResult{}, err
	}
	targets := findTargets(files, req.FuncName)
	if len(targets) == 0 {
		return PatchResult{}, diagnostic(DiagnosticTargetNotFound, "function %s not found in %s", req.FuncName, req.PackageDir)
	}
	if len(targets) > 1 {
		return PatchResult{}, diagnostic(DiagnosticAmbiguousTarget, "function %s found in %d files", req.FuncName, len(targets))
	}
	target := targets[0]
	if target.tags {
		return PatchResult{}, diagnostic(DiagnosticUnsupportedBuildTags, "%s has build constraints", target.path)
	}
	if target.fn.Recv != nil {
		return PatchResult{}, diagnostic(DiagnosticMethodReceiver, "%s is a method", req.FuncName)
	}
	if target.fn.Type.TypeParams != nil && len(target.fn.Type.TypeParams.List) > 0 {
		return PatchResult{}, diagnostic(DiagnosticGenericFunction, "%s is generic", req.FuncName)
	}
	if signature := signatureString(fset, target.fn.Type); signature != req.ExpectedSignature {
		return PatchResult{}, diagnostic(DiagnosticSignatureMismatch, "got %q want %q", signature, req.ExpectedSignature)
	}
	if hasNamedResults(target.fn.Type) && hasNakedReturn(target.fn.Body) {
		return PatchResult{}, diagnostic(DiagnosticNamedNakedReturn, "%s has named results and a naked return", req.FuncName)
	}

	original, err := os.ReadFile(target.path)
	if err != nil {
		return PatchResult{}, err
	}
	originalHash := sha256Hex(original)
	generatedPaths := generatedFilePaths(req)
	if isAlreadyApplied(target.fn, sentinel) {
		return PatchResult{
			PatchedFile:    target.path,
			GeneratedFiles: generatedPaths,
			OriginalSHA256: originalHash,
			PatchedSHA256:  originalHash,
			AlreadyApplied: true,
		}, nil
	}
	if err := scanCollisions(files); err != nil {
		return PatchResult{}, err
	}

	prelude, err := parsePrelude(fset, req.PreludeSpec.GoSource)
	if err != nil {
		return PatchResult{}, err
	}
	target.fn.Body.List = append(prelude, target.fn.Body.List...)

	var added []string
	for _, imp := range req.PreludeSpec.RequiredImports {
		if astutil.AddImport(fset, target.file, imp) {
			added = append(added, imp)
		}
	}
	sort.Strings(added)

	var formatted bytes.Buffer
	if err := format.Node(&formatted, fset, target.file); err != nil {
		return PatchResult{}, err
	}
	if err := os.WriteFile(target.path, formatted.Bytes(), 0o644); err != nil {
		return PatchResult{}, err
	}
	for _, file := range req.GeneratedFiles {
		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return PatchResult{}, err
		}
		if err := os.WriteFile(file.Path, file.Content, 0o644); err != nil {
			return PatchResult{}, err
		}
	}
	patchedHash := sha256Hex(formatted.Bytes())
	result := PatchResult{
		PatchedFile:    target.path,
		AddedImports:   added,
		GeneratedFiles: generatedPaths,
		OriginalSHA256: originalHash,
		PatchedSHA256:  patchedHash,
	}
	if err := writeManifest(req, result, sentinel); err != nil {
		return PatchResult{}, err
	}
	return result, nil
}

func validateRegionRequest(req RegionPatchRequest) *RegionPatchRefusal {
	symbols := map[string]PatchSymbolRequest{}
	files := map[string]GeneratedFile{}
	for _, symbol := range req.Symbols {
		key := symbol.PackageDir + "\x00" + symbol.ReceiverType + "\x00" + symbol.FuncName
		if previous, ok := symbols[key]; ok {
			return &RegionPatchRefusal{Kind: DiagnosticAmbiguousTarget, Message: "duplicate symbol identity", Symbol: previous}
		}
		symbols[key] = symbol
		for _, file := range symbol.GeneratedFiles {
			if previous, ok := files[file.Path]; ok && !bytes.Equal(previous.Content, file.Content) {
				return &RegionPatchRefusal{Kind: DiagnosticIdentifierCollision, Message: "generated-file collision", Symbol: symbol}
			}
			files[file.Path] = file
		}
	}
	for _, file := range req.SharedGeneratedFiles {
		if previous, ok := files[file.Path]; ok && !bytes.Equal(previous.Content, file.Content) {
			return &RegionPatchRefusal{Kind: DiagnosticIdentifierCollision, Message: "generated-file collision"}
		}
		files[file.Path] = file
	}
	return nil
}

func regionSentinel(regionName, packageImportPath string) string {
	sum := sha256.Sum256([]byte(regionName + "\x00" + packageImportPath))
	return "monolift_" + hex.EncodeToString(sum[:])[:12] + "_sentinel"
}

func findRegionTargets(files []targetDecl, symbol PatchSymbolRequest) []targetDecl {
	var targets []targetDecl
	for _, file := range files {
		if symbol.File != "" && filepath.Clean(symbol.File) != filepath.Clean(file.path) && filepath.Base(symbol.File) != filepath.Base(file.path) {
			continue
		}
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != symbol.FuncName {
				continue
			}
			if symbol.ReceiverType != "" && receiverTypeString(token.NewFileSet(), fn) != symbol.ReceiverType {
				continue
			}
			if symbol.ReceiverType == "" && fn.Recv != nil {
				continue
			}
			file.fn = fn
			targets = append(targets, file)
		}
	}
	return targets
}

func receiverTypeString(fset *token.FileSet, fn *ast.FuncDecl) string {
	if fn == nil || fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, fn.Recv.List[0].Type)
	return buf.String()
}

func regionGeneratedFiles(req RegionPatchRequest) []GeneratedFile {
	files := append([]GeneratedFile(nil), req.SharedGeneratedFiles...)
	for _, symbol := range req.Symbols {
		files = append(files, symbol.GeneratedFiles...)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func parsePackageFiles(fset *token.FileSet, dir string) ([]targetDecl, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []targetDecl
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		out = append(out, targetDecl{path: path, file: file, tags: hasBuildTags(src)})
	}
	return out, nil
}

func findTargets(files []targetDecl, name string) []targetDecl {
	var targets []targetDecl
	for _, file := range files {
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != name {
				continue
			}
			file.fn = fn
			targets = append(targets, file)
		}
	}
	return targets
}

func hasBuildTags(src []byte) bool {
	for _, line := range strings.Split(string(src), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || !strings.HasPrefix(trimmed, "//") {
			if trimmed != "" && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
				return false
			}
			continue
		}
		if strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build") {
			return true
		}
	}
	return false
}

func signatureString(fset *token.FileSet, typ *ast.FuncType) string {
	params := fieldTypes(fset, typ.Params, false)
	results := fieldTypes(fset, typ.Results, true)
	if results == "" {
		return fmt.Sprintf("func(%s)", params)
	}
	return fmt.Sprintf("func(%s) %s", params, results)
}

func fieldTypes(fset *token.FileSet, fields *ast.FieldList, result bool) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	for _, field := range fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		var buf bytes.Buffer
		_ = printer.Fprint(&buf, fset, field.Type)
		for i := 0; i < count; i++ {
			parts = append(parts, buf.String())
		}
	}
	if result && len(parts) > 1 {
		return "(" + strings.Join(parts, ", ") + ")"
	}
	return strings.Join(parts, ", ")
}

func hasNamedResults(typ *ast.FuncType) bool {
	if typ.Results == nil {
		return false
	}
	for _, field := range typ.Results.List {
		if len(field.Names) > 0 {
			return true
		}
	}
	return false
}

func hasNakedReturn(body *ast.BlockStmt) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		ret, ok := node.(*ast.ReturnStmt)
		if ok && len(ret.Results) == 0 {
			found = true
			return false
		}
		return true
	})
	return found
}

func isAlreadyApplied(fn *ast.FuncDecl, sentinel string) bool {
	if fn.Body == nil || len(fn.Body.List) == 0 {
		return false
	}
	stmt, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok {
		return false
	}
	ident, ok := stmt.Cond.(*ast.Ident)
	return ok && ident.Name == sentinel
}

func scanCollisions(files []targetDecl) error {
	for _, file := range files {
		for _, decl := range file.file.Decls {
			switch decl := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							if strings.HasPrefix(name.Name, collisionPrefix) {
								return diagnostic(DiagnosticIdentifierCollision, "%s in %s", name.Name, file.path)
							}
						}
					case *ast.TypeSpec:
						if strings.HasPrefix(spec.Name.Name, collisionPrefix) {
							return diagnostic(DiagnosticIdentifierCollision, "%s in %s", spec.Name.Name, file.path)
						}
					}
				}
			case *ast.FuncDecl:
				if strings.HasPrefix(decl.Name.Name, collisionPrefix) {
					return diagnostic(DiagnosticIdentifierCollision, "%s in %s", decl.Name.Name, file.path)
				}
			}
		}
	}
	return nil
}

func parsePrelude(fset *token.FileSet, src string) ([]ast.Stmt, error) {
	wrapped := "package liftpatch\nfunc _() {\n" + src + "\n}\n"
	file, err := parser.ParseFile(fset, "prelude.go", wrapped, 0)
	if err != nil {
		return nil, err
	}
	fn := file.Decls[0].(*ast.FuncDecl)
	return fn.Body.List, nil
}

func generatedFilePaths(req PatchRequest) []string {
	paths := make([]string, 0, len(req.GeneratedFiles))
	for _, file := range req.GeneratedFiles {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}

func writeManifest(req PatchRequest, result PatchResult, sentinel string) error {
	manifest := LiftPatchManifest{
		PackageImportPath: req.PackageImportPath,
		FilePath:          result.PatchedFile,
		FunctionName:      req.FuncName,
		ExpectedSignature: req.ExpectedSignature,
		SentinelIdent:     sentinel,
		OriginalSHA256:    result.OriginalSHA256,
		PatchedSHA256:     result.PatchedSHA256,
		GeneratedFiles:    result.GeneratedFiles,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(req.PackageDir, "LIFTPATCH.json"), data, 0o644)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func diagnostic(kind DiagnosticKind, format string, args ...any) *DiagnosticError {
	return &DiagnosticError{Kind: kind, Message: fmt.Sprintf(format, args...)}
}
