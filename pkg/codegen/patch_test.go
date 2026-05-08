package codegen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchCutFunctionRenamesDeclaration(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.23\n")
	calleePath := filepath.Join(root, "callee", "callee.go")
	writeTestFile(t, calleePath, `package callee

func Work(value string) string {
	return value
}
`)
	stubPath := filepath.Join(root, "callee", "monolift_lift_WORK.go")
	stubContent := []byte(`package callee

import "os"

func Work(value string) string {
	if os.Getenv("MONOLIFT_LIFT_WORK") != "on" {
		return monoliftOriginalWork(value)
	}
	return monoliftOriginalWork(value)
}
`)
	callerPath := filepath.Join(root, "caller", "caller.go")
	writeTestFile(t, callerPath, `package caller

import "example.com/app/callee"

func Run() string {
	return callee.Work("ok")
}
`)
	plan := &Plan{
		SourceModuleRoot: root,
		ClientPath:       stubPath,
		CutPoint: CutPoint{
			PackagePath: "example.com/app/callee",
			FuncName:    "Work",
			File:        calleePath,
		},
		Incoming: IncomingCall{
			File: callerPath,
			Line: 6,
		},
	}

	patched, err := PatchCutFunction(plan, stubContent)
	if err != nil {
		t.Fatal(err)
	}
	if patched != calleePath {
		t.Fatalf("patched path = %s, want %s", patched, calleePath)
	}
	gotBytes, err := os.ReadFile(calleePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "func monoliftOriginalWork(value string) string") {
		t.Fatalf("expected renamed function declaration:\n%s", got)
	}
	if strings.Contains(got, "func Work(") {
		t.Fatalf("original function name should be renamed:\n%s", got)
	}

	stubBytes, err := os.ReadFile(stubPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stubBytes), "func Work(") {
		t.Fatalf("stub should declare the original function name:\n%s", string(stubBytes))
	}

	// Idempotent
	if _, err := PatchCutFunction(plan, stubContent); err != nil {
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
