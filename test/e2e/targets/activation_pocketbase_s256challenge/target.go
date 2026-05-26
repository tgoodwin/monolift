package activation_pocketbase_s256challenge

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

// directInvocationCode is a fixed PKCE-style code verifier fed to the
// direct-invoke oracle compare. S256Challenge hashes any string, so the value
// only needs to be deterministic.
const directInvocationCode = "monolift-fixed-code-verifier-0123456789ab"

// S256Challenge(code string) string is a free function in pocketbase's
// security package (tools/security/crypto.go:18): sha256 of code, base64url
// without padding (the PKCE code challenge). It is reached synchronously via
// the public GET /api/collections/{collection}/auth-methods route, which calls
// security.S256Challenge(info.CodeVerifier) per PKCE-enabled OAuth2 provider
// and surfaces the result as oauth2.providers[].codeChallenge.
//
// This lift proves the generic machinery generalizes to a third app
// (pocketbase) and the plainest possible shape — a string -> string pure
// transform with a single non-error return (result key, no DTO). The oracle is
// an in-cluster service importing the real security package (consistent with
// the columnify/passwordvalidate pocketbase targets). SPRINT-0052 lift #2.
func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-pocketbase-s256challenge",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/targets/activation_pocketbase_s256challenge/baseline/deployment.yaml",
			"test/e2e/targets/activation_pocketbase_s256challenge/baseline/service.yaml",
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   8 * time.Minute,
		Dockerfile:           "test/e2e/targets/activation_pocketbase_s256challenge/Dockerfile",
		ContextDir:           "evaluation/pocketbase",
		SourceDirs:           []string{"evaluation/pocketbase"},
		ImageTag:             "monolift-e2e/pocketbase:e2e",
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "tools/security/crypto.go:18",
			ServiceName:          "monolift-extracted-s256challenge",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_S256CHALLENGE",
			DirectInvocationProbePayload: map[string]any{
				"code": directInvocationCode,
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/pocketbase-s256challenge-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-s256challenge:e2e",
				HostServiceName:      "pocketbase-lifted",
				ExtractedServiceName: "monolift-extracted-s256challenge",
				HostPort:             8090,
				HostReadinessPath:    "/api/health",
				HostBuildPackage:     "./examples/base",
				HostBinaryName:       "pocketbase",
				HostRuntimeImage:     "alpine:3.20",
				HostArgs:             []string{"/bin/sh", "-c", "/pocketbase superuser upsert admin@example.com Monolift123! --dir=/pb_data && exec /pocketbase serve --http=0.0.0.0:8090 --dir=/pb_data"},
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
			"monolift-extracted-s256challenge": "s256challenge",
			"monolift-oracle-s256challenge":    "s256challenge",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-s256challenge",
			Dockerfile:     "lifted/Dockerfile.oracle-s256challenge",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-s256challenge:e2e",
			DeploymentYAML: "lifted/manifests/oracle-s256challenge-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-s256challenge-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"s256challenge": {
				"code": directInvocationCode,
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: authMethodsPath, Status: true, Headers: []string{"Content-Type"}, Body: true}},
		ServiceName: "pocketbase",
		ServicePort: 8090,
	}
}
