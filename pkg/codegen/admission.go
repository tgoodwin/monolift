package codegen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

// Adapter refusal codes produced by the boundary-adapter recovery pass.
// The first six (SPRINT-0051) are the static feasibility obligations from
// the adapter strategy spec; the remaining classification/policy refusals
// are produced by the admission iteration and call-site selection logic.
// Every active code is documented in docs/decisions/0032-boundary-adapter-
// recovery.md; new codes must be added to the ADR vocabulary in the same
// commit that introduces them. SPRINT-0052 task 1.8.
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
	// RefusalAdapterParentForbidden is produced by the admission iteration
	// when a candidate is a strict ancestor (lower Step) of a deeper
	// candidate whose AdapterClass is not DirectBoundary — see
	// adapterParentForbiddenForCandidate in cut_admit.go. The structural
	// property is: the path has an adapter-shaped leaf, so the broader
	// parent must not be selected in its place. The code names no function
	// and no target. Documented in ADR-0032 §"Admission iteration".
	RefusalAdapterParentForbidden = "adapter_parent_forbidden"
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
		if !boundaryAdapterEnabled() {
			verdict = refused(verdict, "callable_boundary_values", fmt.Sprintf("callback class %s would require callable values across the boundary", candidate.Callbacks), string(candidate.Callbacks))
		}
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
	// Multi-return admission with DTO normalization. Single-result and
	// (T, error) shapes are supported directly and never carry a DTO. For
	// shapes the base admission cannot represent (two non-error returns, or
	// > 2 results), DTO packing runs as a recovery gated on the would-be
	// unsupported_result_shape refusal — see admitResultShape.
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

// admitResultShape gates DTO packing on candidacy: a ResultDTO is only built
// when the result shape would otherwise be refused with unsupported_result_shape.
// Natively supported shapes — single result and (T, error) — are admitted as-is
// and never carry a DTO. For a multi-value shape that the base admission cannot
// represent, DTO packing runs as a recovery: a successful pack shadows
// (suppresses) the refusal; a failed pack leaves the refusal standing.
func admitResultShape(plan *Plan, verdict AdmissionVerdict) AdmissionVerdict {
	shapeRefusal, refuses := baseResultShapeRefusal(plan)
	if !refuses {
		return verdict
	}
	funcName := plan.CutPoint.FuncName
	if funcName == "" {
		funcName = plan.CutPoint.Key.FuncName
	}
	dto := BuildResultDTO(funcName, plan.Results)
	if dto == nil {
		return refused(verdict, shapeRefusal.Code, shapeRefusal.Message, shapeRefusal.Type)
	}
	plan.ResultDTO = dto
	plan.ReturnCodec = ReturnCodec{Kind: CodecResultDTO, GoType: dto.Name}
	return verdict
}

// baseResultShapeRefusal returns the unsupported_result_shape refusal the plan
// would receive without DTO packing, and whether such a refusal applies. It is
// the refusal-shadow check that gates DTO packing — packing never fires for the
// natively supported single-result and (T, error) shapes.
func baseResultShapeRefusal(plan *Plan) (AdmissionRefusal, bool) {
	hasError := false
	for _, r := range plan.Results {
		if r.Codec == CodecError {
			hasError = true
		}
	}
	switch {
	case len(plan.Results) <= 1:
		// Error-only or single non-error: admitted as-is.
		return AdmissionRefusal{}, false
	case len(plan.Results) == 2 && hasError:
		// (T, error) (and the degenerate two-error shape): admitted as-is.
		return AdmissionRefusal{}, false
	case len(plan.Results) == 2:
		// (T, U): unrepresentable without packing the non-error returns.
		return AdmissionRefusal{Code: "unsupported_result_shape", Message: "multi-return with no error must have all JSON-codable types", Type: plan.Results[1].GoType}, true
	default:
		// > 2 results: unrepresentable without packing.
		return AdmissionRefusal{Code: "unsupported_result_shape", Message: "multi-return values contain non-JSON-codable types", Type: ""}, true
	}
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
