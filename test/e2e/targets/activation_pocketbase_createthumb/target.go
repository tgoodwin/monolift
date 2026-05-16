package activation_pocketbase_createthumb

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

const (
	directOriginalKey = "wsmn24bux7wo113/84nmscqy84lsi1t/300_WlbFWSGmW9.png"
	directThumbKey    = "monolift-direct/thumbs_300_WlbFWSGmW9.png/100x100_300_WlbFWSGmW9.png"
	// test/e2e/fixtures/kind-config.yaml mounts the same host directory at
	// /data on every worker node. Plain /tmp hostPath directories are
	// node-local and do not work when host and extracted pods land on
	// different workers.
	kindSharedHostPathRoot = "/data/monolift-e2e/pocketbase-createthumb-durable-root"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-pocketbase-createthumb",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/targets/activation_pocketbase_createthumb/baseline/deployment.yaml",
			"test/e2e/targets/activation_pocketbase_createthumb/baseline/service.yaml",
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   8 * time.Minute,
		Dockerfile:           "test/e2e/targets/activation_pocketbase_createthumb/Dockerfile",
		ContextDir:           "evaluation/pocketbase",
		SourceDirs:           []string{"evaluation/pocketbase"},
		ImageTag:             "monolift-e2e/pocketbase-createthumb:e2e",
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "tools/filesystem/filesystem.go:489",
			ServiceName:          "monolift-extracted-createthumb",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_CREATETHUMB",
			DirectInvocationProbePayload: map[string]any{
				"original_key": directOriginalKey,
				"thumb_key":    directThumbKey,
				"thumb_size":   "100x100",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/pocketbase-createthumb-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-createthumb:e2e",
				HostServiceName:      "pocketbase-lifted",
				ExtractedServiceName: "monolift-extracted-createthumb",
				HostPort:             8090,
				HostReadinessPath:    "/api/health",
				HostBuildPackage:     "./examples/base",
				HostBinaryName:       "pocketbase",
				HostRuntimeImage:     "alpine:3.20",
				HostArgs: []string{"/bin/sh", "-c", `
rm -rf /pb_data /monolift/durable/*
mkdir -p /pb_data /monolift/durable
cp -a /seed_pb_data/. /pb_data/
rm -rf /pb_data/storage
cp -a /seed_pb_data/storage/. /monolift/durable/
rm -rf /monolift/durable/wsmn24bux7wo113/84nmscqy84lsi1t/thumbs_300_WlbFWSGmW9.png
ln -s /monolift/durable /pb_data/storage
/pocketbase superuser upsert admin@example.com Monolift123! --dir=/pb_data
exec /pocketbase serve --http=0.0.0.0:8090 --dir=/pb_data
`},
				HostEnvVars: []codegen.EnvVar{
					{Name: "PB_SUPERUSER_EMAIL", Value: "admin@example.com"},
					{Name: "PB_SUPERUSER_PASSWORD", Value: "Monolift123!"},
				},
				HostAssetCopies: []codegen.AssetCopy{
					{From: "tests/data", To: "/seed_pb_data"},
				},
				HostVolumeMounts: []codegen.VolumeMount{
					{Name: "pb-data", MountPath: "/pb_data"},
				},
				HostEmptyDirVolumes: []string{"pb-data"},
				SharedVolumeMounts: []codegen.SharedVolumeMount{{
					Name:      "monolift-durable-root",
					ClaimName: "monolift-extracted-createthumb-durable-root",
					MountPath: "/monolift/durable",
					HostPath:  kindSharedHostPathRoot,
				}},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-createthumb": "createthumb",
		},
		InvokePayloads: map[string]map[string]any{
			"createthumb": {
				"original_key": directOriginalKey,
				"thumb_key":    directThumbKey,
				"thumb_size":   "100x100",
			},
		},
		DirectInvoke: harness.DirectInvokeCheck{
			Expectation: harness.DirectInvokeStatusOnly,
		},
		BehavioralPredicates: []harness.BehavioralPredicate{{
			Name:        "thumbnail-100x100",
			Description: "The file workload must return a generated 100x100 thumbnail image from the shared durable root.",
		}},
		FreshResourcePolicy: harness.FreshResourcePolicy{
			ResourceKind: "filesystem-root",
			Scope:        "per e2e namespace and per workload request",
			Description:  "The host command recreates PocketBase data and storage on startup; each workload request uploads a new image so env-on, env-off, fail-mode, and restored-service checks do not reuse an earlier thumbnail.",
		},
		Workload:    Workload{},
		ServiceName: "pocketbase",
		ServicePort: 8090,
	}
}
