package miniflux

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "miniflux",
		ExpectedVerdict: "accept",
		StopAtStage:     10,
		SkipReason:      "deferred pending v2 compiler FeedProcessor lift — SPRINT-0005",
		SpecTrace:       "docs/specs/monolift-v2-contract.md §Cross-target validation: Miniflux",
		BaselineManifests: []string{
			"test/e2e/fixtures/postgres.yaml",
			"test/e2e/fixtures/rss-feed-server.yaml",
			"test/e2e/targets/miniflux/baseline/deployment.yaml",
			"test/e2e/targets/miniflux/baseline/service.yaml",
		},
		GoldenReport: "test/e2e/targets/miniflux/golden/report.json",
		Workload:     Workload{},
		ServiceName:  "miniflux",
		ServicePort:  8080,
	}
}
