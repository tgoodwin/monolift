package transport

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestSelectHTTPJSONForCtxRequestResponseProfile(t *testing.T) {
	t.Parallel()

	selection := Select(SelectionInput{
		Properties: []reportv2.PropertyEvidence{
			property(string(liftability.PropertyBoundaryContextFirst), "body", "Hold", "types", "first parameter is context.Context"),
			property(string(liftability.PropertyBoundarySerializableViaCustomEncoding), "body", "Hold", "types", "boundary is serializable"),
			property(string(liftability.PropertyContractErrorLast), "body", "Hold", "types", "terminal result is error"),
			property(string(liftability.PropertyLifecycleExecutionProfile), "body", "Hold", "ssa", "detail=sync-short"),
		},
		Signals: SelectionSignals{
			CtxRequestResponseEvidence: []string{"signature matches func(context.Context, T) (U, error)"},
		},
	})

	if selection.Shape != ShapeCtxRequestResponse {
		t.Fatalf("shape=%q want %q", selection.Shape, ShapeCtxRequestResponse)
	}
	if selection.Template != TemplateHTTPJSON {
		t.Fatalf("template=%q want %q", selection.Template, TemplateHTTPJSON)
	}
	if selection.DefaultTransport != "http-json" {
		t.Fatalf("defaultTransport=%q want http-json", selection.DefaultTransport)
	}
	if !containsRule(selection.AppliedRules, selectorRuleHTTPJSON) {
		t.Fatalf("appliedRules=%v want %q", selection.AppliedRules, selectorRuleHTTPJSON)
	}
}

func TestSelectorRuleCoverageGate(t *testing.T) {
	cases := []struct {
		name     string
		input    SelectionInput
		wantRule string
	}{
		{
			name: "handler with evidence",
			input: SelectionInput{
				PragmaTransport: "handler",
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyTransportHandlerBoundary), "body", "Hold", "types", "signature matches net/http handler"),
				},
				Signals: SelectionSignals{
					HandlerBoundaryEvidence: []string{"signature matches net/http handler"},
				},
			},
			wantRule: selectorRuleHandlerWithEvidence,
		},
		{
			name: "handler without evidence refusal",
			input: SelectionInput{
				PragmaTransport: "handler",
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyContractErrorLast), "body", "Hold", "types", "terminal result is error"),
				},
				Signals: SelectionSignals{
					CtxRequestResponseEvidence: []string{"signature matches func(context.Context, T) (U, error)"},
				},
			},
			wantRule: selectorRuleHandlerWithoutEvidence,
		},
		{
			name: "grpc reserved refusal",
			input: SelectionInput{
				PragmaTransport: "grpc",
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyContractErrorLast), "body", "Hold", "types", "terminal result is error"),
				},
				Signals: SelectionSignals{
					CtxRequestResponseEvidence: []string{"signature matches func(context.Context, T) (U, error)"},
				},
			},
			wantRule: selectorRuleGRPCReserved,
		},
		{
			name: "channel consumer",
			input: SelectionInput{
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyLifecycleLongRunningLoop), "body", "Hold", "ssa", "loop back-edge with receive/select observed"),
				},
				Signals: SelectionSignals{
					ChannelConsumerEvidence: []string{"ssa contains channel receive within a loop without channel-typed boundary values"},
				},
			},
			wantRule: selectorRuleChannelConsumer,
		},
		{
			name: "http json",
			input: SelectionInput{
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyBoundarySerializableViaCustomEncoding), "body", "Hold", "types", "boundary is serializable"),
					property(string(liftability.PropertyContractErrorLast), "body", "Hold", "types", "terminal result is error"),
					property(string(liftability.PropertyLifecycleExecutionProfile), "body", "Hold", "ssa", "detail=sync-short"),
				},
				Signals: SelectionSignals{
					CtxRequestResponseEvidence: []string{"signature matches func(context.Context, T) (U, error)"},
				},
			},
			wantRule: selectorRuleHTTPJSON,
		},
		{
			name: "no error channel",
			input: SelectionInput{
				Properties: []reportv2.PropertyEvidence{
					property(string(liftability.PropertyContractErrorLast), "body", "Violate", "types", "signature has no results"),
					property(string(liftability.PropertyContractNoPanicOnlyFailure), "body", "Violate", "ssa", "panic path exists without terminal error result"),
					property(string(liftability.PropertyLifecycleLongRunningLoop), "body", "Hold", "ssa", "loop back-edge with receive/select observed"),
				},
				Signals: SelectionSignals{
					NoResponseEvidence: []string{"signature returns no values"},
				},
			},
			wantRule: selectorRuleNoErrorChannel,
		},
	}

	covered := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selection := Select(tc.input)
			if !containsRule(selection.AppliedRules, tc.wantRule) {
				t.Fatalf("appliedRules=%v want %q", selection.AppliedRules, tc.wantRule)
			}
			covered[tc.wantRule] = true
		})
	}

	wantRules := []string{
		selectorRuleHandlerWithEvidence,
		selectorRuleHandlerWithoutEvidence,
		selectorRuleGRPCReserved,
		selectorRuleChannelConsumer,
		selectorRuleHTTPJSON,
		selectorRuleNoErrorChannel,
	}
	for _, rule := range wantRules {
		if !covered[rule] {
			t.Fatalf("selector rule %q is uncovered", rule)
		}
	}
}

func property(id, subject, verdict, source, detail string) reportv2.PropertyEvidence {
	return reportv2.PropertyEvidence{
		PropertyID: id,
		Subject:    subject,
		Verdict:    verdict,
		Source:     source,
		Detail:     detail,
	}
}
