package codegen

import (
	"path/filepath"
	"strings"
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

func TestApplyLiftOptionsDeployDefaults(t *testing.T) {
	root := t.TempDir()
	plan := &Plan{
		SourceModuleRoot: root,
		OutputDir:        filepath.Join(root, "monolift_gen", "sanitizehtml"),
		ServiceName:      "monolift-extracted-sanitizehtml",
		EnvServiceName:   envServiceName("monolift-extracted-sanitizehtml"),
		CutPoint:         CutPoint{PackageDir: filepath.Join(root, "internal", "reader", "sanitizer")},
	}
	applyLiftOptions(plan, LiftOptions{})
	if plan.EnvServiceName != "SANITIZEHTML" {
		t.Fatalf("env service name = %s, want SANITIZEHTML", plan.EnvServiceName)
	}
	if plan.Deploy.ExtractedPort != 8081 || plan.Deploy.ImagePullPolicy != "IfNotPresent" {
		t.Fatalf("deploy defaults = %+v", plan.Deploy)
	}
	if plan.Deploy.HostPort != 8080 || plan.Deploy.HostReadinessPath != "/healthz" {
		t.Fatalf("host defaults = %+v", plan.Deploy)
	}
	wantDockerfile := filepath.Join(plan.OutputDir, "Dockerfile.extracted-monolift-extracted-sanitizehtml")
	if plan.ExtractedDockerfilePath != wantDockerfile {
		t.Fatalf("extracted dockerfile = %s, want %s", plan.ExtractedDockerfilePath, wantDockerfile)
	}
	wantManifest := filepath.Join(plan.OutputDir, "manifests", "monolift-extracted-sanitizehtml-deployment.yaml")
	if plan.ExtractedDeploymentPath != wantManifest {
		t.Fatalf("extracted deployment = %s, want %s", plan.ExtractedDeploymentPath, wantManifest)
	}
}

func TestApplyLiftOptionsDeployOverrides(t *testing.T) {
	root := t.TempDir()
	plan := &Plan{
		SourceModuleRoot: root,
		OutputDir:        filepath.Join(root, "gen"),
		ServiceName:      "sanitizehtml",
		EnvServiceName:   envServiceName("sanitizehtml"),
		CutPoint:         CutPoint{PackageDir: filepath.Join(root, "pkg")},
	}
	applyLiftOptions(plan, LiftOptions{
		Deploy: DeployOptions{
			HostImage:            "registry/host:v1",
			ExtractedImage:       "registry/extracted:v1",
			HostServiceName:      "miniflux-host",
			ExtractedServiceName: "monolift-extracted-sanitizehtml",
			HostPort:             9090,
			ExtractedPort:        9091,
			HostReadinessPath:    "/ready",
			HostBuildPackage:     "./cmd/miniflux",
			HostBinaryName:       "miniflux",
			HostEnvVars:          []EnvVar{{Name: "DATABASE_URL", Value: "postgres://db"}},
			ImagePullPolicy:      "Never",
		},
	})
	if plan.Deploy.HostImage != "registry/host:v1" || plan.Deploy.ExtractedImage != "registry/extracted:v1" {
		t.Fatalf("image overrides not preserved: %+v", plan.Deploy)
	}
	if plan.Deploy.HostPort != 9090 || plan.Deploy.ExtractedPort != 9091 || plan.Deploy.ImagePullPolicy != "Never" {
		t.Fatalf("port/policy overrides not preserved: %+v", plan.Deploy)
	}
	if len(plan.Deploy.HostEnvVars) != 1 || plan.Deploy.HostEnvVars[0].Name != "DATABASE_URL" {
		t.Fatalf("host env overrides not preserved: %+v", plan.Deploy.HostEnvVars)
	}
}

func TestAdmitPlanValidatesDeployNamesAndPaths(t *testing.T) {
	root := t.TempDir()
	plan := &Plan{
		SourceModuleRoot: root,
		ServiceName:      "sanitizehtml",
		Deploy: DeployOptions{
			HostServiceName:      "invalid_name",
			ExtractedServiceName: strings.Repeat("a", 64),
		},
		HostDockerfilePath:      filepath.Join(root, "Dockerfile.host-invalid_name"),
		ExtractedDockerfilePath: filepath.Join(root, "..", "outside", "Dockerfile.extracted"),
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("AdmitPlan accepted invalid deploy plan")
	}
	var sawDNS, sawPath bool
	for _, refusal := range verdict.Refusals {
		sawDNS = sawDNS || refusal.Code == "invalid_kubernetes_name"
		sawPath = sawPath || refusal.Code == "generated_path_outside_module"
	}
	if !sawDNS || !sawPath {
		t.Fatalf("missing DNS/path refusals: %+v", verdict.Refusals)
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
