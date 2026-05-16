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
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("verdict accepted missing reconstructor")
	}
	if len(verdict.Refusals) != 1 {
		t.Fatalf("refusals = %+v, want exactly one missing_reconstructor", verdict.Refusals)
	}
	refusal := verdict.Refusals[0]
	if refusal.Code != "missing_reconstructor" {
		t.Fatalf("refusal = %s", refusal.Code)
	}
	if refusal.Type != "*storage.Storage" {
		t.Fatalf("refusal type = %s, want *storage.Storage", refusal.Type)
	}
}

// 2G.4: Admit serializable value-receiver
func TestAdmitPlanAcceptsSerializableReceiver(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{
			Receiver: "TemplateContext",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:    "TemplateContext",
			IsPointer: false,
			Policy:    ReceiverBoundary,
			Codec:     CodecJSON,
		},
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive},
			{Name: "err", GoType: "error", Codec: CodecError},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("verdict refused serializable receiver: %s", verdict.Error())
	}
}

// 2G.5: Refuse receiver with *sql.DB
func TestAdmitPlanRefusesSqlDBReceiver(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{
			Receiver: "*sql.DB",
		},
		ReceiverParam: &ReceiverSpec{
			GoType:    "*sql.DB",
			IsPointer: true,
			Policy:    ReceiverBoundary,
			Codec:     CodecJSON,
		},
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("verdict accepted *sql.DB receiver")
	}
	found := false
	for _, r := range verdict.Refusals {
		if r.Code == "non_serializable_receiver" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected non_serializable_receiver refusal, got: %s", verdict.Error())
	}
}

// 2G.6: Admit (string, error) result
func TestAdmitPlanAcceptsStringErrorResult(t *testing.T) {
	plan := &Plan{
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive},
			{Name: "err", GoType: "error", Codec: CodecError},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("verdict refused (string, error) result: %s", verdict.Error())
	}
}

// 2G.7: Refuse io.Writer result
func TestAdmitPlanRefusesIOWriterResult(t *testing.T) {
	plan := &Plan{
		Results: []Result{
			{Name: "w", GoType: "io.Writer", Codec: CodecJSON},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("verdict accepted io.Writer result")
	}
}

func emptyReport(t *testing.T) reportv2.Report {
	t.Helper()
	return reportv2.Report{}
}
