package activation_gitea_pathescapesegments

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-gitea-pathescapesegments",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_gitea_pathescapesegments/baseline/deployment.yaml",
			"test/e2e/targets/activation_gitea_pathescapesegments/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_gitea_pathescapesegments/baseline/deployment.yaml",
				"test/e2e/targets/activation_gitea_pathescapesegments/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 8 * time.Minute,
		LiftedReadyTimeout:   8 * time.Minute,
		SourceDirs:           []string{"evaluation/gitea"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "modules/util/url.go:12",
			ServiceName:          "monolift-extracted-pathescapesegments",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_PATHESCAPESEGMENTS",
			DirectInvocationProbePayload: map[string]any{
				"path": "feature branch/README file.md",
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/gitea-pathescapesegments-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-pathescapesegments:e2e",
				HostServiceName:      "gitea-lifted",
				ExtractedServiceName: "monolift-extracted-pathescapesegments",
				HostPort:             3000,
				HostReadinessPath:    "/api/healthz",
				HostBuildPackage:     ".",
				HostBinaryName:       "gitea",
				HostRuntimeImage:     "gitea/gitea:1.22",
				HostRuntimeSetup:     []string{"rm -f /usr/local/bin/gitea /app/gitea/gitea"},
				HostArgs:             []string{"/gitea", "web"},
				HostEnvVars: []codegen.EnvVar{
					{Name: "GITEA_WORK_DIR", Value: "/app/gitea"},
					{Name: "GITEA__database__DB_TYPE", Value: "postgres"},
					{Name: "GITEA__database__HOST", Value: "postgres:5432"},
					{Name: "GITEA__database__NAME", Value: "miniflux"},
					{Name: "GITEA__database__USER", Value: "miniflux"},
					{Name: "GITEA__database__PASSWD", Value: "miniflux"},
					{Name: "GITEA__database__SSL_MODE", Value: "disable"},
					{Name: "GITEA__security__INSTALL_LOCK", Value: "true"},
					{Name: "GITEA__security__SECRET_KEY", Value: "monolift-gitea-secret-key"},
					{Name: "GITEA__server__HTTP_ADDR", Value: "0.0.0.0"},
					{Name: "GITEA__server__HTTP_PORT", Value: "3000"},
					{Name: "GITEA__server__ROOT_URL", Value: "http://localhost:3000/"},
					{Name: "GITEA__server__START_SSH_SERVER", Value: "false"},
					{Name: "GITEA__repository__ROOT", Value: "/tmp/gitea-repositories"},
					{Name: "GITEA__service__DISABLE_REGISTRATION", Value: "false"},
					{Name: "GITEA__service__REGISTER_EMAIL_CONFIRM", Value: "false"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-pathescapesegments": "pathescapesegments",
			"monolift-oracle-pathescapesegments":    "pathescapesegments",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-pathescapesegments",
			Dockerfile:     "lifted/Dockerfile.oracle-pathescapesegments",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-pathescapesegments:e2e",
			DeploymentYAML: "lifted/manifests/oracle-pathescapesegments-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-pathescapesegments-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"pathescapesegments": {"path": "feature branch/README file.md"},
		},
		Workload: Workload{},
		Invariants: []harness.Invariant{
			{Path: repoFilePath, Status: true, Body: false},
		},
		ServiceName: "gitea",
		ServicePort: 3000,
	}
}
