package codegen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDockerfilesSanitizeHTMLGolden(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	goldens := map[string][]byte{
		filepath.Join("testdata", "sanitizehtml_dockerfile_extracted.golden"): files[plan.ExtractedDockerfilePath],
		filepath.Join("testdata", "sanitizehtml_dockerfile_host.golden"):      files[plan.HostDockerfilePath],
	}
	for goldenPath, got := range goldens {
		if os.Getenv("MONOLIFT_UPDATE_GOLDEN") == "1" {
			if err := os.WriteFile(goldenPath, got, 0644); err != nil {
				t.Fatal(err)
			}
		}
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("rendered Dockerfile does not match %s", goldenPath)
		}
	}
}

func sanitizeHTMLDeployPlan(t *testing.T) *Plan {
	t.Helper()
	root := repoRoot(t)
	fixture := SanitizeHTMLFixture(root)
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	applyLiftOptions(plan, LiftOptions{
		Output:      filepath.Join(plan.SourceModuleRoot, ".monolift-sanitizehtml"),
		ServiceName: "sanitizehtml",
		Deploy: DeployOptions{
			HostImage:            "monolift-e2e/miniflux-host:e2e",
			ExtractedImage:       "monolift-e2e/extracted-sanitizehtml:e2e",
			HostServiceName:      "miniflux",
			ExtractedServiceName: "monolift-extracted-sanitizehtml",
			HostPort:             8080,
			HostReadinessPath:    "/healthcheck",
			HostBuildPackage:     ".",
			HostBinaryName:       "miniflux",
			HostEnvVars: []EnvVar{
				{Name: "DATABASE_URL", Value: "postgres://miniflux@postgres/miniflux?sslmode=disable"},
				{Name: "RUN_MIGRATIONS", Value: "1"},
			},
		},
	})
	return plan
}

func TestRenderDockerfilesIncludesExpectedDirectives(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)
	files, err := RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	extracted := string(files[plan.ExtractedDockerfilePath])
	for _, want := range []string{
		"FROM golang:1.26.0 AS builder",
		"go build -mod=mod -o /out/sanitizehtml ./cmd/sanitizehtml",
		"EXPOSE 8081",
		`ENTRYPOINT ["/sanitizehtml"]`,
	} {
		if !strings.Contains(extracted, want) {
			t.Fatalf("extracted Dockerfile missing %q:\n%s", want, extracted)
		}
	}
	host := string(files[plan.HostDockerfilePath])
	for _, want := range []string{
		"FROM golang:1.26.0 AS builder",
		"go build -mod=mod -o /out/miniflux .",
		`ENV DATABASE_URL="postgres://miniflux@postgres/miniflux?sslmode=disable"`,
		"EXPOSE 8080",
		`ENTRYPOINT ["/miniflux"]`,
	} {
		if !strings.Contains(host, want) {
			t.Fatalf("host Dockerfile missing %q:\n%s", want, host)
		}
	}

	plan.Deploy.HostAssetCopies = []AssetCopy{{From: "static", To: "/srv/static"}}
	files, err = RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	host = string(files[plan.HostDockerfilePath])
	for _, want := range []string{
		"RUN chmod -R a+rX /src/static",
		"COPY --from=builder /src/static /srv/static",
	} {
		if !strings.Contains(host, want) {
			t.Fatalf("host Dockerfile with assets missing %q:\n%s", want, host)
		}
	}
}

func TestRenderDockerfilesHostBuildCommand(t *testing.T) {
	plan := sanitizeHTMLDeployPlan(t)

	host, err := RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	defaultHost := string(host[plan.HostDockerfilePath])
	if !strings.Contains(defaultHost, "go build -mod=mod") {
		t.Fatalf("default host Dockerfile should use go build:\n%s", defaultHost)
	}

	plan.Deploy.HostBuildCommand = "make build && stuffbin -a stuff -o /out/myapp ./myapp"
	host, err = RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	customHost := string(host[plan.HostDockerfilePath])
	if strings.Contains(customHost, "go build -mod=mod") {
		t.Fatalf("custom build command should replace go build:\n%s", customHost)
	}
	if !strings.Contains(customHost, "make build && stuffbin -a stuff -o /out/myapp ./myapp") {
		t.Fatalf("custom build command not found in Dockerfile:\n%s", customHost)
	}
}
