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

// 2H.3: plan with context.Context param produces server-side context.Background()
func TestReconstructorContextBackground(t *testing.T) {
	imp := importer.Default()
	pkg, err := imp.Import("context")
	if err != nil {
		t.Fatal(err)
	}
	obj := pkg.Scope().Lookup("Context")
	if obj == nil {
		t.Fatal("context.Context not found")
	}
	recon, ok := LookupReconstructor(obj.Type())
	if !ok {
		t.Fatal("LookupReconstructor(context.Context) = false")
	}
	if recon.ID != "context_background" {
		t.Fatalf("reconstructor ID = %s, want context_background", recon.ID)
	}
	// Verify server init produces context.Background()
	param := ReconstructedParam{
		Param:         Param{Name: "ctx"},
		Reconstructor: recon,
	}
	lines := serverReconstructorInit(param)
	if len(lines) != 1 || lines[0] != "state.Ctx = context.Background()" {
		t.Fatalf("init lines = %v, want [state.Ctx = context.Background()]", lines)
	}
}

// 2H.4: plan with mlog.LoggerIFace param produces discard logger
func TestReconstructorDiscardLogger(t *testing.T) {
	// Construct a synthetic named interface type with "Logger" in the name,
	// simulating mlog.LoggerIFace without requiring the mattermost dependency.
	pkg := types.NewPackage("github.com/mattermost/mattermost/server/public/shared/mlog", "mlog")
	iface := types.NewInterfaceType(nil, nil)
	iface.Complete()
	tn := types.NewTypeName(0, pkg, "LoggerIFace", nil)
	named := types.NewNamed(tn, iface, nil)

	recon, ok := LookupReconstructor(named)
	if !ok {
		t.Fatal("LookupReconstructor(mlog.LoggerIFace) = false")
	}
	if recon.ID != "discard_logger" {
		t.Fatalf("reconstructor ID = %s, want discard_logger", recon.ID)
	}
	// Verify server init produces nil assignment (discard logger)
	param := ReconstructedParam{
		Param:         Param{Name: "logger"},
		Reconstructor: recon,
	}
	lines := serverReconstructorInit(param)
	if len(lines) != 1 || lines[0] != "state.Logger = nil" {
		t.Fatalf("init lines = %v, want [state.Logger = nil]", lines)
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
	if recon.ConstructorPkg != "storage" {
		t.Fatalf("constructor pkg = %s, want storage", recon.ConstructorPkg)
	}
	if recon.ConstructorFunc != "NewStorage" {
		t.Fatalf("constructor func = %s, want NewStorage", recon.ConstructorFunc)
	}
	if len(recon.ConstructorArgOrder) != 1 || recon.ConstructorArgOrder[0] != "db" {
		t.Fatalf("constructor arg order = %v, want [db]", recon.ConstructorArgOrder)
	}
}
