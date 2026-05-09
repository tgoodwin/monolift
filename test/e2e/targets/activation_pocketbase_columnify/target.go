package activation_pocketbase_columnify

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-pocketbase-columnify",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/targets/activation_pocketbase_columnify/baseline/deployment.yaml",
			"test/e2e/targets/activation_pocketbase_columnify/baseline/service.yaml",
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   8 * time.Minute,
		Dockerfile:           "test/e2e/targets/activation_pocketbase_columnify/Dockerfile",
		ContextDir:           ".",
		SourceDirs:           []string{"evaluation/pocketbase"},
		ImageTag:             "monolift-e2e/pocketbase:e2e",
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "tools/inflector/inflector.go:24",
			ServiceName:          "monolift-extracted-columnify",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_COLUMNIFY",
			DirectInvocationProbePayload: map[string]any{
				"str": "Hello World! @#$",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/pocketbase-columnify-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-columnify:e2e",
				HostServiceName:      "pocketbase-lifted",
				ExtractedServiceName: "monolift-extracted-columnify",
				HostPort:             8090,
				HostReadinessPath:    "/api/health",
				HostBuildPackage:     "./examples/base",
				HostBinaryName:       "pocketbase",
				HostRuntimeImage:     "alpine:3.20",
				HostArgs:             []string{"/bin/sh", "-c", "/pocketbase superuser upsert admin@example.com Monolift123! --dir=/pb_data && exec /pocketbase serve --http=0.0.0.0:8090 --dir=/pb_data"},
				HostVolumeMounts: []codegen.VolumeMount{
					{Name: "pb-data", MountPath: "/pb_data"},
				},
				HostEmptyDirVolumes: []string{"pb-data"},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-columnify": "columnify",
			"monolift-oracle-columnify":    "columnify",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-columnify",
			Dockerfile:     "lifted/Dockerfile.oracle-columnify",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-columnify:e2e",
			DeploymentYAML: "lifted/manifests/oracle-columnify-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-columnify-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"columnify": {"str": "Hello World! @#$"},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: collectionsPath, Status: true, Headers: []string{"Content-Type"}, Body: true}},
		ServiceName: "pocketbase",
		ServicePort: 8090,
	}
}
