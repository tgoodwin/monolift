package transport

import (
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestAdmitPositiveStringBoolToString(t *testing.T) {
	t.Parallel()

	admitted, reasons := Admit(admissibleProperties())
	if !admitted {
		t.Fatalf("admitted=false reasons=%v", reasons)
	}
}

func TestAdmitNegatives(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		properties []reportv2.PropertyEvidence
		wantReason string
	}{
		{
			name:       "func typed param",
			properties: withVerdict(admissibleProperties(), liftability.PropertyBoundaryNoCallableValues, "Violate", "boundary contains func value"),
			wantReason: "boundary.no-callable-values",
		},
		{
			name:       "channel result",
			properties: withVerdict(admissibleProperties(), liftability.PropertyBoundaryNoStreamingValues, "Violate", "boundary contains channel"),
			wantReason: "boundary.no-streaming-values",
		},
		{
			name:       "type parameter result",
			properties: withVerdict(admissibleProperties(), liftability.PropertyBoundaryFullyInstantiated, "Violate", "type parameter remains"),
			wantReason: "boundary.fully-instantiated",
		},
		{
			name:       "sync primitive arg",
			properties: withVerdict(admissibleProperties(), liftability.PropertyBoundaryNoSyncPrimitives, "Violate", "boundary contains sync.Mutex"),
			wantReason: "boundary.no-sync-primitives",
		},
		{
			name:       "variadic",
			properties: withVerdict(admissibleProperties(), liftability.PropertyBoundaryVariadicFree, "Violate", "signature is variadic"),
			wantReason: "boundary.variadic-free",
		},
		{
			name:       "missing sync short detail default deny",
			properties: withoutProperty(admissibleProperties(), liftability.PropertyLifecycleExecutionProfile),
			wantReason: "sync-short",
		},
		{
			name:       "long running execution profile",
			properties: withDetail(admissibleProperties(), liftability.PropertyLifecycleExecutionProfile, "detail=long-running"),
			wantReason: "sync-short",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admitted, reasons := Admit(tc.properties)
			if admitted {
				t.Fatalf("admitted=true reasons=%v", reasons)
			}
			if !containsSubstring(reasons, tc.wantReason) {
				t.Fatalf("reasons=%v want substring %q", reasons, tc.wantReason)
			}
		})
	}
}

func TestRegionAdmitsInRegionChannelSeam(t *testing.T) {
	t.Parallel()

	admitted, reasons := RegionAdmits(
		map[string]bool{"Hub": true, "WebConn": true},
		nil,
		[]reportv2.SeamEntry{{
			Type:    "ChannelField",
			Field:   "WebConn.send",
			Writers: []string{"Hub"},
			Readers: []string{"WebConn"},
		}},
		[]string{"Hub", "WebConn"},
	)
	if !admitted {
		t.Fatalf("admitted=false reasons=%v", reasons)
	}
}

func TestRegionAdmitsRejectsEscapingChannelSeam(t *testing.T) {
	t.Parallel()

	admitted, reasons := RegionAdmits(
		map[string]bool{"Hub": true, "WebConn": true},
		nil,
		[]reportv2.SeamEntry{{
			Type:    "ChannelField",
			Field:   "WebConn.send",
			Writers: []string{"Hub"},
			Readers: []string{"Other"},
		}},
		[]string{"Hub", "WebConn"},
	)
	if admitted || len(reasons) == 0 {
		t.Fatalf("admitted=%v reasons=%v, want refusal", admitted, reasons)
	}
}

func admissibleProperties() []reportv2.PropertyEvidence {
	return []reportv2.PropertyEvidence{
		property(string(liftability.PropertyBoundarySerializableViaCustomEncoding), "body", "Hold", "types", "boundary is serializable"),
		property(string(liftability.PropertyBoundaryNoCallableValues), "body", "Hold", "types", "no func-typed values"),
		property(string(liftability.PropertyBoundaryNoStreamingValues), "body", "Hold", "types", "no channel or stream values"),
		property(string(liftability.PropertyBoundaryFullyInstantiated), "body", "Hold", "types", "fully instantiated"),
		property(string(liftability.PropertyBoundaryVariadicFree), "body", "Hold", "types", "not variadic"),
		property(string(liftability.PropertyBoundaryNoSyncPrimitives), "body", "Hold", "types", "no sync primitives"),
		property(string(liftability.PropertyLifecycleExecutionProfile), "body", "Hold", "ssa", "detail=sync-short"),
	}
}

func withVerdict(props []reportv2.PropertyEvidence, propertyID liftability.PropertyID, verdict, detail string) []reportv2.PropertyEvidence {
	out := append([]reportv2.PropertyEvidence(nil), props...)
	for i := range out {
		if out[i].PropertyID == string(propertyID) {
			out[i].Verdict = verdict
			out[i].Detail = detail
			return out
		}
	}
	return append(out, property(string(propertyID), "body", verdict, "types", detail))
}

func withDetail(props []reportv2.PropertyEvidence, propertyID liftability.PropertyID, detail string) []reportv2.PropertyEvidence {
	out := append([]reportv2.PropertyEvidence(nil), props...)
	for i := range out {
		if out[i].PropertyID == string(propertyID) {
			out[i].Detail = detail
			return out
		}
	}
	return out
}

func withoutProperty(props []reportv2.PropertyEvidence, propertyID liftability.PropertyID) []reportv2.PropertyEvidence {
	out := make([]reportv2.PropertyEvidence, 0, len(props))
	for _, prop := range props {
		if prop.PropertyID != string(propertyID) {
			out = append(out, prop)
		}
	}
	return out
}

func containsSubstring(items []string, needle string) bool {
	for _, item := range items {
		if strings.Contains(item, needle) {
			return true
		}
	}
	return false
}
