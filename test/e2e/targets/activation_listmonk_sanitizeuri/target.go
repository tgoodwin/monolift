package activation_listmonk_sanitizeuri

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-listmonk-sanitizeuri",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_listmonk_sanitizeuri/baseline/deployment.yaml",
			"test/e2e/targets/activation_listmonk_sanitizeuri/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_listmonk_sanitizeuri/baseline/deployment.yaml",
				"test/e2e/targets/activation_listmonk_sanitizeuri/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/listmonk"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "internal/utils/utils.go:41",
			ServiceName:          "monolift-extracted-sanitizeuri",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_SANITIZEURI",
			DirectInvocationProbePayload: map[string]any{
				"u": "https://evil.com/dashboard?x=1",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/listmonk-sanitizeuri-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-sanitizeuri:e2e",
				HostServiceName:      "listmonk-lifted",
				ExtractedServiceName: "monolift-extracted-sanitizeuri",
				HostPort:             9000,
				HostReadinessPath:    "/admin/login",
				HostBuildPackage: "./cmd",
				HostBinaryName:   "listmonk-patched",
				HostBuildCommand: "CGO_ENABLED=0 go build -mod=mod -o /tmp/listmonk-patched ./cmd && go install github.com/knadh/stuffbin/...@latest && stuffbin -a stuff -in /tmp/listmonk-patched -out /out/listmonk-patched /static/public:/static/public /static/email-templates:/static/email-templates /i18n:/i18n /queries.sql /schema.sql",
				HostRuntimeImage: "listmonk/listmonk:latest",
				HostRuntimeSetup: []string{"rm -f /listmonk/listmonk"},
				HostArgs:         []string{"sh", "-c", "cd /listmonk && /listmonk-patched --install --idempotent --yes --config '' && /listmonk-patched --upgrade --yes --config '' && /listmonk-patched --config ''"},
				HostEnvVars: []codegen.EnvVar{
					{Name: "LISTMONK_app__address", Value: "0.0.0.0:9000"},
					{Name: "LISTMONK_db__host", Value: "postgres"},
					{Name: "LISTMONK_db__port", Value: "5432"},
					{Name: "LISTMONK_db__user", Value: "miniflux"},
					{Name: "LISTMONK_db__password", Value: "miniflux"},
					{Name: "LISTMONK_db__database", Value: "miniflux"},
					{Name: "LISTMONK_db__ssl_mode", Value: "disable"},
					{Name: "LISTMONK_app__admin_username", Value: "admin"},
					{Name: "LISTMONK_app__admin_password", Value: "admin"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-sanitizeuri": "sanitizeuri",
			"monolift-oracle-sanitizeuri":    "sanitizeuri",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-sanitizeuri",
			Dockerfile:     "lifted/Dockerfile.oracle-sanitizeuri",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-sanitizeuri:e2e",
			DeploymentYAML: "lifted/manifests/oracle-sanitizeuri-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-sanitizeuri-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"sanitizeuri": {
				"u": "https://evil.com/dashboard?x=1",
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: loginPath, Status: true, Body: true}},
		ServiceName: "listmonk",
		ServicePort: 9000,
	}
}
