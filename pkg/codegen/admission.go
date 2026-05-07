package codegen

import (
	"fmt"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func AdmitCut(_ reportv2.Report, cut activation.CutResult) AdmissionVerdict {
	verdict := AdmissionVerdict{Accepted: true, Reasons: []string{"recommended cut passed generator hard gates"}}
	if cut.Recommended == nil {
		return refused(verdict, "missing_recommended_cut", "cut analysis did not produce a recommended cut", "")
	}
	verdict.Cut = cut.Recommended
	candidate := cut.Recommended
	if candidate.Feasibility != activation.Feasible {
		verdict = refused(verdict, "non_feasible_cut", fmt.Sprintf("recommended cut feasibility is %s", candidate.Feasibility), "")
	}
	switch candidate.BoundaryData {
	case activation.Trivial, activation.Serializable, activation.Reconstructible:
	default:
		verdict = refused(verdict, "unsupported_boundary_data", fmt.Sprintf("boundary data class %s is not supported by HTTP/JSON generation", candidate.BoundaryData), string(candidate.BoundaryData))
	}
	switch candidate.Callbacks {
	case activation.ZeroConfirmed, activation.ZeroEstimated, activation.Low:
	default:
		verdict = refused(verdict, "callable_boundary_values", fmt.Sprintf("callback class %s would require callable values across the boundary", candidate.Callbacks), string(candidate.Callbacks))
	}
	return verdict
}

func AdmitPlan(plan *Plan, base AdmissionVerdict) AdmissionVerdict {
	verdict := base
	if plan == nil {
		return refused(verdict, "missing_plan", "no codegen plan was built", "")
	}
	for _, param := range plan.BoundaryParams {
		lowerType := strings.ToLower(param.GoType)
		switch {
		case strings.Contains(lowerType, "chan "):
			verdict = refused(verdict, "streaming_type", "channel boundary parameter cannot be sent over HTTP/JSON", param.GoType)
		case strings.Contains(lowerType, "io.reader") || strings.Contains(lowerType, "io.writer"):
			verdict = refused(verdict, "streaming_type", "streaming boundary parameter cannot be sent over HTTP/JSON", param.GoType)
		case strings.Contains(lowerType, "sync."):
			verdict = refused(verdict, "sync_primitive", "sync primitive boundary parameter cannot be sent over HTTP/JSON", param.GoType)
		case param.Codec == "":
			verdict = refused(verdict, "missing_codec", "boundary parameter has no JSON codec", param.GoType)
		}
	}
	for _, param := range plan.ReconstructedParams {
		if param.Reconstructor.ID == "" {
			verdict = refused(verdict, "missing_reconstructor", "reconstructed parameter has no registered reconstructor", param.GoType)
		}
	}
	if len(plan.Results) > 1 {
		verdict = refused(verdict, "unsupported_result_shape", "multiple return values are not supported by the MVP HTTP/JSON generator", "")
	}
	if len(verdict.Refusals) == 0 {
		verdict.Accepted = true
		if len(verdict.Reasons) == 0 {
			verdict.Reasons = []string{"accepted by generator admission"}
		}
	}
	return verdict
}

func refused(verdict AdmissionVerdict, code, message, typ string) AdmissionVerdict {
	verdict.Accepted = false
	verdict.Refusals = append(verdict.Refusals, AdmissionRefusal{Code: code, Message: message, Type: typ})
	return verdict
}

func (v AdmissionVerdict) Error() string {
	if v.Accepted {
		return ""
	}
	if len(v.Refusals) == 0 {
		return "generation refused"
	}
	parts := make([]string, 0, len(v.Refusals))
	for _, refusal := range v.Refusals {
		if refusal.Type != "" {
			parts = append(parts, refusal.Code+": "+refusal.Message+" ("+refusal.Type+")")
		} else {
			parts = append(parts, refusal.Code+": "+refusal.Message)
		}
	}
	return strings.Join(parts, "; ")
}
