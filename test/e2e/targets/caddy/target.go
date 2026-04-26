package caddy

import (
	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

func Target() harness.TargetCase {
	trueValue := true
	falseValue := false
	return harness.TargetCase{
		Name:                  "caddy",
		ExpectedVerdict:       "refuse-blocking",
		ExpectedRootShape:     "http-handler",
		ExpectedTransport:     "handler",
		ExpectedArchetypeKind: "alternative_set",
		ExpectedPrimary: harness.ExpectedArchetypeChoice{
			Archetype:              "serialized-actor",
			ContributingArchetypes: []string{"serialized-actor"},
			Alias:                  "",
			Emittable:              &trueValue,
			RuntimeSelectable:      &falseValue,
		},
		ExpectedAlternatives: []harness.ExpectedArchetypeChoice{{
			Archetype:              "keyed-partitioned-state",
			ContributingArchetypes: []string{"keyed-partitioned-state"},
			RationaleTierEqual:     "[TOPOLOGY]",
			RationaleNonEmpty:      true,
		}},
		ExpectedAdapterKind: "actor",
		ExpectedAdapterID:   "serialized-actor",
		RequiredRootFacts: []harness.ExpectedPropertyFact{
			{PropertyID: string(liftability.PropertyTransportHandlerBoundary), Verdict: "Hold"},
		},
		StopAtStage: 10,
		RequiredDiagnostics: []string{
			"MLV2_CHANNEL_BOUNDARY",
			"MLV2_REFLECTION_DISPATCH",
			"MLV2_SERIALIZATION_UNSUPPORTED",
			"MLV2_SHAPE_UNSUPPORTED",
		},
		SpecTrace: "docs/specs/monolift-v2-contract.md §Cross-target validation: Caddy",
		BaselineManifests: []string{
			"test/e2e/targets/caddy/baseline/caddyfile-configmap.yaml",
			"test/e2e/targets/caddy/baseline/echo-upstream.yaml",
			"test/e2e/targets/caddy/baseline/deployment.yaml",
			"test/e2e/targets/caddy/baseline/service.yaml",
		},
		Dockerfile: "test/e2e/targets/caddy/Dockerfile",
		ContextDir: ".",
		SourceDirs: []string{"evaluation/caddy", "test/e2e/targets/caddy"},
		ImageTag:   "monolift-e2e/caddy:e2e",
		LiftedHostBuild: &harness.HostBuildSpec{
			Dockerfile:     "lifted/Dockerfile.host",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/caddy-lifted:e2e",
			ServiceName:    "caddy-lifted",
			DeploymentYAML: "lifted/manifests/caddy-lifted-deployment.yaml",
			ServiceYAML:    "lifted/manifests/caddy-lifted-service.yaml",
		},
		LiftedExtractedServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-extracted-cleanpath",
			Dockerfile:     "lifted/Dockerfile.extracted-cleanpath",
			ContextRoot:    "lifted/host-patch",
			ImageTag:       "monolift-e2e/extracted-cleanpath:e2e",
			DeploymentYAML: "lifted/manifests/extracted-cleanpath-deployment.yaml",
			ServiceYAML:    "lifted/manifests/extracted-cleanpath-service.yaml",
			ReadinessPath:  "/healthz",
		}, {
			Name:           "monolift-extracted-sanitizemethod",
			Dockerfile:     "lifted/Dockerfile.extracted-sanitizemethod",
			ContextRoot:    "lifted/host-patch",
			ImageTag:       "monolift-e2e/extracted-sanitizemethod:e2e",
			DeploymentYAML: "lifted/manifests/extracted-sanitizemethod-deployment.yaml",
			ServiceYAML:    "lifted/manifests/extracted-sanitizemethod-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		// Regen: go build -o ./bin/e2e-compile ./test/e2e/e2ecompile && ./bin/e2e-compile --target=caddy --output=$(mktemp -d) --source=evaluation/caddy --source=test/e2e/targets/caddy
		GoldenReport: "test/e2e/targets/caddy/golden/report.json",
		Workload:     Workload{},
		Oracle:       Oracle{},
		Invariants: []harness.Invariant{
			{Path: "/static/hello.txt", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/proxy?x=1", Status: true, Headers: []string{"X-Caddy"}, Body: true},
			{Path: "/headers", Status: true, Headers: []string{"X-Caddy"}, Body: true},
		},
		ServiceName: "caddy",
		ServicePort: 8080,
	}
}
