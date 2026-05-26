package activation_miniflux_extractcontent

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/test/e2e/harness"
)

// extractInputHTML is the deterministic page driven through the direct-invoke
// oracle-compare. It carries a <base href> (so ExtractContent's first return,
// baseURL, is non-trivial), a readable article body containing a marker, and a
// <script> that ExtractContent must strip. The same shape is served in-cluster
// as article.html (rss-feed fixture) for the route-driven workload.
const extractInputHTML = `<!DOCTYPE html>
<html>
<head>
<base href="https://example.org/base/">
<title>Monolift Readability Fixture</title>
</head>
<body>
<nav>navigation chrome that readability should discard</nav>
<article>
<h1>Monolift Readability Article</h1>
<p>Monolift extracts the main article content from a web page using the readability algorithm. This paragraph is intentionally long enough to be scored as meaningful content by the candidate selection heuristics that drive content extraction across the lifted boundary.</p>
<p>The second paragraph reinforces the article body so the readability scorer keeps this block as the dominant content region, well above the surrounding navigation and footer chrome that should be discarded during extraction.</p>
<p>A third paragraph ensures the extracted marker MONOLIFT-EXTRACT-MARKER survives the round trip through the lifted ExtractContent service and back into the host response.</p>
<script>alert('xss-should-be-stripped');</script>
</article>
<footer>footer chrome that readability should discard</footer>
</body>
</html>`

// ExtractContent(page io.Reader) (baseURL, extractedContent string, err error)
// is a free function in miniflux's readability package, reached synchronously
// via GET /v1/entries/{entryID}/fetch-content (fetchContentHandler ->
// processor.ProcessEntryWebPage -> scraper.ScrapeWebsite -> ExtractContent).
//
// It exercises two generic mechanisms at once: the io.Reader parameter via the
// streaming-bytes codec ([]byte wire field "page"), and the two non-error
// string returns via ResultDTO packing (base_url / extracted_content). The
// oracle is an in-cluster service that imports the real readability package
// (the test module has no replace directive for miniflux.app, so a local
// SymbolInvoker cannot import it) — the stage-8 direct-invoke compare is then
// byte-exact between the lifted symbol and the oracle. SPRINT-0052 lift #1.
func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "activation-miniflux-extractcontent",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     10,
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/fixtures/rss-feed-server.yaml",
			"test/e2e/targets/activation_miniflux_extractcontent/baseline/deployment.yaml",
			"test/e2e/targets/activation_miniflux_extractcontent/baseline/service.yaml",
		},
		BaselineManifestPhases: [][]string{
			{"test/e2e/fixtures/postgres.yaml"},
			{"test/e2e/fixtures/rss-feed-server.yaml"},
			{
				"test/e2e/targets/activation_miniflux_extractcontent/baseline/deployment.yaml",
				"test/e2e/targets/activation_miniflux_extractcontent/baseline/service.yaml",
			},
		},
		BaselineReadyTimeout: 5 * time.Minute,
		LiftedReadyTimeout:   5 * time.Minute,
		SourceDirs:           []string{"evaluation/miniflux"},
		ActivationLift: &harness.ActivationLiftSpec{
			Target:               "internal/reader/readability/readability.go:73",
			ServiceName:          "monolift-extracted-extractcontent",
			ExpectedEnvVarPrefix: "MONOLIFT_LIFT_EXTRACTCONTENT",
			DirectInvocationProbePayload: map[string]any{
				"page": []byte(extractInputHTML),
			},
			Deploy: codegen.DeployOptions{
				HostImage:            "monolift-e2e/miniflux-extractcontent-host:e2e",
				ExtractedImage:       "monolift-e2e/extracted-extractcontent:e2e",
				HostServiceName:      "miniflux-lifted",
				ExtractedServiceName: "monolift-extracted-extractcontent",
				HostPort:             8080,
				HostReadinessPath:    "/healthcheck",
				HostBuildPackage:     ".",
				HostBinaryName:       "miniflux",
				HostEnvVars: []codegen.EnvVar{
					{Name: "DATABASE_URL", Value: "postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable"},
					{Name: "RUN_MIGRATIONS", Value: "1"},
					{Name: "CREATE_ADMIN", Value: "1"},
					{Name: "ADMIN_USERNAME", Value: "admin"},
					{Name: "ADMIN_PASSWORD", Value: "test123"},
					{Name: "LISTEN_ADDR", Value: "0.0.0.0:8080"},
					{Name: "FETCHER_ALLOW_PRIVATE_NETWORKS", Value: "1"},
				},
			},
		},
		ServiceSymbols: map[string]string{
			"monolift-extracted-extractcontent": "extractcontent",
			"monolift-oracle-extractcontent":    "extractcontent",
		},
		LiftedOracleServices: []harness.ExtractedServiceSpec{{
			Name:           "monolift-oracle-extractcontent",
			Dockerfile:     "lifted/Dockerfile.oracle-extractcontent",
			ContextRoot:    "lifted",
			ImageTag:       "monolift-e2e/oracle-extractcontent:e2e",
			DeploymentYAML: "lifted/manifests/oracle-extractcontent-deployment.yaml",
			ServiceYAML:    "lifted/manifests/oracle-extractcontent-service.yaml",
			ReadinessPath:  "/healthz",
		}},
		InvokePayloads: map[string]map[string]any{
			"extractcontent": {
				"page": []byte(extractInputHTML),
			},
		},
		Workload:    Workload{},
		Invariants:  []harness.Invariant{{Path: fetchContentPath, Status: true, Body: true}},
		ServiceName: "miniflux",
		ServicePort: 8080,
	}
}
