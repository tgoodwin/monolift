package liftability

import (
	"fmt"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func decideAdmission(loaded *extract.LoadedModule, op Operation, properties []Evidence) (Classification, []extract.Diagnostic) {
	classification := Classification{
		Operation:  op.Identity,
		Admission:  AdmissionLiftable,
		Properties: append([]Evidence(nil), properties...),
	}
	handlerBoundary := propertyHasVerdict(properties, PropertyTransportHandlerBoundary, VerdictHold)
	var diagnostics []extract.Diagnostic
	for _, property := range properties {
		if property.Verdict != VerdictViolate {
			continue
		}
		code, message, ruleIDs, ok := refusalForProperty(property, handlerBoundary)
		if !ok {
			continue
		}
		classification.Admission = AdmissionRefused
		if classification.RefusalCode == "" {
			classification.RefusalCode = code
		}
		diagnostics = append(diagnostics, extract.Diagnostic{
			Code:     code,
			Severity: extract.SeverityError,
			Message:  message,
			Span:     loaded.RootPragma.Span,
			RuleIDs:  append([]string(nil), ruleIDs...),
		})
	}
	if classification.Admission == AdmissionLiftable {
		return classification, nil
	}
	sortExtractDiagnostics(diagnostics)
	return classification, dedupeDiagnostics(diagnostics)
}

func refusalForProperty(property Evidence, handlerBoundary bool) (code, message string, ruleIDs []string, ok bool) {
	switch property.PropertyID {
	case PropertyBoundaryVariadicFree, PropertyBoundaryNoCallableValues:
		return "MLV2_SHAPE_UNSUPPORTED", property.Detail, []string{"TA-REFUSE-1", "TA-SHAPE-1", "AS-FUNC-2"}, true
	case PropertyBoundaryNoStreamingValues:
		return "MLV2_CHANNEL_BOUNDARY", property.Detail, []string{"TA-SER-7"}, true
	case PropertyBoundaryNoSyncPrimitives, PropertyBoundarySerializableViaCustomEncoding:
		return "MLV2_SERIALIZATION_UNSUPPORTED", property.Detail, []string{"TA-REFUSE-1"}, true
	case PropertyBoundaryFullyInstantiated:
		return "MLV2_SURFACE_DEFERRED_GENERIC_DECL", property.Detail, []string{"AS-FUNC-2"}, true
	case PropertyEffectsNoParamHeapMutation:
		return "MLV2_POINTER_ALIAS_UNSUPPORTED", property.Detail, []string{"TA-REFUSE-1"}, true
	case PropertyEffectsNoGlobalWrites:
		return "MLV2_SHARED_MUTABLE_STATE", property.Detail, []string{"SS-DISP-2"}, true
	case PropertyEffectsNoReflectUnsafe:
		if strings.Contains(property.Detail, "reflect.") {
			return "MLV2_REFLECTION_DISPATCH", property.Detail, []string{"TA-REFUSE-1"}, true
		}
		return "MLV2_UNSAFE_CODE", property.Detail, []string{"TA-REFUSE-1"}, true
	case PropertyContractErrorLast, PropertyContractNoPanicOnlyFailure:
		if handlerBoundary {
			return "", "", nil, false
		}
		return "MLV2_NO_ERROR_CHANNEL", property.Detail, []string{"SS-WALDO-2", "TA-SHAPE-1"}, true
	case PropertyTransportReceiverReturnsSelf:
		return "MLV2_BUILDER_CHAIN_ROOT", property.Detail, []string{"TA-SHAPE-1"}, true
	default:
		return "", "", nil, false
	}
}

func aggregateRoot(loaded *extract.LoadedModule, root reportv2.Root, perOperation []Classification) (Classification, []extract.Diagnostic) {
	rootClassification := Classification{
		Operation:  root.Identity,
		Admission:  AdmissionUnsupported,
		Properties: []Evidence{},
	}
	if len(perOperation) == 0 {
		rootClassification.Properties = []Evidence{bodyEvidence(PropertyLifecycleExecutionProfile, VerdictUnknown, SourceSSA, "no exposed operations resolved for root")}
		return rootClassification, nil
	}
	if len(perOperation) == 1 {
		rootClassification.Admission = perOperation[0].Admission
		rootClassification.Properties = prefixOperationProperties(perOperation[0].Properties, perOperation[0].Operation)
		rootClassification.RefusalCode = perOperation[0].RefusalCode
		return rootClassification, nil
	}

	allLiftable := true
	for _, operation := range perOperation {
		rootClassification.Properties = append(rootClassification.Properties, prefixOperationProperties(operation.Properties, operation.Operation)...)
		if operation.Admission != AdmissionLiftable {
			allLiftable = false
			if rootClassification.RefusalCode == "" {
				rootClassification.RefusalCode = operation.RefusalCode
			}
		}
	}
	sortEvidence(rootClassification.Properties)
	if allLiftable {
		rootClassification.Admission = AdmissionLiftable
		return rootClassification, nil
	}
	rootClassification.Admission = AdmissionRefused
	code := rootClassification.RefusalCode
	if code == "" {
		code = "MLV2_SHAPE_UNSUPPORTED"
	}
	if loaded.RootPragma.Surface == extract.SurfaceStruct {
		code = "MLV2_STRUCT_SURFACE_UNSUPPORTED"
	}
	diag := extract.Diagnostic{
		Code:     code,
		Severity: extract.SeverityError,
		Message:  "not every exposed operation on the root is liftable",
		Span:     loaded.RootPragma.Span,
		RuleIDs:  []string{"AS-STRUCT-2", "TA-REFUSE-1"},
	}
	return rootClassification, []extract.Diagnostic{diag}
}

func propertyHasVerdict(properties []Evidence, id PropertyID, verdict Verdict) bool {
	for _, property := range properties {
		if property.PropertyID == id && property.Verdict == verdict {
			return true
		}
	}
	return false
}

func prefixOperationProperties(properties []Evidence, identity reportv2.SymbolIdentity) []Evidence {
	out := make([]Evidence, 0, len(properties))
	for _, property := range properties {
		cloned := property
		cloned.Detail = fmt.Sprintf("%s: %s", identity.ObjectName, property.Detail)
		out = append(out, cloned)
	}
	return out
}

func dedupeDiagnostics(diags []extract.Diagnostic) []extract.Diagnostic {
	seen := map[string]bool{}
	out := make([]extract.Diagnostic, 0, len(diags))
	for _, diag := range diags {
		key := diag.Code + "|" + diag.Message
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, diag)
	}
	sortExtractDiagnostics(out)
	return out
}
