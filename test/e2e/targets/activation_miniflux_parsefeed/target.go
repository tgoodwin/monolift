package activation_miniflux_parsefeed

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

// Minimal RSS 2.0 feed as base64 for direct invocation probe.
const rssBase64 = "PD94bWwgdmVyc2lvbj0iMS4wIj8+PHJzcyB2ZXJzaW9uPSIyLjAiPjxjaGFubmVsPjx0aXRsZT5UZXN0PC90aXRsZT48bGluaz5odHRwczovL2V4YW1wbGUub3JnLzwvbGluaz48aXRlbT48dGl0bGU+SGVsbG88L3RpdGxlPjxsaW5rPmh0dHBzOi8vZXhhbXBsZS5vcmcvaGVsbG88L2xpbms+PC9pdGVtPjwvY2hhbm5lbD48L3Jzcz4="

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-parsefeed",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     7,
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
			Target:               "internal/reader/parser/parser.go:20",
			ServiceName:          "monolift-extracted-parsefeed",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_PARSEFEED",
			DirectInvocationProbePayload: map[string]any{
				"base_url": "https://example.org/",
				"r":        rssBase64,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-parsefeed-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-parsefeed:e2e",
				HostServiceName:      "miniflux-lifted",
				ExtractedServiceName: "monolift-extracted-parsefeed",
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
			"monolift-extracted-parsefeed": "parsefeed",
		},
		InvokePayloads: map[string]map[string]any{
			"parsefeed": {
				"base_url": "https://example.org/",
				"r":        rssBase64,
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: refreshPath, Status: true}},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
