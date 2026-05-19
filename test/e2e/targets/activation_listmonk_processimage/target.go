package activation_listmonk_processimage

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	payload := directInvokePayload()
	return harness.TargetCase{
		Name:            "activation-listmonk-processimage",
		ExpectedVerdict: "refuse-blocking",
		ExpectedRoot:    "processImage",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_listmonk_processimage/baseline/deployment.yaml",
			"test/e2e/targets/activation_listmonk_processimage/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_listmonk_processimage/baseline/deployment.yaml",
				"test/e2e/targets/activation_listmonk_processimage/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/listmonk"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:                       "cmd/media.go:212",
			ServiceName:                  "monolift-extracted-processimage",
			ExpectedEnvVarPrefix:         "MONOLIFT_LIFT_PROCESSIMAGE",
			DirectInvocationProbePayload: payload,
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/listmonk-processimage-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-processimage:e2e",
				HostServiceName:      "listmonk-lifted",
				ExtractedServiceName: "monolift-extracted-processimage",
				HostPort:             9000,
				HostReadinessPath:    "/admin/login",
				HostBuildPackage:     "./cmd",
				HostBinaryName:       "listmonk-patched",
				HostBuildCommand:     "mkdir -p /out && CGO_ENABLED=0 go build -mod=mod -o /tmp/listmonk-patched ./cmd && go install github.com/knadh/stuffbin/...@v1.3.0 && stuffbin -a stuff -in /tmp/listmonk-patched -out /out/listmonk-patched config.toml.sample schema.sql queries:/queries permissions.json static/public:/public static/email-templates i18n:/i18n",
				HostRuntimeImage:     "listmonk/listmonk:latest",
				HostRuntimeSetup:     []string{"rm -f /listmonk/listmonk"},
				HostArgs:             []string{"sh", "-c", "cd /listmonk && mkdir -p /listmonk/uploads && /listmonk-patched --install --idempotent --yes --config '' && /listmonk-patched --upgrade --yes --config '' && /listmonk-patched --config ''"},
				HostEnvVars: []codegen.EnvVar{
					{Name: "LISTMONK_app__address", Value: "0.0.0.0:9000"},
					{Name: "LISTMONK_db__host", Value: "postgres"},
					{Name: "LISTMONK_db__port", Value: "5432"},
					{Name: "LISTMONK_db__user", Value: "miniflux"},
					{Name: "LISTMONK_db__password", Value: "miniflux"},
					{Name: "LISTMONK_db__database", Value: "miniflux"},
					{Name: "LISTMONK_db__ssl_mode", Value: "disable"},
					{Name: "LISTMONK_app__admin_username", Value: "admin"},
					{Name: "LISTMONK_app__admin_password", Value: "adminpass123"},
					{Name: "LISTMONK_upload__extensions", Value: "png,jpg,jpeg,gif,svg,*"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-processimage": "processimage",
		},
		InvokePayloads: map[string]map[string]any{
			"processimage": payload,
		},
		DirectInvoke: harness.DirectInvokeCheck{Expectation: harness.DirectInvokeOracleCompare},
		Oracle:       Oracle{},
		BehavioralPredicates: []harness.BehavioralPredicate{{
			Name:        "thumbnail-metadata",
			Description: "The media upload response records original image dimensions and a thumbnail URL.",
		}},
		Workload:    Workload{},
		ServiceName: "listmonk",
		ServicePort: 9000,
	}
}
