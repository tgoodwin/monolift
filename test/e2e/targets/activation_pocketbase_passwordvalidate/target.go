package activation_pocketbase_passwordvalidate

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-pocketbase-passwordvalidate",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/targets/activation_pocketbase_passwordvalidate/baseline/deployment.yaml",
			"test/e2e/targets/activation_pocketbase_passwordvalidate/baseline/service.yaml",
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   8 * time.Minute,
		Dockerfile:           "test/e2e/targets/activation_pocketbase_passwordvalidate/Dockerfile",
		ContextDir:           ".",
		SourceDirs:           []string{"evaluation/pocketbase"},
		ImageTag:             "monolift-e2e/pocketbase:e2e",
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "core/field_password.go:317",
			ServiceName:          "monolift-extracted-passwordvalidate",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_PASSWORDVALIDATE",
			DirectInvocationProbePayload: map[string]any{
				"receiver": map[string]any{
					"Hash": directInvocationHash,
				},
				"pass": directInvocationPass,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/pocketbase-passwordvalidate-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-passwordvalidate:e2e",
				HostServiceName:      "pocketbase-lifted",
				ExtractedServiceName: "monolift-extracted-passwordvalidate",
				HostPort:             8090,
				HostReadinessPath:    "/api/health",
				HostBuildPackage:     "./examples/base",
				HostBinaryName:       "pocketbase",
				HostRuntimeImage:     "alpine:3.20",
				HostArgs: []string{"/bin/sh", "-c", "/pocketbase superuser upsert admin@example.com Monolift123! --dir=/pb_data && exec /pocketbase serve --http=0.0.0.0:8090 --dir=/pb_data"},
				HostEnvVars: []codegen.EnvVar{
					{Name: "PB_SUPERUSER_EMAIL", Value: "admin@example.com"},
					{Name: "PB_SUPERUSER_PASSWORD", Value: "Monolift123!"},
				},
				HostVolumeMounts: []codegen.VolumeMount{
					{Name: "pb-data", MountPath: "/pb_data"},
				},
				HostEmptyDirVolumes: []string{"pb-data"},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-passwordvalidate": "passwordvalidate",
			"monolift-oracle-passwordvalidate":    "passwordvalidate",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-passwordvalidate",
			Dockerfile:     "lifted/Dockerfile.oracle-passwordvalidate",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-passwordvalidate:e2e",
			DeploymentYAML: "lifted/manifests/oracle-passwordvalidate-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-passwordvalidate-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"passwordvalidate": {
				"receiver": map[string]any{
					"Hash": directInvocationHash,
				},
				"pass": directInvocationPass,
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: collectionsPath, Status: true, Headers: []string{"Content-Type"}, Body: true}},
		ServiceName: "pocketbase",
		ServicePort: 8090,
	}
}
