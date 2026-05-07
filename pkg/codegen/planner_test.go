package codegen

import (
	"path/filepath"
	"testing"
)

func TestBuildPlanSanitizeHTMLFixture(t *testing.T) {
	fixture := SanitizeHTMLFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CutPoint.FuncName != "SanitizeHTML" {
		t.Fatalf("func = %s", plan.CutPoint.FuncName)
	}
	if len(plan.BoundaryParams) != 3 {
		t.Fatalf("boundary param count = %d, want 3", len(plan.BoundaryParams))
	}
	assertParam(t, plan.BoundaryParams[0], "baseURL", "base_url", "string")
	assertParam(t, plan.BoundaryParams[1], "rawHTML", "input", "string")
	assertParam(t, plan.BoundaryParams[2], "sanitizerOptions", "sanitizer_options", "*SanitizerOptions")
	if len(plan.ReconstructedParams) != 0 {
		t.Fatalf("reconstructed params = %d, want 0", len(plan.ReconstructedParams))
	}
	if plan.ReturnCodec.GoType != "string" {
		t.Fatalf("return type = %s", plan.ReturnCodec.GoType)
	}
}

func TestBuildPlanRefreshFeedFixture(t *testing.T) {
	fixture := RefreshFeedFixture(repoRoot(t))
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	if plan.CutPoint.FuncName != "RefreshFeed" {
		t.Fatalf("func = %s", plan.CutPoint.FuncName)
	}
	if len(plan.BoundaryParams) != 3 {
		t.Fatalf("boundary param count = %d, want 3", len(plan.BoundaryParams))
	}
	if len(plan.ReconstructedParams) != 1 {
		t.Fatalf("reconstructed params = %d, want 1", len(plan.ReconstructedParams))
	}
	assertParam(t, plan.ReconstructedParams[0].Param, "store", "store", "*storage.Storage")
	assertParam(t, plan.BoundaryParams[0], "userID", "user_id", "int64")
	assertParam(t, plan.BoundaryParams[1], "feedID", "feed_id", "int64")
	assertParam(t, plan.BoundaryParams[2], "forceRefresh", "force_refresh", "bool")
	if plan.ReturnCodec.Kind != CodecLocalizedErrorWrapper {
		t.Fatalf("return codec = %s", plan.ReturnCodec.Kind)
	}
	if plan.ReconstructedParams[0].Reconstructor.ID != "sql_db_wrapper" {
		t.Fatalf("reconstructor = %s", plan.ReconstructedParams[0].Reconstructor.ID)
	}
}

func assertParam(t *testing.T, got Param, name, jsonName, goType string) {
	t.Helper()
	if got.Name != name || got.JSONName != jsonName || got.GoType != goType {
		t.Fatalf("param = (%s, %s, %s), want (%s, %s, %s)", got.Name, got.JSONName, got.GoType, name, jsonName, goType)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
