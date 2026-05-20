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

// SPRINT-0052 task 2.2 (flag B-10): the MONOLIFT_BOUNDARY_ADAPTER flag must do
// exactly one thing — enable the recovery branch in admitCutCandidates. It must
// not silently suppress the callable_boundary_values refusal in the base
// AdmitCut verdict. AdmitCut reports the refusal regardless of the flag; a
// callable candidate is admitted only when adapter recovery later succeeds.
func TestAdmitCutReportsCallableBoundaryRegardlessOfFlag(t *testing.T) {
	for _, flag := range []string{"", "0", "1"} {
		t.Run("flag="+flag, func(t *testing.T) {
			t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", flag)
			candidate := activation.CutCandidate{
				Feasibility:  activation.Feasible,
				BoundaryData: activation.Serializable,
				Callbacks:    activation.Many,
			}
			verdict := AdmitCut(emptyReport(t), activation.CutResult{Recommended: &candidate})
			if verdict.Accepted {
				t.Fatalf("AdmitCut accepted a high-callback candidate with flag=%q; callable_boundary_values must be reported", flag)
			}
			if !hasRefusal(verdict, "callable_boundary_values") {
				t.Fatalf("flag=%q: expected callable_boundary_values refusal, got: %s", flag, verdict.Error())
			}
		})
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

func TestAdmitPlanAcceptsReconstructedFilesystemReceiver(t *testing.T) {
	plan := filesystemReceiverPlan()
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("verdict refused reconstructed filesystem receiver: %s", verdict.Error())
	}
}

func TestAdmitPlanRefusesReconstructedReceiverMissingMetadata(t *testing.T) {
	plan := filesystemReceiverPlan()
	plan.ReceiverParam.Reconstructor = Reconstructor{}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("verdict accepted reconstructed receiver without metadata")
	}
	found := false
	for _, refusal := range verdict.Refusals {
		if refusal.Code == "missing_reconstructor" && refusal.Type == "*System" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing metadata refusal not found: %s", verdict.Error())
	}
}

func TestPreflightReceiverAdmissionAllowsKnownFilesystemReconstructor(t *testing.T) {
	candidate := activation.CutCandidate{
		NodeKey: activation.FunctionKey{
			PackagePath: "github.com/pocketbase/pocketbase/tools/filesystem",
			Receiver:    "*System",
			FuncName:    "CreateThumb",
		},
		State: activation.ClientReconstructible,
	}
	verdict, refused := preflightReceiverAdmission(AdmissionVerdict{Accepted: true}, candidate)
	if refused || !verdict.Accepted {
		t.Fatalf("preflight refused known filesystem reconstructor: refused=%v verdict=%s", refused, verdict.Error())
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

// --- DTO normalization admission tests (SPRINT-0051 Phase 2, requirement 6) ---

// Shape 1: (T, error) — standard two-result, admitted as-is without DTO.
func TestAdmitPlanTErrorShapeNoDTO(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Fetch"},
		Results: []Result{
			{Name: "result", GoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 1},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("(T, error) shape refused: %s", verdict.Error())
	}
	if plan.ResultDTO != nil {
		t.Fatal("(T, error) shape should not produce a ResultDTO")
	}
}

// Shape 2: (T) — single non-error result, admitted as-is without DTO.
func TestAdmitPlanSingleResultNoDTO(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Validate"},
		Results: []Result{
			{Name: "result", GoType: "bool", Codec: CodecPrimitive, Index: 0},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("(T) shape refused: %s", verdict.Error())
	}
	if plan.ResultDTO != nil {
		t.Fatal("(T) shape should not produce a ResultDTO")
	}
}

// Shape 3: (T, U, error) — three results with error, admitted via DTO.
func TestAdmitPlanTUErrorShapeDTO(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Parse"},
		Results: []Result{
			{Name: "name", GoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "count", GoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 2},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("(T, U, error) shape refused: %s", verdict.Error())
	}
	if plan.ResultDTO == nil {
		t.Fatal("(T, U, error) shape should produce a ResultDTO")
	}
	if len(plan.ResultDTO.Fields) != 2 {
		t.Fatalf("expected 2 DTO fields, got %d", len(plan.ResultDTO.Fields))
	}
	if plan.ReturnCodec.Kind != CodecResultDTO {
		t.Fatalf("expected ReturnCodec.Kind = %s, got %s", CodecResultDTO, plan.ReturnCodec.Kind)
	}
}

// Shape 4: ([]byte, int, int, error) — the M-4 processImage shape, admitted via DTO.
func TestAdmitPlanM4ProcessImageShapeDTO(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "ProcessImage"},
		Results: []Result{
			{Name: "data", GoType: "[]byte", Codec: CodecJSON, Index: 0},
			{Name: "width", GoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "height", GoType: "int", Codec: CodecPrimitive, Index: 2},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 3},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("([]byte, int, int, error) shape refused: %s", verdict.Error())
	}
	if plan.ResultDTO == nil {
		t.Fatal("M-4 shape should produce a ResultDTO")
	}
	if len(plan.ResultDTO.Fields) != 3 {
		t.Fatalf("expected 3 DTO fields, got %d", len(plan.ResultDTO.Fields))
	}
	if plan.ResultDTO.Name != "processImageResult" {
		t.Fatalf("expected DTO name processImageResult, got %s", plan.ResultDTO.Name)
	}
}

// Shape 5: (T, T) — two non-error results, admitted via DTO.
func TestAdmitPlanTwoNonErrorShapeDTO(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Split"},
		Results: []Result{
			{Name: "first", GoType: "string", Codec: CodecPrimitive, Index: 0},
			{Name: "second", GoType: "string", Codec: CodecPrimitive, Index: 1},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if !verdict.Accepted {
		t.Fatalf("(T, T) shape refused: %s", verdict.Error())
	}
	if plan.ResultDTO == nil {
		t.Fatal("(T, T) shape should produce a ResultDTO")
	}
	if len(plan.ResultDTO.Fields) != 2 {
		t.Fatalf("expected 2 DTO fields, got %d", len(plan.ResultDTO.Fields))
	}
}

// Shape 6: void — no results, refused with void_side_effect.
func TestAdmitPlanVoidRefused(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Fire"},
		Results:  nil,
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("void shape should be refused")
	}
	found := false
	for _, r := range verdict.Refusals {
		if r.Code == "void_side_effect" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected void_side_effect refusal, got: %s", verdict.Error())
	}
}

// Negative: channel result in multi-return refuses with streaming_type.
func TestAdmitPlanRefusesChanResult(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "Stream"},
		Results: []Result{
			{Name: "ch", GoType: "chan int", Codec: CodecJSON, Index: 0},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 1},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("chan result should be refused")
	}
	found := false
	for _, r := range verdict.Refusals {
		if r.Code == "streaming_type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected streaming_type refusal for chan, got: %s", verdict.Error())
	}
}

// Negative: func() error in multi-return refuses with unsupported_result_shape
// (not JSON-codable, so DTO normalization fails).
func TestAdmitPlanRefusesFuncResultInMultiReturn(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "BadMulti"},
		Results: []Result{
			{Name: "callback", GoType: "func() error", Codec: CodecJSON, Index: 0},
			{Name: "count", GoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 2},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("func() error in multi-return should be refused")
	}
	found := false
	for _, r := range verdict.Refusals {
		if r.Code == "unsupported_result_shape" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected unsupported_result_shape refusal, got: %s", verdict.Error())
	}
}

// Negative: io.Writer in multi-return refuses with streaming_type (per-result loop).
func TestAdmitPlanRefusesIOWriterInMultiReturn(t *testing.T) {
	plan := &Plan{
		CutPoint: CutPoint{FuncName: "BadIO"},
		Results: []Result{
			{Name: "w", GoType: "io.Writer", Codec: CodecJSON, Index: 0},
			{Name: "count", GoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", GoType: "error", Codec: CodecError, Index: 2},
		},
	}
	verdict := AdmitPlan(plan, AdmissionVerdict{Accepted: true})
	if verdict.Accepted {
		t.Fatal("io.Writer in multi-return should be refused")
	}
	found := false
	for _, r := range verdict.Refusals {
		if r.Code == "streaming_type" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected streaming_type refusal for io.Writer, got: %s", verdict.Error())
	}
}

func emptyReport(t *testing.T) reportv2.Report {
	t.Helper()
	return reportv2.Report{}
}
