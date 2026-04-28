package entrypath

import (
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/tools/go/ssa"
)

func TestOracleTraceReportsPhasePresence(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "targeted_wrapper"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}
	spec := targetedWrapperOracleSpec()

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		OracleSpec: spec,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.OracleTrace == nil {
		t.Fatal("missing oracle trace")
	}
	if status := oracleNodePhaseStatus(result.OracleTrace, "external", OraclePhaseLoadedSSA); status != "present" {
		t.Fatalf("external loaded SSA status = %q, want present", status)
	}
	if status := oracleNodePhaseStatus(result.OracleTrace, "external", OraclePhaseFunctionRefIndex); status != "present" {
		t.Fatalf("external function-ref status = %q, want present", status)
	}
	if status := oracleNodePhaseStatus(result.OracleTrace, "external", OraclePhaseFinalClassification); status != "present" {
		t.Fatalf("external final status = %q, want present", status)
	}
	if status := oracleRelationshipPhaseStatus(result.OracleTrace, "external-registration", OraclePhaseFunctionRefIndex); status != "present" {
		t.Fatalf("relationship function-ref status = %q, want present", status)
	}
	if status := oracleRelationshipPhaseStatus(result.OracleTrace, "external-registration", OraclePhaseFinalClassification); status != "present" {
		t.Fatalf("relationship final status = %q, want present", status)
	}
	if missing := oracleRelationshipMissingPhase(result.OracleTrace, "external-registration"); missing != "" {
		t.Fatalf("relationship first missing phase = %q, want recovered", missing)
	}
}

func TestOracleBridgeModeSeedsBoundedPackageBridge(t *testing.T) {
	prog, mainPkg := loadFixtureProgram(t, filepath.Join("testdata", "targeted_wrapper"))
	root := mainPkg.Func("root")
	if root == nil {
		t.Fatal("root function not found")
	}

	result, err := ProbeWithOptions(prog, mainPkg, []*ssa.Function{root}, ProbeOptions{
		FunctionIndexMode:               FunctionIndexModeOracleBridge,
		OracleSpec:                      targetedWrapperOracleSpec(),
		OracleBridgeMaxPackageFunctions: 20,
		OracleBridgeMaxOwners:           20,
		OracleBridgeMaxDuration:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FunctionIndexSeeds.OracleBridgeOwners == 0 {
		t.Fatalf("oracle bridge did not add owners: %+v", result.Stats.FunctionIndexSeeds)
	}
	if !hasRegistrationOwner(result.RegistrationSites, "wrapper") {
		t.Fatalf("oracle bridge missed wrapper registration: %+v", result.RegistrationSites)
	}
	if status := oracleRelationshipPhaseStatus(result.OracleTrace, "external-registration", OraclePhaseFinalClassification); status != "present" {
		t.Fatalf("oracle bridge relationship final status = %q, want present", status)
	}
}

func targetedWrapperOracleSpec() OracleSpec {
	return OracleSpec{
		Nodes: []OracleNodeSpec{
			{ID: "external", ObjectName: "external"},
			{ID: "wrapper", ObjectName: "wrapper"},
		},
		Relationships: []OracleRelationshipSpec{
			{
				ID:                 "external-registration",
				Kind:               OracleRelationshipRegistration,
				From:               "external",
				To:                 "wrapper",
				EdgeKind:           EdgeFunctionValueArg,
				StaticTypeContains: "net/http.Handler",
				SinkKind:           "http-handler",
			},
		},
		BridgeStarts: []string{"external"},
	}
}

func oracleNodePhaseStatus(trace *OracleTrace, id, phase string) string {
	if trace == nil {
		return ""
	}
	for _, node := range trace.Nodes {
		if node.ID != id {
			continue
		}
		for _, item := range node.Phases {
			if item.Phase == phase {
				return item.Status
			}
		}
	}
	return ""
}

func oracleRelationshipPhaseStatus(trace *OracleTrace, id, phase string) string {
	if trace == nil {
		return ""
	}
	for _, rel := range trace.Relationships {
		if rel.ID != id {
			continue
		}
		for _, item := range rel.Phases {
			if item.Phase == phase {
				return item.Status
			}
		}
	}
	return ""
}

func oracleRelationshipMissingPhase(trace *OracleTrace, id string) string {
	if trace == nil {
		return ""
	}
	for _, rel := range trace.Relationships {
		if rel.ID == id {
			return rel.FirstMissingPhase
		}
	}
	return ""
}
