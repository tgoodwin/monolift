package mattermost

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Target() harness.TargetCase {
	return harness.TargetCase{
		Name:            "mattermost",
		ExpectedVerdict: "refuse-blocking",
		ExpectedRoot:    "connection-hub-buffer",
		ExpectedRoots: []string{
			"Hub.Start",
			"Hub.Broadcast",
			"Hub.Register",
			"Hub.Unregister",
			"Hub.CheckConn",
			"Hub.SendMessage",
			"Hub.ProcessAsync",
			"Hub.Stop",
			"WebConn.Pump",
			"WebConn.writePump",
		},
		SourceDirs: []string{
			"evaluation/mattermost/server",
			"test/e2e/targets/mattermost",
		},
		StopAtStage:           10,
		SkipReason:            "SPRINT-0023 branch R: machinery lands, Mattermost blocked by docs/research/runs/SPRINT-0023-mattermost-attempt.md",
		SpecTrace:             "docs/specs/monolift-v2-contract.md §Cross-target validation: Mattermost; docs/research/runs/SPRINT-0022-mattermost-overlay.md",
		EntryPathProbePackage: "./cmd/mattermost",
		EntryPathProbeRoots: []string{
			"(*Hub).Start",
			"(*WebConn).Pump",
		},
	}
}
