package activation_caddy_cleanpath

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-caddy-cleanpath",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/targets/caddy/baseline/caddyfile-configmap.yaml",
			"test/e2e/targets/caddy/baseline/echo-upstream.yaml",
			"test/e2e/targets/caddy/baseline/deployment.yaml",
			"test/e2e/targets/caddy/baseline/service.yaml",
		},
		BaselineReadyTimeout: 3 * time.Minute,
		LiftedReadyTimeout:   3 * time.Minute,
		Dockerfile:           "test/e2e/targets/caddy/Dockerfile",
		ContextDir:           ".",
		SourceDirs:           []string{"evaluation/caddy", "test/e2e/targets/caddy"},
		ImageTag:             "monolift-e2e/caddy:e2e",
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "modules/caddyhttp/caddyhttp.go:279",
			ServiceName:          "monolift-extracted-cleanpath",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_CLEANPATH",
			DirectInvocationProbePayload: map[string]any{
				"p":                "/static/hello.txt",
				"collapse_slashes": true,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/caddy-cleanpath-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-cleanpath-activation:e2e",
				HostServiceName:      "caddy-lifted",
				ExtractedServiceName: "monolift-extracted-cleanpath",
				HostPort:             8080,
				HostReadinessPath:    "/static/hello.txt",
				HostBuildPackage:     "./cmd/caddy",
				HostBinaryName:       "caddy",
				HostArgs:             []string{"/caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"},
				HostAssetCopies: []codegen.AssetCopy{
					{From: "static", To: "/srv/static"},
				},
				HostVolumeMounts: []codegen.VolumeMount{
					{Name: "caddyfile", MountPath: "/etc/caddy"},
				},
				HostConfigMapVolumes: []codegen.ConfigMapVolume{
					{Name: "caddyfile", ConfigMapName: "caddyfile"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-cleanpath": "cleanpath",
			"monolift-oracle-cleanpath":    "cleanpath",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-cleanpath",
			Dockerfile:     "lifted/Dockerfile.oracle-cleanpath",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-cleanpath:e2e",
			DeploymentYAML: "lifted/manifests/oracle-cleanpath-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-cleanpath-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"cleanpath": {
				"p":                "/static/hello.txt",
				"collapse_slashes": true,
			},
		},
		Workload: Workload{},
		Invariants: []harness.Invariant{
			{Path: "/static/hello.txt", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/proxy?x=1", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/headers", Status: true, Headers: []string{"X-Caddy"}, Body: true},
		},
		ServiceName: "caddy",
		ServicePort: 8080,
	}
}
