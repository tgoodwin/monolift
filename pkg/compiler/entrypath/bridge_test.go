package entrypath

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/tools/go/ssa"
)

func TestBridgeModeFindsRegistrationFromReverseTouchpoint(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	root := mainPkg.Func("regionRoot")
	if root == nil {
		t.Fatal("regionRoot function not found")
	}

	reverseOnly, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode: FunctionIndexModeReversePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasExternalSurface(reverseOnly.ExternalSurfaces, "callback") {
		t.Fatalf("reverse-only mode unexpectedly recovered callback: %+v", reverseOnly.ExternalSurfaces)
	}

	bridge, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:     FunctionIndexModeBridge,
		BridgeMaxStarts:       20,
		BridgeMaxPackages:     5,
		BridgeMaxOwners:       50,
		BridgeMaxInstructions: 2000,
		BridgeMaxDuration:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasTouchpoint(bridge.RegionTouchpoints, "callback") {
		t.Fatalf("bridge fixture did not expose callback as a reverse touchpoint: %+v", bridge.RegionTouchpoints)
	}
	if !hasExternalSurface(bridge.ExternalSurfaces, "callback") {
		t.Fatalf("bridge mode missed callback external surface: %+v", bridge.ExternalSurfaces)
	}
	if !hasRegistrationOwner(bridge.RegistrationSites, "(*API).install") {
		t.Fatalf("bridge mode missed install registration owner: %+v", bridge.RegistrationSites)
	}
	if bridge.Stats.FunctionIndexSeeds.BridgeOwners == 0 {
		t.Fatalf("bridge seed stats not populated: %+v", bridge.Stats.FunctionIndexSeeds)
	}
	if bridge.Stats.BridgeDiscovery.SelectedStartCount == 0 || bridge.Stats.BridgeDiscovery.BridgeBoundaryOwnerCount == 0 {
		t.Fatalf("bridge discovery stats not populated: %+v", bridge.Stats.BridgeDiscovery)
	}
}

func TestBridgeScheduledPackageKeysPrioritizeDenseStartPackages(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	_ = prog
	callback := mainPkg.Func("callback")
	otherCallback := mainPkg.Func("otherCallback")
	if callback == nil || otherCallback == nil {
		t.Fatalf("fixture functions not found: callback=%v otherCallback=%v", callback, otherCallback)
	}

	got := bridgeScheduledPackageKeys(map[string][]*ssa.Function{
		"a-low":  {callback},
		"z-high": {callback, otherCallback},
	})
	want := []string{"z-high", "a-low"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("package schedule = %+v, want %+v", got, want)
	}
}

func TestBridgeCoverageReportsPackageAndOracleAudit(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	root := mainPkg.Func("regionRoot")
	if root == nil {
		t.Fatal("regionRoot function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:     FunctionIndexModeBridge,
		BridgeMaxStarts:       20,
		BridgeMaxPackages:     5,
		BridgeMaxOwners:       50,
		BridgeMaxInstructions: 2000,
		BridgeMaxDuration:     time.Second,
		OracleSpec: OracleSpec{
			Nodes: []OracleNodeSpec{
				{ID: "callback", ObjectName: "callback"},
				{ID: "install", ObjectName: "(*API).install"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	pkgCoverage := findBridgePackageCoverage(result.Stats.BridgeDiscovery.Coverage.Packages, functionPackagePath(root))
	if pkgCoverage == nil {
		t.Fatalf("missing package coverage: %+v", result.Stats.BridgeDiscovery.Coverage.Packages)
	}
	if !pkgCoverage.Scheduled || !pkgCoverage.Scanned || !pkgCoverage.Completed {
		t.Fatalf("package scheduling/scanning status not populated: %+v", *pkgCoverage)
	}
	if pkgCoverage.ScannedFunctionCount == 0 || pkgCoverage.InstructionCount == 0 || pkgCoverage.BridgeOwnersAdmitted == 0 {
		t.Fatalf("package coverage counts not populated: %+v", *pkgCoverage)
	}
	if len(pkgCoverage.SelectedStarts) == 0 {
		t.Fatalf("package coverage missing selected starts: %+v", *pkgCoverage)
	}

	callbackTarget := findBridgeOracleTarget(result.Stats.BridgeDiscovery.Coverage.OracleTargets, "callback")
	if callbackTarget == nil || !callbackTarget.PackageSelected || !callbackTarget.PackageScanned || !callbackTarget.ProducedBridgeSeed || !callbackTarget.FunctionRefIndexed {
		t.Fatalf("callback oracle coverage incomplete: %+v", callbackTarget)
	}
	installTarget := findBridgeOracleTarget(result.Stats.BridgeDiscovery.Coverage.OracleTargets, "install")
	if installTarget == nil || !installTarget.OwnerScanned || !installTarget.RefMatcherInspected || !installTarget.ProducedBridgeSeed {
		t.Fatalf("install oracle coverage incomplete: %+v", installTarget)
	}

	installAudit := findBridgeRefMatchAudit(result.Stats.BridgeDiscovery.Coverage.RefMatchAudit, "install")
	if installAudit == nil {
		t.Fatalf("missing install ref audit: %+v", result.Stats.BridgeDiscovery.Coverage.RefMatchAudit)
	}
	if !installAudit.BridgeScanned || !installAudit.RefMatcherInspected || !installAudit.ProducedBridgeSeed {
		t.Fatalf("install audit scan/seed status incomplete: %+v", *installAudit)
	}
	if installAudit.Counts.DirectTouchpointRefs == 0 {
		t.Fatalf("install audit missing direct touchpoint refs: %+v", *installAudit)
	}
	if len(installAudit.BoundaryEvidence) == 0 {
		t.Fatalf("install audit missing boundary evidence: %+v", *installAudit)
	}
}

func TestBridgeDiscoveryBudgetsAndDuplicateSuppression(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	root := mainPkg.Func("regionRoot")
	callback := mainPkg.Func("callback")
	if root == nil || callback == nil {
		t.Fatalf("fixture functions not found: root=%v callback=%v", root, callback)
	}
	reverse := reverseBFS(prog, buildApplicationCallGraph(prog, mainPkg).graph, []*ssa.Function{root})
	if len(reverse.TouchpointFunctions) == 0 {
		t.Fatal("fixture produced no reverse touchpoint functions")
	}

	duplicated := append(append([]*ssa.Function{}, reverse.TouchpointFunctions...), callback)
	result := discoverBridgeSeeds(prog, duplicated, BridgeOptions{
		MaxStarts:           20,
		MaxPackages:         5,
		MaxPackageFunctions: 20,
		MaxOwners:           50,
		MaxBoundaryOwners:   50,
		MaxInstructions:     2000,
		MaxDuration:         time.Second,
	})
	if result.Stats.DuplicateOwnerSuppressions == 0 {
		t.Fatalf("duplicate bridge owners were not suppressed: %+v", result.Stats)
	}
	if result.Stats.SkippedStartCount == 0 || !hasBridgeSkipReason(result.Stats.SkipReasons, "duplicate") {
		t.Fatalf("duplicate bridge starts were not reported: %+v", result.Stats.SkipReasons)
	}
	if !seedSetContains(result.Seeds, mainPkg.Func("callback")) {
		t.Fatalf("bridge seeds missing callback start")
	}
	if !seedSetContains(result.Seeds, findFixtureFunction(prog, "(*API).install")) {
		t.Fatalf("bridge seeds missing local reference owner")
	}

	cases := []struct {
		name    string
		options BridgeOptions
		reason  string
	}{
		{
			name: "start",
			options: BridgeOptions{
				MaxStarts:           1,
				MaxPackages:         5,
				MaxPackageFunctions: 20,
				MaxOwners:           50,
				MaxBoundaryOwners:   50,
				MaxInstructions:     2000,
				MaxDuration:         time.Second,
			},
			reason: "start_budget",
		},
		{
			name: "package function",
			options: BridgeOptions{
				MaxStarts:           20,
				MaxPackages:         5,
				MaxPackageFunctions: 1,
				MaxOwners:           50,
				MaxBoundaryOwners:   50,
				MaxInstructions:     2000,
				MaxDuration:         time.Second,
			},
			reason: "package_function_budget",
		},
		{
			name: "owner",
			options: BridgeOptions{
				MaxStarts:           20,
				MaxPackages:         5,
				MaxPackageFunctions: 20,
				MaxOwners:           1,
				MaxBoundaryOwners:   50,
				MaxInstructions:     2000,
				MaxDuration:         time.Second,
			},
			reason: "owner_budget",
		},
		{
			name: "boundary owner",
			options: BridgeOptions{
				MaxStarts:           20,
				MaxPackages:         5,
				MaxPackageFunctions: 20,
				MaxOwners:           50,
				MaxBoundaryOwners:   1,
				MaxInstructions:     2000,
				MaxDuration:         time.Second,
			},
			reason: "boundary_owner_budget",
		},
		{
			name: "instruction",
			options: BridgeOptions{
				MaxStarts:           20,
				MaxPackages:         5,
				MaxPackageFunctions: 20,
				MaxOwners:           50,
				MaxBoundaryOwners:   50,
				MaxInstructions:     1,
				MaxDuration:         time.Second,
			},
			reason: "instruction_budget",
		},
		{
			name: "duration",
			options: BridgeOptions{
				MaxStarts:           20,
				MaxPackages:         5,
				MaxPackageFunctions: 20,
				MaxOwners:           50,
				MaxBoundaryOwners:   50,
				MaxInstructions:     2000,
				MaxDuration:         time.Nanosecond,
			},
			reason: "duration_budget",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := discoverBridgeSeeds(prog, reverse.TouchpointFunctions, tc.options)
			if !hasString(result.Stats.StopReasons, tc.reason) {
				t.Fatalf("missing stop reason %q: %+v", tc.reason, result.Stats)
			}
		})
	}
}

func TestBridgeCoverageReportsPackageStopReasons(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	root := mainPkg.Func("regionRoot")
	if root == nil {
		t.Fatal("regionRoot function not found")
	}
	reverse := reverseBFS(prog, buildApplicationCallGraph(prog, mainPkg).graph, []*ssa.Function{root})
	result := discoverBridgeSeedsWithOracleSpec(prog, reverse.TouchpointFunctions, BridgeOptions{
		MaxStarts:           20,
		MaxPackages:         5,
		MaxPackageFunctions: 1,
		MaxOwners:           50,
		MaxBoundaryOwners:   50,
		MaxInstructions:     2000,
		MaxDuration:         time.Second,
	}, OracleSpec{
		Nodes: []OracleNodeSpec{
			{ID: "install", ObjectName: "(*API).install"},
		},
	})

	pkgCoverage := findBridgePackageCoverage(result.Stats.Coverage.Packages, functionPackagePath(root))
	if pkgCoverage == nil {
		t.Fatalf("missing package coverage: %+v", result.Stats.Coverage.Packages)
	}
	if !hasString(pkgCoverage.StopReasons, "package_function_budget") {
		t.Fatalf("package coverage missing package function stop: %+v", *pkgCoverage)
	}
	target := findBridgeOracleTarget(result.Stats.Coverage.OracleTargets, "install")
	if target == nil || !hasString(target.SkipCauses, "package_function_budget") {
		t.Fatalf("oracle target missing stop cause: %+v", target)
	}
}

func TestBridgeIndexPriorityDiagnosticsAndConstrainedOrdering(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "bridge_callback"))
	boundaryOwner := mainPkg.Func("callback")
	directOwner := findFixtureFunction(prog, "(*API).install")
	lowOwner := mainPkg.Func("main")
	if boundaryOwner == nil || directOwner == nil || lowOwner == nil {
		t.Fatalf("fixture functions not found: boundary=%v direct=%v low=%v", boundaryOwner, directOwner, lowOwner)
	}

	seeds := NewFunctionIndexSeedSet()
	seeds.Add(lowOwner, FunctionIndexSeedReasonBridge)
	seeds.Add(directOwner, FunctionIndexSeedReasonBridge)
	seeds.Add(boundaryOwner, FunctionIndexSeedReasonBridge)
	seeds.AddBoundaryEvidence(BoundaryPredicateEvidence{
		Predicate: netHTTPPackagePath,
		Owner:     boundaryOwner,
		Reason:    "test_boundary",
	})
	priority := bridgeIndexPriorityContext{
		selectedTouchpointPackages: map[string]bool{
			functionPackagePath(boundaryOwner): true,
		},
		directTouchpointRefs: map[*ssa.Function]int{
			directOwner: 1,
		},
		boundaryEvidenceCounts: map[*ssa.Function]int{
			boundaryOwner: 1,
		},
	}

	ordered := bridgeIndexOwners(seeds, priority)
	if len(ordered) != 3 {
		t.Fatalf("ordered owners = %d, want 3", len(ordered))
	}
	if ordered[0] != boundaryOwner || ordered[1] != directOwner || ordered[2] != lowOwner {
		t.Fatalf("priority order = [%s, %s, %s], want boundary/direct/low",
			functionObjectName(ordered[0]), functionObjectName(ordered[1]), functionObjectName(ordered[2]))
	}

	index := buildBridgeFunctionRefIndex(seeds, priority, FunctionRefIndexOptions{MaxFunctions: 1})
	stats := finalizeBridgeDiscoveryStats(BridgeDiscoveryStats{}, seeds, index, priority)
	if stats.IndexedBridgeOwnerCount != 1 {
		t.Fatalf("indexed bridge owners = %d, want 1", stats.IndexedBridgeOwnerCount)
	}
	if !index.ScannedOwners[boundaryOwner] {
		t.Fatalf("boundary owner was not indexed first")
	}
	if index.ScannedOwners[directOwner] || index.ScannedOwners[lowOwner] {
		t.Fatalf("lower-priority owners indexed under constrained budget")
	}

	boundaryDiag := findBridgeIndexOwner(stats.Coverage.IndexOwners, functionObjectName(boundaryOwner))
	if boundaryDiag == nil || !boundaryDiag.Indexed || boundaryDiag.PriorityClass != "boundary_bridge" {
		t.Fatalf("boundary diagnostic incomplete: %+v", boundaryDiag)
	}
	if boundaryDiag.PackagePath == "" || !hasString(boundaryDiag.SeedReasons, FunctionIndexSeedReasonBridge) {
		t.Fatalf("boundary diagnostic missing package or seed reasons: %+v", *boundaryDiag)
	}
	directDiag := findBridgeIndexOwner(stats.Coverage.IndexOwners, functionObjectName(directOwner))
	if directDiag == nil || directDiag.Indexed || directDiag.SkipReason != "max_functions" || directDiag.BudgetResponsible != "max_functions" {
		t.Fatalf("direct-owner skip diagnostic incomplete: %+v", directDiag)
	}
	if directDiag.PriorityInputs.DirectTouchpointRefs != 1 || !directDiag.PriorityInputs.SelectedTouchpointPackage {
		t.Fatalf("direct-owner priority inputs incomplete: %+v", directDiag.PriorityInputs)
	}
	if !hasBridgeIndexClass(stats.IndexPriorityClassCounts, "boundary_bridge", 1, 1, 0) {
		t.Fatalf("missing boundary class aggregate: %+v", stats.IndexPriorityClassCounts)
	}
	if !hasBridgeIndexSkipReason(stats.IndexSkipReasonCounts, "max_functions", 2) {
		t.Fatalf("missing skip aggregate: %+v", stats.IndexSkipReasonCounts)
	}
}

func TestBridgeFunctionIndexBudgetIsPhaseLocal(t *testing.T) {
	options := FunctionRefIndexOptions{
		Budget:       60 * time.Second,
		MaxFunctions: 12,
	}

	got := bridgeFunctionIndexOptions(options)
	if got.Budget != options.Budget {
		t.Fatalf("bridge index budget = %s, want phase-local %s", got.Budget, options.Budget)
	}
	if got.MaxFunctions != options.MaxFunctions {
		t.Fatalf("bridge max functions = %d, want %d", got.MaxFunctions, options.MaxFunctions)
	}

	started := time.Now().Add(-2 * time.Minute)
	if remaining := remainingFunctionIndexBudget(started, options.Budget); remaining == got.Budget {
		t.Fatalf("shared remaining-budget helper unexpectedly preserved budget: %s", remaining)
	}
}

func hasBridgeSkipReason(reasons []BridgeSkipReason, want string) bool {
	for _, reason := range reasons {
		if reason.Reason == want {
			return true
		}
	}
	return false
}

func findFixtureFunction(prog *ssa.Program, objectName string) *ssa.Function {
	for _, fn := range sortedProgramFunctions(prog) {
		if functionObjectName(fn) == objectName {
			return fn
		}
	}
	return nil
}

func findBridgePackageCoverage(items []BridgePackageCoverage, packagePath string) *BridgePackageCoverage {
	for i := range items {
		if items[i].PackagePath == packagePath {
			return &items[i]
		}
	}
	return nil
}

func findBridgeOracleTarget(items []BridgeOracleTargetCoverage, id string) *BridgeOracleTargetCoverage {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func findBridgeRefMatchAudit(items []BridgeRefMatchAudit, id string) *BridgeRefMatchAudit {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

func findBridgeIndexOwner(items []BridgeIndexOwnerDiagnostic, objectName string) *BridgeIndexOwnerDiagnostic {
	for i := range items {
		if items[i].ObjectName == objectName {
			return &items[i]
		}
	}
	return nil
}

func hasBridgeIndexClass(items []BridgeIndexClassCount, class string, count, indexed, skipped int) bool {
	for _, item := range items {
		if item.PriorityClass == class && item.Count == count && item.Indexed == indexed && item.Skipped == skipped {
			return true
		}
	}
	return false
}

func hasBridgeIndexSkipReason(items []BridgeIndexSkipReasonCount, reason string, count int) bool {
	for _, item := range items {
		if item.Reason == reason && item.Count == count {
			return true
		}
	}
	return false
}
