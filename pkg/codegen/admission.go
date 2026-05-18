package codegen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

// Adapter refusal codes. These are the ten refusal codes produced by the
// boundary-adapter recovery pass (SPRINT-0051). The first six are the
// static feasibility obligations from the adapter strategy spec; the
// remaining four are classification or policy refusals.
const (
	RefusalAdapterFiniteInput       = "adapter_finite_input"
	RefusalAdapterLocalLifecycle    = "adapter_local_lifecycle"
	RefusalAdapterUseShape          = "adapter_use_shape"
	RefusalAdapterReturnRehydration = "adapter_return_rehydration"
	RefusalAdapterErrorOrder        = "adapter_error_order"
	RefusalAdapterCallSite          = "adapter_call_site"
	RefusalAdapterPayloadTooLarge   = "adapter_payload_too_large"
	RefusalAdapterUnknown           = "adapter_unknown"
	RefusalAdapterImpossible        = "adapter_impossible"
	RefusalLiveProxyRequired        = "live_proxy_required"
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
		case param.Codec != CodecStreamingBytes && (strings.Contains(lowerType, "io.reader") || strings.Contains(lowerType, "io.writer")):
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
	// Receiver admission: when CutPoint declares a receiver, a policy must
	// have been assigned. Boundary receivers are serialized, while factory and
	// reconstructed receivers are rebuilt inside the extracted service.
	if plan.CutPoint.Receiver != "" && plan.ReceiverParam == nil {
		verdict = refused(verdict, "receiver_requires_reconstruction", "receiver type has no applicable policy (boundary/zero/factory/reconstructed)", plan.CutPoint.Receiver)
	}
	if plan.ReceiverParam != nil {
		switch plan.ReceiverParam.Policy {
		case ReceiverBoundary:
			if !isSerializableReceiverType(plan.ReceiverParam.GoType) {
				verdict = refused(verdict, "non_serializable_receiver", "receiver type cannot be serialized over HTTP/JSON", plan.ReceiverParam.GoType)
			}
		case ReceiverReconstructed:
			if plan.ReceiverParam.Reconstructor.ID == "" {
				verdict = refused(verdict, "missing_reconstructor", "reconstructed receiver has no registered reconstructor", plan.ReceiverParam.GoType)
			}
		case ReceiverFactory, ReceiverZero:
		default:
			verdict = refused(verdict, "receiver_requires_reconstruction", "receiver type has no applicable policy (boundary/zero/factory/reconstructed)", plan.ReceiverParam.GoType)
		}
	}
	// Void-with-side-effects: refuse functions with no observable return.
	if len(plan.Results) == 0 {
		verdict = refused(verdict, "void_side_effect", "void function with no return value cannot be verified over HTTP/JSON", "")
	}
	// Multi-return admission with DTO normalization (SPRINT-0051 Phase 2).
	// (T, error) is supported directly. For > 2 results, attempt to pack
	// non-error returns into a ResultDTO. This is generic and runs for
	// every boundary, not just adapter-eligible ones.
	verdict = admitResultShape(plan, verdict)
	for _, result := range plan.Results {
		if result.Codec == CodecError {
			continue
		}
		lowerType := strings.ToLower(result.GoType)
		switch {
		case strings.Contains(lowerType, "chan "):
			verdict = refused(verdict, "streaming_type", "channel result cannot be sent over HTTP/JSON", result.GoType)
		case strings.Contains(lowerType, "io.reader") || strings.Contains(lowerType, "io.writer"):
			verdict = refused(verdict, "streaming_type", "streaming result cannot be sent over HTTP/JSON", result.GoType)
		case strings.Contains(lowerType, "sync."):
			verdict = refused(verdict, "sync_primitive", "sync primitive result cannot be sent over HTTP/JSON", result.GoType)
		}
	}
	for _, path := range deployArtifactPaths(plan) {
		if path == "" {
			continue
		}
		if err := validateGeneratedPath(plan, path); err != nil {
			verdict = refused(verdict, "generated_path_outside_module", err.Error(), path)
		}
	}
	for _, name := range []string{plan.Deploy.HostServiceName, plan.Deploy.ExtractedServiceName} {
		if name == "" {
			continue
		}
		if !isDNS1123Label(name) {
			verdict = refused(verdict, "invalid_kubernetes_name", "Kubernetes resource name must be a DNS-1123 label", name)
		}
	}
	if len(verdict.Refusals) == 0 {
		verdict.Accepted = true
		if len(verdict.Reasons) == 0 {
			verdict.Reasons = []string{"accepted by generator admission"}
		}
	}
	return verdict
}

func deployArtifactPaths(plan *Plan) []string {
	if plan == nil {
		return nil
	}
	return []string{
		plan.HostDockerfilePath,
		plan.ExtractedDockerfilePath,
		plan.HostDeploymentPath,
		plan.HostServicePath,
		plan.ExtractedDeploymentPath,
		plan.ExtractedServicePath,
		plan.SharedVolumeClaimPath,
	}
}

var dns1123LabelPattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

func isDNS1123Label(name string) bool {
	return len(name) <= 63 && dns1123LabelPattern.MatchString(name)
}

// isSerializableReceiverType returns false for types that cannot round-trip
// through JSON: channels, io interfaces, sync primitives, and function types.
func isSerializableReceiverType(goType string) bool {
	base := strings.TrimPrefix(goType, "*")
	lower := strings.ToLower(base)
	switch {
	case strings.Contains(lower, "chan "):
		return false
	case strings.Contains(lower, "io.reader"),
		strings.Contains(lower, "io.writer"),
		strings.Contains(lower, "io.readcloser"),
		strings.Contains(lower, "io.writecloser"),
		strings.Contains(lower, "io.readwriter"):
		return false
	case strings.Contains(lower, "sync."):
		return false
	case strings.HasPrefix(base, "func(") || strings.HasPrefix(base, "func ("):
		return false
	case strings.Contains(lower, "sql.db"),
		strings.Contains(lower, "sql.tx"),
		strings.Contains(lower, "sql.conn"):
		return false
	}
	return true
}

func refused(verdict AdmissionVerdict, code, message, typ string) AdmissionVerdict {
	verdict.Accepted = false
	verdict.Refusals = append(verdict.Refusals, AdmissionRefusal{Code: code, Message: message, Type: typ})
	return verdict
}

// admitResultShape checks the result shape and, for multi-return functions,
// attempts DTO normalization. For <= 2 results, the existing logic applies.
// For > 2 results, all non-error returns must be JSON-codable; if so, a
// ResultDTO is built and attached to the plan.
func admitResultShape(plan *Plan, verdict AdmissionVerdict) AdmissionVerdict {
	nonErrorCount := 0
	hasError := false
	for _, r := range plan.Results {
		if r.Codec == CodecError {
			hasError = true
		} else {
			nonErrorCount++
		}
	}
	// Single result: error-only or single non-error — admitted as-is.
	if len(plan.Results) <= 1 {
		return verdict
	}
	// (T, error): standard two-result shape — admitted as-is.
	if len(plan.Results) == 2 && nonErrorCount == 1 && hasError {
		return verdict
	}
	// Resolve funcName for DTO naming.
	funcName := plan.CutPoint.FuncName
	if funcName == "" {
		funcName = plan.CutPoint.Key.FuncName
	}
	// Two results, second is not error — attempt DTO.
	if len(plan.Results) == 2 && !hasError {
		dto := BuildResultDTO(funcName, plan.Results)
		if dto == nil {
			return refused(verdict, "unsupported_result_shape", "multi-return with no error must have all JSON-codable types", plan.Results[1].GoType)
		}
		plan.ResultDTO = dto
		plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: dto.Name}
		return verdict
	}
	// > 2 results: attempt DTO normalization.
	if len(plan.Results) > 2 {
		dto := BuildResultDTO(funcName, plan.Results)
		if dto == nil {
			return refused(verdict, "unsupported_result_shape", "multi-return values contain non-JSON-codable types", "")
		}
		plan.ResultDTO = dto
		plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: dto.Name}
		return verdict
	}
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
