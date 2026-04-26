package pocketbase

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "pocketbase",
		ExpectedVerdict: "refuse-blocking",
		StopAtStage:     4,
		SourceDirs:      []string{"evaluation/pocketbase"},
		RequiredDiagnostics: []string{
			"MLV2_EMBEDDED_DB_APP_ROOT",
			"MLV2_CLOSURE_TOO_LARGE",
			"MLV2_NO_ERROR_CHANNEL",
		},
		GoldenReport: "test/e2e/targets/pocketbase/golden/report.json",
		SpecTrace:    "docs/specs/monolift-v2-contract.md §Cross-target validation: PocketBase; docs/decisions/0008-pocketbase-negative-case.md",
	}
}
