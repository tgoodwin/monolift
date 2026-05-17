package activation_gitea_rpmrepo

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-gitea-rpmrepo",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     7,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/targets/activation_gitea_argon2hash/baseline/deployment.yaml",
			"test/e2e/targets/activation_gitea_argon2hash/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{
				"test/e2e/targets/activation_gitea_argon2hash/baseline/deployment.yaml",
				"test/e2e/targets/activation_gitea_argon2hash/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 8 * time.Minute,
		LiftedReadyTimeout:   10 * time.Minute,
		SourceDirs:           []string{"evaluation/gitea"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "services/packages/rpm/repository.go:163",
			ServiceName:          "monolift-extracted-gitea-rpmrepo",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_GITEA_RPMREPO",
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/gitea-rpmrepo-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-gitea-rpmrepo:e2e",
				HostServiceName:      "gitea-lifted",
				ExtractedServiceName: "monolift-extracted-gitea-rpmrepo",
				HostPort:             3000,
				HostReadinessPath:    "/api/healthz",
				HostBuildPackage:     ".",
				HostBinaryName:       "gitea",
				HostBuildCommand:     "CGO_ENABLED=0 go generate -tags 'bindata sqlite sqlite_unlock_notify timetzdata' ./modules/options ./modules/public ./modules/templates ./modules/migration && CGO_ENABLED=0 go build -mod=mod -tags 'bindata sqlite sqlite_unlock_notify timetzdata' -o /out/gitea .",
				HostRuntimeImage:     "gitea/gitea:1.26.1",
				HostRuntimeSetup:     []string{"sed -i 's#/usr/local/bin/gitea#/gitea#g' /etc/s6/gitea/run"},
				HostArgs:             []string{"/usr/bin/entrypoint"},
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
			"monolift-extracted-gitea-rpmrepo": "gitea_rpmrepo",
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: healthPath, Status: true, Body: true}},
		ServiceName: "gitea",
		ServicePort: 3000,
	}
}
