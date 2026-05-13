package activation

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestBuildSSAFixture(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/simple")
	cfg := Config{Dir: dir, Packages: []string{"."}}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	program.BuildSSA()
	if program.SSAProgram == nil {
		t.Fatal("SSAProgram is nil")
	}
	if len(program.SSAPackages) != 1 {
		t.Fatalf("len(SSAPackages) = %d, want 1", len(program.SSAPackages))
	}
	if program.SSAPackages[0].Func("main") == nil {
		t.Fatal("main function not built")
	}
}

func TestProgramFunctionsCachedDeterministicOrder(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/simple")

	first := program.Functions()
	second := program.Functions()
	if len(first) == 0 {
		t.Fatal("Functions returned no SSA functions")
	}
	if len(first) != len(second) {
		t.Fatalf("second Functions length = %d, want %d", len(second), len(first))
	}
	if &first[0] != &second[0] {
		t.Fatal("Functions did not reuse cached slice backing array")
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("function %d changed from %s to %s", i, first[i], second[i])
		}
		if i > 0 && !functionOrderLessOrEqual(first[i-1], first[i]) {
			t.Fatalf("functions are not sorted at %d: %s > %s", i, FunctionKeyForSSA(first[i-1]), FunctionKeyForSSA(first[i]))
		}
	}
}

func functionOrderLessOrEqual(a, b *ssa.Function) bool {
	ak := FunctionKeyForSSA(a)
	bk := FunctionKeyForSSA(b)
	if ak.PackagePath != bk.PackagePath {
		return ak.PackagePath < bk.PackagePath
	}
	if ak.Receiver != bk.Receiver {
		return ak.Receiver < bk.Receiver
	}
	if ak.FuncName != bk.FuncName {
		return ak.FuncName < bk.FuncName
	}
	return a.String() <= b.String()
}
