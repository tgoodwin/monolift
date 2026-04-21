package pragma

import "github.com/tgoodwin/monolift/test/e2e/harness"

func Targets() []harness.TargetCase {
	return []harness.TargetCase{
		target("pragma-parse", "parse", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_PARSE"),
		target("pragma-unknown-key", "unknown-key", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_UNKNOWN_KEY"),
		target("pragma-invalid-surface", "invalid-key-for-surface", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_INVALID_KEY_FOR_SURFACE"),
		target("pragma-misattached", "misattached", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_MISATTACHED"),
		target("pragma-duplicate", "duplicate", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_DUPLICATE"),
		target("pragma-unknown-verb", "unknown-verb", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_UNKNOWN_VERB"),
		target("pragma-v1-deprecated", "v1-deprecated", "accept-with-warnings", "docs/specs/monolift-v2-contract.md §Pragma Syntax v2; docs/sprints/SPRINT-0005.md Phase 2", "MLV2_PRAGMA_V1_DEPRECATED"),
		target("shape-transport-handler-mismatch", "shape-transport-handler-mismatch", "refuse-blocking", "docs/specs/monolift-v2-contract.md §Canonical Shapes TA-SHAPE-1, TA-HANDLER-1", "MLV2_SHAPE_UNSUPPORTED"),
		target("state-decl-conflict-stateless-global-store", "state-decl-conflict-stateless-global-store", "refuse-blocking", "docs/specs/monolift-v2-contract.md §State Semantics SS-CLASS-3", "MLV2_STATE_DECL_CONFLICT"),
	}
}

func target(name, fixture, verdict, specTrace, diagnostic string) harness.TargetCase {
	return harness.TargetCase{
		Name:                name,
		ExpectedVerdict:     verdict,
		StopAtStage:         4,
		RequiredDiagnostics: []string{diagnostic},
		SourceDirs:          []string{"test/e2e/targets/pragma/fixtures/" + fixture},
		SpecTrace:           specTrace,
	}
}
