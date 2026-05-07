package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchCallsiteRewritesSelectedCall(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.23\n")
	writeTestFile(t, filepath.Join(root, "callee", "callee.go"), `package callee

func Work(value string) string {
	return value
}
`)
	writeTestFile(t, filepath.Join(root, "callee", "monolift_lift_WORK.go"), `package callee

func Work_monolift(value string) string {
	return Work(value)
}
`)
	callerPath := filepath.Join(root, "caller", "caller.go")
	callerSource := `package caller

import "example.com/app/callee"

func Run() string {
	// preserved call comment
	return callee.Work("ok")
}
`
	writeTestFile(t, callerPath, callerSource)
	line, column := sourcePosition(t, callerSource, "callee.Work")
	plan := &Plan{
		SourceModuleRoot: root,
		CutPoint: CutPoint{
			PackagePath: "example.com/app/callee",
			FuncName:    "Work",
		},
		Incoming: IncomingCall{
			File:   callerPath,
			Line:   line,
			Column: column,
		},
	}

	patched, err := PatchCallsite(plan)
	if err != nil {
		t.Fatal(err)
	}
	if patched != callerPath {
		t.Fatalf("patched path = %s, want %s", patched, callerPath)
	}
	gotBytes, err := os.ReadFile(callerPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "callee.Work_monolift(\"ok\")") {
		t.Fatalf("patched source did not call stub:\n%s", got)
	}
	if !strings.Contains(got, "// preserved call comment") {
		t.Fatalf("patched source did not preserve comment:\n%s", got)
	}

	if _, err := PatchCallsite(plan); err != nil {
		t.Fatalf("second patch should be idempotent: %v", err)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func sourcePosition(t *testing.T, source, needle string) (int, int) {
	t.Helper()
	offset := strings.Index(source, needle)
	if offset < 0 {
		t.Fatalf("%q not found in source", needle)
	}
	line, column := 1, 1
	for i := 0; i < offset; i++ {
		if source[i] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return line, column
}
