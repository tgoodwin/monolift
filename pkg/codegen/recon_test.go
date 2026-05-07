package codegen

import (
	"go/importer"
	"go/types"
	"testing"
)

func TestReconstructionRegistryDirectTypes(t *testing.T) {
	cases := []struct {
		pkgPath string
		name    string
		wantID  string
	}{
		{"database/sql", "DB", "sql_db"},
		{"net/http", "Client", "http_client"},
		{"log", "Logger", "logger"},
	}
	imp := importer.Default()
	for _, tc := range cases {
		pkg, err := imp.Import(tc.pkgPath)
		if err != nil {
			t.Fatal(err)
		}
		obj := pkg.Scope().Lookup(tc.name)
		if obj == nil {
			t.Fatalf("%s.%s not found", tc.pkgPath, tc.name)
		}
		recon, ok := LookupReconstructor(types.NewPointer(obj.Type()))
		if !ok {
			t.Fatalf("LookupReconstructor(*%s.%s) = false", tc.pkgPath, tc.name)
		}
		if recon.ID != tc.wantID {
			t.Fatalf("reconstructor ID = %s, want %s", recon.ID, tc.wantID)
		}
	}
}

func TestReconstructionRegistrySQLWrapper(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.ReconstructedParams) != 1 {
		t.Fatalf("reconstructed params = %d", len(plan.ReconstructedParams))
	}
	recon := plan.ReconstructedParams[0].Reconstructor
	if recon.ID != "sql_db_wrapper" {
		t.Fatalf("reconstructor = %s", recon.ID)
	}
	if recon.ConstructorName != "NewStorage" {
		t.Fatalf("constructor = %s", recon.ConstructorName)
	}
}
