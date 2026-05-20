package activation_miniflux_extractcontent

import (
	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

// ExtractContent(page io.Reader) (baseURL, content string, err error) is a free
// function in miniflux's readability package, reachable synchronously via
// GET /v1/entries/{id}/fetch-content (handler -> ProcessEntryWebPage ->
// ScrapeWebsite -> ExtractContent). It exercises two generic mechanisms at once:
// the io.Reader parameter via the streaming-bytes codec, and the two non-error
// string returns via ResultDTO packing. SPRINT-0052 generalization lift #1.
//
// Stage 4 (compile + selection) first: prove the lift compiles and ExtractContent
// is selected as the cut root. Higher stages (deploy + route-driven round-trip)
// are layered on once the compile is green.
func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-extractcontent",
		ExpectedVerdict: "refuse-blocking",
		ExpectedRoot:    "ExtractContent",
		StopAtStage:     4,
		SourceDirs:      []string{"evaluation/miniflux"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "internal/reader/readability/readability.go:73",
			ServiceName:          "monolift-extracted-extractcontent",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_EXTRACTCONTENT",
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-extractcontent-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-extractcontent:e2e",
				HostServiceName:      "miniflux-extractcontent-lifted",
				ExtractedServiceName: "monolift-extracted-extractcontent",
				HostPort:             8080,
				HostReadinessPath:    "/healthcheck",
				HostBuildPackage:     ".",
				HostBinaryName:       "miniflux",
				HostEnvVars: []codegen.EnvVar{
					{Name: "DATABASE_URL", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
					{Name: "RUN_MIGRATIONS", Value: "1"},
					{Name: "LISTEN_ADDR", Value: "0.0.0.0:8080"},
				},
				ExtractedEnvVars: []codegen.EnvVar{
					{Name: "DATABASE_URL", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-extractcontent": "extractcontent",
		},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
