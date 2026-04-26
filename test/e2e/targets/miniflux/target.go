package miniflux

import (
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "miniflux",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		RequiredDiagnostics: []string{
			"MLV2_NO_ERROR_CHANNEL",
			"MLV2_REFLECTION_DISPATCH",
		},
		SpecTrace: "docs/specs/monolift-v2-contract.md §Cross-target validation: Miniflux; SPRINT-0020 real-compiler regen",
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/fixtures/rss-feed-server.yaml",
			"test/e2e/targets/miniflux/baseline/deployment.yaml",
			"test/e2e/targets/miniflux/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{"test/e2e/fixtures/rss-feed-server.yaml"},
			{
				"test/e2e/targets/miniflux/baseline/deployment.yaml",
				"test/e2e/targets/miniflux/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/miniflux"},
		LiftedHostBuild: &harness.HostBuildSpec{
			Dockerfile:     "lifted/Dockerfile.host",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/miniflux-lifted:e2e",
			ServiceName:    "miniflux-lifted",
			DeploymentYAML: "lifted/manifests/miniflux-lifted-deployment.yaml",
			ServiceYAML:    "lifted/manifests/miniflux-lifted-service.yaml",
		},
		LiftedExtractedServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-extracted-estimatereadingtime",
			Dockerfile:     "lifted/Dockerfile.extracted-estimatereadingtime",
			ContextRoot:    "lifted/host-patch",
			ImageTag:       "monolift-e2e/extracted-estimatereadingtime:e2e",
			DeploymentYAML: "lifted/manifests/extracted-estimatereadingtime-deployment.yaml",
			ServiceYAML:    "lifted/manifests/extracted-estimatereadingtime-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-estimatereadingtime",
			Dockerfile:     "lifted/Dockerfile.oracle-estimatereadingtime",
			ContextRoot:    "lifted/host-patch",
			ImageTag:       "monolift-e2e/oracle-estimatereadingtime:e2e",
			DeploymentYAML: "lifted/manifests/oracle-estimatereadingtime-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-estimatereadingtime-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		GoldenReport: "test/e2e/targets/miniflux/golden/report.json",
		Workload:     Workload{},
		ServiceName:  "miniflux",
		ServicePort:  8080,
	}
}
