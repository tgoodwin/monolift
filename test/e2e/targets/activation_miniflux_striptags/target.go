package activation_miniflux_striptags

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-striptags",
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
			Target:               "internal/reader/sanitizer/strip_tags.go:15",
			ServiceName:          "monolift-extracted-striptags",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_STRIPTAGS",
			DirectInvocationProbePayload: map[string]any{
				"input": directInvocationInput,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-striptags-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-striptags:e2e",
				HostServiceName:      "miniflux-lifted",
				ExtractedServiceName: "monolift-extracted-striptags",
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
			"monolift-extracted-striptags": "striptags",
			"monolift-oracle-striptags":    "striptags",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-striptags",
			Dockerfile:     "lifted/Dockerfile.oracle-striptags",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-striptags:e2e",
			DeploymentYAML: "lifted/manifests/oracle-striptags-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-striptags-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"striptags": {
				"input": directInvocationInput,
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: importPath, Status: true, Body: true}},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
