package diagnostics

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnosticsFilesDoNotImportPragmaPackages(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob diagnostics files: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no diagnostics package files found")
	}

	for _, file := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports for %s: %v", file, err)
		}
		for _, imported := range parsed.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if strings.Contains(path, "/pkg/compiler/pragma") {
				t.Fatalf("%s imports %s; diagnostics must depend on compiler.Diagnostic values, not pragma internals", file, path)
			}
		}
	}
}
