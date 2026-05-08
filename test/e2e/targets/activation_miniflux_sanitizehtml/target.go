package activation_miniflux_sanitizehtml

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-sanitizehtml",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/fixtures/rss-feed-server.yaml",
			"test/e2e/targets/activation_miniflux_sanitizehtml/baseline/deployment.yaml",
			"test/e2e/targets/activation_miniflux_sanitizehtml/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{"test/e2e/fixtures/rss-feed-server.yaml"},
			{
				"test/e2e/targets/activation_miniflux_sanitizehtml/baseline/deployment.yaml",
				"test/e2e/targets/activation_miniflux_sanitizehtml/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/miniflux"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "internal/reader/sanitizer/sanitizer.go:217",
			ServiceName:          "monolift-extracted-sanitizehtml",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_SANITIZEHTML",
			DirectInvocationProbePayload: map[string]any{
				"base_url":          "https://example.org/base/",
				"input":             `<p>Hello</p><script>alert(1)</script><a href="/next">next</a>`,
				"sanitizer_options": map[string]any{"OpenLinksInNewTab": true},
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-sanitizehtml-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-sanitizehtml:e2e",
				HostServiceName:      "miniflux-lifted",
				ExtractedServiceName: "monolift-extracted-sanitizehtml",
				HostPort:             8080,
				HostReadinessPath:    "/healthcheck",
				HostBuildPackage:     ".",
				HostBinaryName:       "miniflux",
				HostEnvVars: []codegen.EnvVar{
					{Name: "DATABASE_URL", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
					{Name: "RUN_MIGRATIONS", Value: "1"},
					{Name: "CREATE_ADMIN", Value: "1"},
					{Name: "ADMIN_USERNAME", Value: "admin"},
					{Name: "ADMIN_PASSWORD", Value: "test123"},
					{Name: "LISTEN_ADDR", Value: "0.0.0.0:8080"},
					{Name: "FETCHER_ALLOW_PRIVATE_NETWORKS", Value: "1"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-sanitizehtml": "sanitizehtml",
			"monolift-oracle-sanitizehtml":    "sanitizehtml",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-sanitizehtml",
			Dockerfile:     "lifted/Dockerfile.oracle-sanitizehtml",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-sanitizehtml:e2e",
			DeploymentYAML: "lifted/manifests/oracle-sanitizehtml-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-sanitizehtml-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"sanitizehtml": {
				"base_url":          "https://example.org/base/",
				"input":             `<p>Hello</p><script>alert(1)</script><a href="/next">next</a>`,
				"sanitizer_options": map[string]any{"OpenLinksInNewTab": true},
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: importPath, Status: true, Body: true}},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
