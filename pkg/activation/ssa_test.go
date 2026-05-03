package activation

import (
	"path/filepath"
	"testing"
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
