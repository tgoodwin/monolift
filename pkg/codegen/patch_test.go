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

func TestPatchCutFunctionRenamesMethodDeclaration(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.23\n")
	calleePath := filepath.Join(root, "callee", "callee.go")
	writeTestFile(t, calleePath, `package callee

type Argon2Hasher struct {
	Memory uint32
}

func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return append(password, salt...)
}
`)
	stubPath := filepath.Join(root, "callee", "monolift_lift_HASHWITHSALTBYTES.go")
	stubContent := []byte(`package callee

func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return h.monoliftOriginalHashWithSaltBytes(password, salt)
}
`)
	plan := &Plan{
		SourceModuleRoot: root,
		ClientPath:       stubPath,
		CutPoint: CutPoint{
			PackagePath: "example.com/app/callee",
			FuncName:    "HashWithSaltBytes",
			Receiver:    "*Argon2Hasher",
			File:        calleePath,
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
	if !strings.Contains(got, "func (h *Argon2Hasher) monoliftOriginalHashWithSaltBytes(") {
		t.Fatalf("expected renamed method declaration with *Argon2Hasher receiver:\n%s", got)
	}
	if strings.Contains(got, "func (h *Argon2Hasher) HashWithSaltBytes(") {
		t.Fatalf("original method name should be renamed:\n%s", got)
	}

	// Idempotent
	if _, err := PatchCutFunction(plan, stubContent); err != nil {
		t.Fatalf("second patch should be idempotent: %v", err)
	}
}

func TestPatchCutFunctionRenamesOnlyMatchingReceiverMethod(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.23\n")
	calleePath := filepath.Join(root, "callee", "callee.go")
	writeTestFile(t, calleePath, `package callee

type Argon2Hasher struct {
	Memory uint32
}

// Standalone function with the same name.
func HashWithSaltBytes(data []byte) []byte {
	return data
}

// Method on *Argon2Hasher — this one should be renamed.
func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return append(password, salt...)
}
`)
	stubPath := filepath.Join(root, "callee", "monolift_lift_HASHWITHSALTBYTES.go")
	stubContent := []byte(`package callee

func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return h.monoliftOriginalHashWithSaltBytes(password, salt)
}
`)
	plan := &Plan{
		SourceModuleRoot: root,
		ClientPath:       stubPath,
		CutPoint: CutPoint{
			PackagePath: "example.com/app/callee",
			FuncName:    "HashWithSaltBytes",
			Receiver:    "*Argon2Hasher",
			File:        calleePath,
		},
	}

	_, err := PatchCutFunction(plan, stubContent)
	if err != nil {
		t.Fatal(err)
	}
	gotBytes, err := os.ReadFile(calleePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)

	// The method should be renamed.
	if !strings.Contains(got, "func (h *Argon2Hasher) monoliftOriginalHashWithSaltBytes(") {
		t.Fatalf("expected method to be renamed:\n%s", got)
	}
	// The standalone function should remain unchanged.
	if !strings.Contains(got, "func HashWithSaltBytes(data []byte) []byte") {
		t.Fatalf("standalone function should not be renamed:\n%s", got)
	}
}

func TestPatchCutFunctionCollisionRefusesWithDiagnostic(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n\ngo 1.23\n")
	calleePath := filepath.Join(root, "callee", "callee.go")
	writeTestFile(t, calleePath, `package callee

type Argon2Hasher struct {
	Memory uint32
}

func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return append(password, salt...)
}

// Pre-existing method with the collision name.
func (h *Argon2Hasher) monoliftOriginalHashWithSaltBytes(password []byte, salt []byte) []byte {
	return salt
}
`)
	stubPath := filepath.Join(root, "callee", "monolift_lift_HASHWITHSALTBYTES.go")
	stubContent := []byte(`package callee

func (h *Argon2Hasher) HashWithSaltBytes(password []byte, salt []byte) []byte {
	return h.monoliftOriginalHashWithSaltBytes(password, salt)
}
`)
	plan := &Plan{
		SourceModuleRoot: root,
		ClientPath:       stubPath,
		CutPoint: CutPoint{
			PackagePath: "example.com/app/callee",
			FuncName:    "HashWithSaltBytes",
			Receiver:    "*Argon2Hasher",
			File:        calleePath,
		},
	}

	_, err := PatchCutFunction(plan, stubContent)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Fatalf("expected collision diagnostic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "monoliftOriginalHashWithSaltBytes") {
		t.Fatalf("expected collision name in diagnostic, got: %v", err)
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
