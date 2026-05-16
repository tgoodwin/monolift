package activation_miniflux_feedicon

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
	refreshfeed "github.com/tgoodwin/monolift/test/e2e/targets/activation_miniflux_refreshfeed"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-feedicon",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     4,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/fixtures/rss-feed-server.yaml",
			"test/e2e/targets/activation_miniflux_refreshfeed/baseline/deployment.yaml",
			"test/e2e/targets/activation_miniflux_refreshfeed/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{"test/e2e/fixtures/rss-feed-server.yaml"},
			{
				"test/e2e/targets/activation_miniflux_refreshfeed/baseline/deployment.yaml",
				"test/e2e/targets/activation_miniflux_refreshfeed/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/miniflux"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "internal/reader/icon/checker.go:28",
			ServiceName:          "monolift-extracted-feedicon",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_FEEDICON",
			DirectInvocationProbePayload: map[string]any{
				"user_id":       int64(1),
				"feed_id":       int64(1),
				"force_refresh": true,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-feedicon-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-feedicon:e2e",
				HostServiceName:      "miniflux-feedicon-lifted",
				ExtractedServiceName: "monolift-extracted-feedicon",
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
				ExtractedEnvVars: []codegen.EnvVar{
					{Name: "DATABASE_URL", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-feedicon": "feedicon",
		},
		Workload:    refreshfeed.Workload{},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
