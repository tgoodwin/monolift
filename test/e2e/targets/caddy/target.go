package caddy

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "caddy",
		ExpectedVerdict: "accept",
		StopAtStage:     10,
		SpecTrace:       "docs/specs/monolift-v2-contract.md §Cross-target validation: Caddy",
		BaselineManifests: []string{
			"test/e2e/targets/caddy/baseline/caddyfile-configmap.yaml",
			"test/e2e/targets/caddy/baseline/echo-upstream.yaml",
			"test/e2e/targets/caddy/baseline/deployment.yaml",
			"test/e2e/targets/caddy/baseline/service.yaml",
		},
		Dockerfile:   "test/e2e/targets/caddy/Dockerfile",
		ContextDir:   ".",
		SourceDirs:   []string{"evaluation/caddy", "test/e2e/targets/caddy"},
		ImageTag:     "monolift-e2e/caddy:e2e",
		LiftedHostBuild: &harness.HostBuildSpec{
			Dockerfile:  "lifted/Dockerfile.host",
			ContextRoot: "lifted",
			ImageTag:    "monolift-e2e/caddy-lifted:e2e",
		},
		LiftedExtractedServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-extracted-cleanpath",
			Dockerfile:     "lifted/extracted-cleanpath/Dockerfile",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/extracted-cleanpath:e2e",
			DeploymentYAML: "lifted/manifests/extracted-deployment.yaml",
			ServiceYAML:    "lifted/manifests/extracted-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		GoldenReport: "test/e2e/targets/caddy/golden/report.json",
		Workload:     Workload{},
		Invariants: []harness.Invariant{
			{Path: "/static/hello.txt", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/proxy?x=1", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/headers", Status: true, Headers: []string{"X-Caddy"}, Body: true},
		},
		ServiceName: "caddy",
		ServicePort: 8080,
	}
}
