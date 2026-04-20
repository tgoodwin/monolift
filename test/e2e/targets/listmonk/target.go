package listmonk

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "listmonk",
		ExpectedVerdict: "accept-with-state-rules",
		ExpectedRoot:    "CampaignWorker",
		StopAtStage:     10,
		SkipReason:      "deferred to SPRINT-0005",
		SpecTrace:       "docs/specs/monolift-v2-contract.md §Cross-target validation: Listmonk",
	}
}
