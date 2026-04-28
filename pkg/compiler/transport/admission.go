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

func RegionAdmits(perRootVerdicts map[string]bool, perRootReasons map[string][]string, seams []reportv2.SeamEntry, regionRoots []string) (bool, []string) {
	rootSet := map[string]bool{}
	for _, root := range regionRoots {
		rootSet[root] = true
	}
	var reasons []string
	for root, admitted := range perRootVerdicts {
		if admitted {
			continue
		}
		rootReasons := perRootReasons[root]
		if len(rootReasons) == 0 {
			reasons = append(reasons, fmt.Sprintf("root %s refused", root))
			continue
		}
		for _, reason := range rootReasons {
			reasons = append(reasons, fmt.Sprintf("root %s: %s", root, reason))
		}
	}
	for _, seam := range seams {
		if seam.Type != "ChannelField" {
			continue
		}
		if ok, reason := channelSeamAdmits(seam, rootSet); !ok {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) > 0 {
		return false, reasons
	}
	return true, []string{"admitted by per-root AND plus in-region channel seam check"}
}

func channelSeamAdmits(seam reportv2.SeamEntry, regionRoots map[string]bool) (bool, string) {
	if len(seam.Writers) == 0 || len(seam.Readers) == 0 {
		return false, fmt.Sprintf("channel seam %s has empty endpoint set", seam.Field)
	}
	for _, root := range append(append([]string(nil), seam.Writers...), seam.Readers...) {
		if !regionRoots[root] {
			return false, fmt.Sprintf("channel seam %s endpoint %s escapes the lifted region", seam.Field, root)
		}
	}
	return true, ""
}

func admissionReasonEvidence(reasons []string) []string {
	out := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		out = append(out, "transport admission: "+strings.TrimSpace(reason))
	}
	return out
}
