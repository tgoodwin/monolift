package activation_mattermost_pbkdf2hash

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-mattermost-pbkdf2hash",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     7,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_mattermost_pbkdf2hash/baseline/deployment.yaml",
			"test/e2e/targets/activation_mattermost_pbkdf2hash/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_mattermost_pbkdf2hash/baseline/deployment.yaml",
				"test/e2e/targets/activation_mattermost_pbkdf2hash/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 8 * time.Minute,
		LiftedReadyTimeout:   10 * time.Minute,
		SourceDirs:           []string{"evaluation/mattermost/server"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "channels/app/password/hashers/pbkdf2.go:151",
			ServiceName:          "monolift-extracted-pbkdf2hash",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_PBKDF2HASH",
			DirectInvocationProbePayload: map[string]any{
				"password": "monolift-test-password",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/mattermost-pbkdf2hash-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-pbkdf2hash:e2e",
				HostServiceName:      "mattermost-lifted",
				ExtractedServiceName: "monolift-extracted-pbkdf2hash",
				HostPort:             8065,
				HostReadinessPath:    "/api/v4/system/ping",
				HostBuildPackage:     "./cmd/mattermost",
				HostBinaryName:       "mattermost",
				HostAssetCopies: []codegen.AssetCopy{
					{From: "i18n", To: "/i18n"},
					{From: "templates", To: "/templates"},
					{From: "fonts", To: "/fonts"},
					{From: "config", To: "/config"},
				},
				HostEnvVars: []codegen.EnvVar{
					{Name: "MM_SQLSETTINGS_DATASOURCE", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
					{Name: "MM_SQLSETTINGS_DRIVERNAME", Value: "postgres"},
					{Name: "MM_SERVICESETTINGS_LISTENADDRESS", Value: ":8065"},
					{Name: "MM_SERVICESETTINGS_SITEURL", Value: "http://localhost:8065"},
					{Name: "MM_TEAMSETTINGS_ENABLEOPENSERVER", Value: "true"},
				},
			},
			GoWorkModules: []string{".", "./public"},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-pbkdf2hash": "hash",
		},
		InvokePayloads: map[string]map[string]any{
			"hash": {
				"password": "monolift-test-password",
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: loginPath, Status: true}},
		ServiceName: "mattermost",
		ServicePort: 8065,
	}
}
