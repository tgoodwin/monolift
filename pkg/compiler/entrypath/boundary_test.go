package entrypath

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/tools/go/ssa"
)

func TestNetHTTPBoundaryPredicateFindsBoundaryShapes(t *testing.T) {
	_, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_predicate"))
	predicate := netHTTPBoundaryPredicate{}

	for _, name := range []string{
		"handlerOwner",
		"handlerFuncOwner",
		"serverOwner",
		"shapeOwner",
	} {
		owner := mainPkg.Func(name)
		if owner == nil {
			t.Fatalf("%s function not found", name)
		}
		evidence := predicate.MatchOwner(owner)
		if len(evidence) == 0 {
			t.Fatalf("%s had no boundary predicate evidence", name)
		}
		assertBoundaryEvidenceComplete(t, owner.String(), evidence)
	}
}

func TestNetHTTPBoundaryPredicateIgnoresUnrelatedCallback(t *testing.T) {
	_, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_predicate"))
	owner := mainPkg.Func("callbackOwner")
	if owner == nil {
		t.Fatal("callbackOwner function not found")
	}

	evidence := (netHTTPBoundaryPredicate{}).MatchOwner(owner)
	if len(evidence) != 0 {
		t.Fatalf("callbackOwner unexpectedly matched boundary predicate: %+v", evidence)
	}
}

func TestNetHTTPBoundaryPredicateRecordsInstructionEvidence(t *testing.T) {
	_, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_predicate"))
	owner := mainPkg.Func("handlerOwner")
	if owner == nil {
		t.Fatal("handlerOwner function not found")
	}

	for _, evidence := range (netHTTPBoundaryPredicate{}).MatchOwner(owner) {
		if evidence.Instruction != nil {
			return
		}
	}
	t.Fatalf("handlerOwner evidence did not include an instruction")
}

func TestBoundaryFrontierRecordsSeparatePhaseStats(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_predicate"))
	root := mainPkg.Func("handlerOwner")
	if root == nil {
		t.Fatal("handlerOwner function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:                     FunctionIndexModeHTTPSinks,
		BoundaryDiscoveryMode:                 BoundaryDiscoveryModeFrontier,
		BoundaryFrontierMaxReverseOwners:      20,
		BoundaryFrontierMaxAdjacentOwners:     20,
		BoundaryFrontierMaxBoundaryCandidates: 20,
		BoundaryFrontierDepth:                 1,
		BoundaryFrontierMaxDuration:           time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	stats := result.Stats.BoundaryDiscovery
	if stats.Mode != string(BoundaryDiscoveryModeFrontier) {
		t.Fatalf("boundary discovery mode = %q, want frontier", stats.Mode)
	}
	if stats.ReverseOwners == 0 || stats.CandidateOwnerCount == 0 || stats.BoundaryCandidateOwners == 0 || stats.BoundaryEvidence == 0 || stats.SeedSetOwners == 0 {
		t.Fatalf("boundary discovery stats not populated: %+v", stats)
	}
	for _, name := range []string{
		"boundary_reverse_frontier",
		"boundary_adjacent_expansion",
		"boundary_predicate_scan",
		"boundary_seed_set_assembly",
		"function_ref_index",
	} {
		if !hasPhaseTiming(result.Stats.PhaseTimings, name) {
			t.Fatalf("missing phase timing %q in %+v", name, result.Stats.PhaseTimings)
		}
	}
}

func TestBoundaryFrontierStopDiagnostics(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_frontier"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	cases := []struct {
		name       string
		options    ProbeOptions
		diagnostic string
		reason     string
	}{
		{
			name: "reverse owner",
			options: ProbeOptions{
				BoundaryFrontierMaxReverseOwners:      1,
				BoundaryFrontierMaxAdjacentOwners:     50,
				BoundaryFrontierMaxBoundaryCandidates: 50,
				BoundaryFrontierDepth:                 2,
				BoundaryFrontierMaxDuration:           time.Second,
			},
			diagnostic: "boundary_frontier_reverse_owner_budget_exceeded",
			reason:     "reverse_owner_budget",
		},
		{
			name: "depth",
			options: ProbeOptions{
				BoundaryFrontierMaxReverseOwners:      50,
				BoundaryFrontierMaxAdjacentOwners:     50,
				BoundaryFrontierMaxBoundaryCandidates: 50,
				BoundaryFrontierDepth:                 1,
				BoundaryFrontierMaxDuration:           time.Second,
			},
			diagnostic: "boundary_frontier_depth_budget_exceeded",
			reason:     "depth_budget",
		},
		{
			name: "package",
			options: ProbeOptions{
				BoundaryFrontierMaxReverseOwners:      50,
				BoundaryFrontierMaxAdjacentOwners:     50,
				BoundaryFrontierMaxBoundaryCandidates: 50,
				BoundaryFrontierDepth:                 2,
				BoundaryFrontierMaxPackages:           1,
				BoundaryFrontierMaxDuration:           time.Second,
				FunctionRefIndexMaxFunctions:          20,
			},
			diagnostic: "boundary_frontier_package_budget_exceeded",
			reason:     "package_budget",
		},
		{
			name: "duration",
			options: ProbeOptions{
				BoundaryFrontierMaxReverseOwners:      50,
				BoundaryFrontierMaxAdjacentOwners:     50,
				BoundaryFrontierMaxBoundaryCandidates: 50,
				BoundaryFrontierDepth:                 2,
				BoundaryFrontierMaxDuration:           time.Nanosecond,
			},
			diagnostic: "boundary_frontier_duration_budget_exceeded",
			reason:     "duration_budget",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := tc.options
			options.FunctionIndexMode = FunctionIndexModeHTTPSinks
			options.BoundaryDiscoveryMode = BoundaryDiscoveryModeFrontier
			result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, options)
			if err != nil {
				t.Fatal(err)
			}
			if !hasDiagnostic(result.Diagnostics, tc.diagnostic) {
				t.Fatalf("missing diagnostic %q: %+v", tc.diagnostic, result.Diagnostics)
			}
			if !hasString(result.Stats.BoundaryDiscovery.StopReasons, tc.reason) {
				t.Fatalf("missing stop reason %q: %+v", tc.reason, result.Stats.BoundaryDiscovery.StopReasons)
			}
		})
	}
}

func TestBoundaryFrontierFindsBoundaryWithoutWholeProgramScan(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_frontier"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:                     FunctionIndexModeHTTPSinks,
		BoundaryDiscoveryMode:                 BoundaryDiscoveryModeFrontier,
		BoundaryFrontierMaxReverseOwners:      20,
		BoundaryFrontierMaxAdjacentOwners:     20,
		BoundaryFrontierMaxBoundaryCandidates: 20,
		BoundaryFrontierDepth:                 1,
		BoundaryFrontierMaxDuration:           time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FunctionRefIndex.ScannedFunctions >= result.Stats.FunctionCount {
		t.Fatalf("frontier scanned whole program: index=%d program=%d", result.Stats.FunctionRefIndex.ScannedFunctions, result.Stats.FunctionCount)
	}
	if result.Stats.BoundaryDiscovery.CandidateOwnerCount >= result.Stats.FunctionCount {
		t.Fatalf("frontier candidate set was whole program: %+v functionCount=%d", result.Stats.BoundaryDiscovery, result.Stats.FunctionCount)
	}
	if !hasExternalSurface(result.ExternalSurfaces, "external") {
		t.Fatalf("frontier missed external surface: %+v", result.ExternalSurfaces)
	}
	if !hasRegistrationOwner(result.RegistrationSites, "entry") {
		t.Fatalf("frontier missed entry registration site: %+v", result.RegistrationSites)
	}
}

func TestBoundaryFrontierAdjacentExpansionRunsWhenReverseBudgetSaturated(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_frontier"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:                     FunctionIndexModeHTTPSinks,
		BoundaryDiscoveryMode:                 BoundaryDiscoveryModeFrontier,
		BoundaryFrontierMaxReverseOwners:      1,
		BoundaryFrontierMaxAdjacentOwners:     20,
		BoundaryFrontierMaxBoundaryCandidates: 20,
		BoundaryFrontierDepth:                 1,
		BoundaryFrontierMaxDuration:           time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	stats := result.Stats.BoundaryDiscovery
	if stats.ReverseOwners != 1 {
		t.Fatalf("reverse owners = %d, want saturated budget of 1: %+v", stats.ReverseOwners, stats)
	}
	if stats.AdjacentExpansionOwners == 0 {
		t.Fatalf("adjacent expansion did not run after reverse budget saturation: %+v", stats)
	}
	if !hasString(stats.StopReasons, "reverse_owner_budget") {
		t.Fatalf("missing reverse budget stop: %+v", stats.StopReasons)
	}
}

func TestBoundaryFrontierAdjacentAndCandidateBudgetsAreIndependent(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_frontier"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:                     FunctionIndexModeHTTPSinks,
		BoundaryDiscoveryMode:                 BoundaryDiscoveryModeFrontier,
		BoundaryFrontierMaxReverseOwners:      20,
		BoundaryFrontierMaxAdjacentOwners:     2,
		BoundaryFrontierMaxBoundaryCandidates: 1,
		BoundaryFrontierDepth:                 2,
		BoundaryFrontierMaxDuration:           time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	stats := result.Stats.BoundaryDiscovery
	if stats.AdjacentExpansionOwners != 2 {
		t.Fatalf("adjacent expansion owners = %d, want 2: %+v", stats.AdjacentExpansionOwners, stats)
	}
	if stats.BoundaryCandidateOwners != 1 {
		t.Fatalf("boundary candidate owners = %d, want 1: %+v", stats.BoundaryCandidateOwners, stats)
	}
	if !hasString(stats.StopReasons, "adjacent_owner_budget") {
		t.Fatalf("missing adjacent budget stop: %+v", stats.StopReasons)
	}
	if !hasString(stats.StopReasons, "boundary_candidate_budget") {
		t.Fatalf("missing boundary candidate budget stop: %+v", stats.StopReasons)
	}
}

func TestBoundaryFrontierDurationAndFinalIndexBudgetsAreIndependent(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "boundary_frontier"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:                     FunctionIndexModeHTTPSinks,
		BoundaryDiscoveryMode:                 BoundaryDiscoveryModeFrontier,
		BoundaryFrontierMaxReverseOwners:      20,
		BoundaryFrontierMaxAdjacentOwners:     20,
		BoundaryFrontierMaxBoundaryCandidates: 20,
		BoundaryFrontierDepth:                 1,
		BoundaryFrontierMaxDuration:           time.Second,
		FunctionRefIndexBudget:                time.Nanosecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	stats := result.Stats.BoundaryDiscovery
	if stats.BoundaryCandidateOwners == 0 {
		t.Fatalf("frontier work was consumed by final index budget: %+v", stats)
	}
	if !hasString(stats.StopReasons, "index_budget") {
		t.Fatalf("missing final index budget stop: %+v", stats.StopReasons)
	}
}

func assertBoundaryEvidenceComplete(t *testing.T, owner string, evidence []BoundaryPredicateEvidence) {
	t.Helper()
	for _, item := range evidence {
		if item.Owner == nil {
			t.Fatalf("%s evidence missing owner: %+v", owner, item)
		}
		if item.StaticType == "" {
			t.Fatalf("%s evidence missing static type: %+v", owner, item)
		}
		if item.Reason == "" {
			t.Fatalf("%s evidence missing reason: %+v", owner, item)
		}
		if item.Predicate != netHTTPPackagePath {
			t.Fatalf("%s evidence predicate = %q, want %q", owner, item.Predicate, netHTTPPackagePath)
		}
	}
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasPhaseTiming(timings []PhaseTiming, name string) bool {
	for _, timing := range timings {
		if timing.Name == name {
			return true
		}
	}
	return false
}
