package compiler

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestPragmaCodesMatchSpec(t *testing.T) {
	specPath := filepath.Join("..", "..", "docs", "specs", "monolift-v2-contract.md")
	content, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}

	re := regexp.MustCompile("`(MLV2_PRAGMA_[A-Z0-9_]+)`")
	found := map[string]bool{}
	for _, match := range re.FindAllSubmatch(content, -1) {
		found[string(match[1])] = true
	}
	if len(found) == 0 {
		t.Fatalf("no MLV2_PRAGMA_* codes found in %s", specPath)
	}

	var specCodes []string
	for code := range found {
		specCodes = append(specCodes, code)
	}
	sort.Strings(specCodes)

	parserCodes := knownPragmaCodes()
	sort.Strings(parserCodes)

	if strings.Join(parserCodes, "\n") != strings.Join(specCodes, "\n") {
		t.Fatalf("pragma diagnostic codes drifted\nparser:\n%s\nspec:\n%s", strings.Join(parserCodes, "\n"), strings.Join(specCodes, "\n"))
	}
}

func TestPragmaFilesDoNotImportReportV2(t *testing.T) {
	files, err := filepath.Glob("pragma*.go")
	if err != nil {
		t.Fatalf("glob pragma files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no pragma*.go files found")
	}

	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports for %s: %v", file, err)
		}
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if strings.HasSuffix(path, "/pkg/compiler/reportv2") {
				t.Fatalf("%s imports %s; pragma parser diagnostics must remain parser-internal", file, path)
			}
		}
	}
}
