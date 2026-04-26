package transport

import (
	"fmt"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

var admissionRequiredHoldProperties = []liftability.PropertyID{
	liftability.PropertyBoundarySerializableViaCustomEncoding,
	liftability.PropertyBoundaryNoCallableValues,
	liftability.PropertyBoundaryNoStreamingValues,
	liftability.PropertyBoundaryFullyInstantiated,
	liftability.PropertyBoundaryVariadicFree,
	liftability.PropertyBoundaryNoSyncPrimitives,
}

// Admit applies the v0 transport admission rule for simple sync HTTP/JSON lifts.
func Admit(props []reportv2.PropertyEvidence) (admitted bool, reasons []string) {
	for _, propertyID := range admissionRequiredHoldProperties {
		if propertyVerdict(props, string(propertyID)) != string(liftability.VerdictHold) {
			reasons = append(reasons, fmt.Sprintf("%s is not Hold", propertyID))
		}
	}

	if !propertyHasDetail(props, string(liftability.PropertyLifecycleExecutionProfile), string(liftability.VerdictHold), "sync-short") {
		reasons = append(reasons, "lifecycle.execution-profile is not Hold with sync-short detail")
	}

	if len(reasons) > 0 {
		return false, reasons
	}
	return true, []string{"admitted by transport admission v0"}
}

func admissionReasonEvidence(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, "transport admission: "+strings.TrimSpace(reason))
	}
	return out
}
