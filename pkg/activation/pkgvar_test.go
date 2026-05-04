package activation

import (
	"path/filepath"
	"testing"
)

func TestPackageVarFunctionFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/pkgvar/direct")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if err := AugmentPackageVars(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "target", PackageVarFuncValue)
}

func TestPackageVarInterfaceFixture(t *testing.T) {
	program := loadFixtureProgram(t, "pkg/activation/testdata/pkgvar/interface")
	graph := newTestGraph(findFunctionByName(t, program, "dispatch"))

	if err := AugmentPackageVars(graph, program); err != nil {
		t.Fatal(err)
	}

	assertEdge(t, graph, "dispatch", "Run", PackageVarFuncValue)
}

func TestPackageVarFixtureEntrypointsLoad(t *testing.T) {
	for _, name := range []string{"direct", "interface"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/pkgvar", name)
			cfg := Config{Dir: dir, Packages: []string{"."}}
			program, err := cfg.LoadProgram()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := cfg.FindEntrypoints(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
