package codegen

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestAdmitCutRefusesNonFeasible(t *testing.T) {
	candidate := activation.CutCandidate{
		Feasibility:  activation.Infeasible,
		BoundaryData: activation.BoundaryInfeasible,
		Callbacks:    activation.ZeroConfirmed,
	}
	verdict := AdmitCut(emptyReport(t), activation.CutResult{Recommended: &candidate})
	if verdict.Accepted {
		t.Fatal("verdict accepted infeasible cut")
	}
	if len(verdict.Refusals) == 0 {
		t.Fatal("missing refusal diagnostics")
	}
}

func TestAdmitCutAcceptsLowCallbackEvidence(t *testing.T) {
	candidate := activation.CutCandidate{
		Feasibility:  activation.Feasible,
		BoundaryData: activation.Reconstructible,
		Callbacks:    activation.Low,
	}
	verdict := AdmitCut(emptyReport(t), activation.CutResult{Recommended: &candidate})
	if !verdict.Accepted {
		t.Fatalf("verdict refused low callback evidence: %s", verdict.Error())
	}
}

func TestAdmitPlanRefusesMissingReconstructor(t *testing.T) {
	plan := &Plan{
		ReconstructedParams: []ReconstructedParam{
			{Param: Param{Name: "store", GoType: "*storage.Storage"}},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("verdict accepted missing reconstructor")
	}
	if verdict.Refusals[0].Code != "missing_reconstructor" {
		t.Fatalf("refusal = %s", verdict.Refusals[0].Code)
	}
}

func emptyReport(t *testing.T) reportv2.Report {
	t.Helper()
	return reportv2.Report{}
}
