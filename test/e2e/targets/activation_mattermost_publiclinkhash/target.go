package activation_mattermost_publiclinkhash

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-mattermost-publiclinkhash",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_mattermost_publiclinkhash/baseline/deployment.yaml",
			"test/e2e/targets/activation_mattermost_publiclinkhash/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_mattermost_publiclinkhash/baseline/deployment.yaml",
				"test/e2e/targets/activation_mattermost_publiclinkhash/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 8 * time.Minute,
		LiftedReadyTimeout:   10 * time.Minute,
		SourceDirs:           []string{"evaluation/mattermost/server"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "channels/app/file.go:588",
			ServiceName:          "monolift-extracted-publiclinkhash",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_GENERATEPUBLICHASH",
			DirectInvocationProbePayload: map[string]any{
				"file_id": "test-file-001",
				"salt":    "monolift-test-salt",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/mattermost-publiclinkhash-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-publiclinkhash:e2e",
				HostServiceName:      "mattermost-lifted",
				ExtractedServiceName: "monolift-extracted-publiclinkhash",
				HostPort:             8065,
				HostReadinessPath:    "/api/v4/system/ping",
				HostBuildPackage:     "./cmd/mattermost",
				HostBinaryName:       "mattermost",
				HostEnvVars: []codegen.EnvVar{
					{Name: "MM_SQLSETTINGS_DATASOURCE", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
					{Name: "MM_SQLSETTINGS_DRIVERNAME", Value: "postgres"},
					{Name: "MM_SERVICESETTINGS_LISTENADDRESS", Value: ":8065"},
					{Name: "MM_SERVICESETTINGS_SITEURL", Value: "http://localhost:8065"},
					{Name: "MM_TEAMSETTINGS_ENABLEOPENSERVER", Value: "true"},
					{Name: "MM_FILESETTINGS_PUBLICLINKSALT", Value: "monolift-test-salt"},
				},
			},
			GoWorkModules: []string{".", "./public"},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-publiclinkhash": "generatepublichash",
			"monolift-oracle-publiclinkhash":    "generatepublichash",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-publiclinkhash",
			Dockerfile:     "lifted/Dockerfile.oracle-publiclinkhash",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-publiclinkhash:e2e",
			DeploymentYAML: "lifted/manifests/oracle-publiclinkhash-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-publiclinkhash-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"generatepublichash": {
				"file_id": "test-file-001",
				"salt":    "monolift-test-salt",
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: pingPath, Status: true, Body: true}},
		ServiceName: "mattermost",
		ServicePort: 8065,
	}
}
