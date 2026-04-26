package transport

import (
	"sort"
	"strings"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/liftability"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type Template string

const (
	TemplateNone            Template = ""
	TemplateHandler         Template = "handler"
	TemplateHTTPJSON        Template = "http-json"
	TemplateChannelConsumer Template = "channel-consumer"
)

const (
	selectorRuleHandlerWithEvidence    = "handler-with-evidence"
	selectorRuleHandlerWithoutEvidence = "handler-without-evidence-refusal"
	selectorRuleGRPCReserved           = "grpc-reserved-refusal"
	selectorRuleChannelConsumer        = "channel-consumer"
	selectorRuleHTTPJSON               = "http-json"
	selectorRuleNoErrorChannel         = "no-error-channel"
	selectorRuleImplicitHandler        = "implicit-handler"
)

type SelectionInput struct {
	PragmaTransport string
	Properties      []reportv2.PropertyEvidence
	Signals         SelectionSignals
}

type SelectionSignals struct {
	HandlerBoundaryEvidence    []string
	ChannelConsumerEvidence    []string
	BuilderChainEvidence       []string
	CtxRequestResponseEvidence []string
	MultiDomainArgsEvidence    []string
	NoResponseEvidence         []string
}

type Selection struct {
	Shape            Shape
	Template         Template
	DefaultTransport string
	Evidence         []string
	AppliedRules     []string
}

func Select(input SelectionInput) Selection {
	selection := Selection{
		Shape:        ShapeUnsupported,
		Template:     TemplateNone,
		Evidence:     []string{"no transport selector matched the operation boundary"},
		AppliedRules: []string{},
	}

	switch {
	case propertyVerdict(input.Properties, string(liftability.PropertyTransportHandlerBoundary)) == string(liftability.VerdictHold) && len(input.Signals.HandlerBoundaryEvidence) > 0:
		selection.Shape = ShapeHTTPHandler
		selection.Template = TemplateHandler
		selection.DefaultTransport = "handler"
		selection.Evidence = stableEvidence(input.Signals.HandlerBoundaryEvidence)
		selection.AppliedRules = append(selection.AppliedRules, selectorRuleImplicitHandler)
	case propertyVerdict(input.Properties, string(liftability.PropertyLifecycleLongRunningLoop)) == string(liftability.VerdictHold) && len(input.Signals.ChannelConsumerEvidence) > 0:
		selection.Shape = ShapeChannelConsumer
		selection.Template = TemplateChannelConsumer
		selection.Evidence = stableEvidence(input.Signals.ChannelConsumerEvidence)
		selection.AppliedRules = append(selection.AppliedRules, selectorRuleChannelConsumer)
	case propertyVerdict(input.Properties, string(liftability.PropertyTransportReceiverReturnsSelf)) == string(liftability.VerdictViolate) && len(input.Signals.BuilderChainEvidence) > 0:
		selection.Shape = ShapeBuilderChain
		selection.Evidence = stableEvidence(input.Signals.BuilderChainEvidence)
	case propertyVerdict(input.Properties, string(liftability.PropertyContractErrorLast)) == string(liftability.VerdictHold) && len(input.Signals.CtxRequestResponseEvidence) > 0:
		selection.Shape = ShapeCtxRequestResponse
		selection.Evidence = stableEvidence(input.Signals.CtxRequestResponseEvidence)
	case propertyVerdict(input.Properties, string(liftability.PropertyContractErrorLast)) == string(liftability.VerdictHold) && len(input.Signals.MultiDomainArgsEvidence) > 0:
		selection.Shape = ShapeMultiDomainArgs
		selection.Evidence = stableEvidence(input.Signals.MultiDomainArgsEvidence)
	case propertyPresent(input.Properties, string(liftability.PropertyContractErrorLast)) && len(input.Signals.NoResponseEvidence) > 0:
		selection.Shape = ShapeNoResponse
		selection.Evidence = stableEvidence(input.Signals.NoResponseEvidence)
	}

	if isHTTPJSONCandidate(input.Properties, selection.Shape) {
		selection.Template = TemplateHTTPJSON
		selection.DefaultTransport = "http-json"
		selection.AppliedRules = append(selection.AppliedRules, selectorRuleHTTPJSON)
	}
	if shouldFlagNoErrorChannel(input.Properties, selection.Shape) {
		selection.AppliedRules = append(selection.AppliedRules, selectorRuleNoErrorChannel)
	}

	switch input.PragmaTransport {
	case "handler":
		if propertyVerdict(input.Properties, string(liftability.PropertyTransportHandlerBoundary)) == string(liftability.VerdictHold) && len(input.Signals.HandlerBoundaryEvidence) > 0 {
			selection.Shape = ShapeHTTPHandler
			selection.Template = TemplateHandler
			selection.DefaultTransport = "handler"
			selection.Evidence = stableEvidence(input.Signals.HandlerBoundaryEvidence)
			selection.AppliedRules = append(selection.AppliedRules, selectorRuleHandlerWithEvidence)
		} else {
			selection.AppliedRules = append(selection.AppliedRules, selectorRuleHandlerWithoutEvidence)
		}
	case "grpc":
		selection.AppliedRules = append(selection.AppliedRules, selectorRuleGRPCReserved)
	case "http-json":
		if supportsHTTPJSON(selection.Shape) {
			selection.Template = TemplateHTTPJSON
			selection.DefaultTransport = "http-json"
			if !containsRule(selection.AppliedRules, selectorRuleHTTPJSON) {
				selection.AppliedRules = append(selection.AppliedRules, selectorRuleHTTPJSON)
			}
		}
	}

	selection.AppliedRules = uniqueRules(selection.AppliedRules)
	return selection
}

func buildSelectionInput(loaded *extract.LoadedModule, handle operationHandle, lift extract.LiftabilityClassification) SelectionInput {
	input := SelectionInput{
		PragmaTransport: loaded.RootPragma.Options["transport"],
		Properties:      append([]reportv2.PropertyEvidence(nil), lift.Properties...),
		Signals:         SelectionSignals{},
	}
	if evidence, ok := isHTTPHandler(loaded, handle); ok {
		input.Signals.HandlerBoundaryEvidence = evidence
	}
	if evidence, ok := isChannelConsumer(handle); ok {
		input.Signals.ChannelConsumerEvidence = evidence
	}
	if evidence, ok := isBuilderChain(handle.signature); ok {
		input.Signals.BuilderChainEvidence = evidence
	}
	if evidence, ok := isCtxRequestResponse(loaded, handle.signature); ok {
		input.Signals.CtxRequestResponseEvidence = evidence
	}
	if evidence, ok := isMultiDomainArgs(loaded, handle.signature); ok {
		input.Signals.MultiDomainArgsEvidence = evidence
	}
	if evidence, _, ok := isNoResponse(loaded, handle); ok {
		input.Signals.NoResponseEvidence = evidence
	}
	return input
}

func isHTTPJSONCandidate(properties []reportv2.PropertyEvidence, shape Shape) bool {
	if !supportsHTTPJSON(shape) {
		return false
	}
	if propertyVerdict(properties, string(liftability.PropertyBoundarySerializableViaCustomEncoding)) != string(liftability.VerdictHold) {
		return false
	}
	if !propertyHasDetail(properties, string(liftability.PropertyLifecycleExecutionProfile), string(liftability.VerdictHold), "detail=sync-short") {
		return false
	}
	if shape == ShapeNoResponse {
		return propertyPresent(properties, string(liftability.PropertyContractErrorLast))
	}
	return propertyVerdict(properties, string(liftability.PropertyContractErrorLast)) == string(liftability.VerdictHold)
}

func shouldFlagNoErrorChannel(properties []reportv2.PropertyEvidence, shape Shape) bool {
	if shape != ShapeNoResponse {
		return false
	}
	if propertyVerdict(properties, string(liftability.PropertyLifecycleLongRunningLoop)) != string(liftability.VerdictHold) {
		return false
	}
	return propertyVerdict(properties, string(liftability.PropertyContractNoPanicOnlyFailure)) == string(liftability.VerdictViolate)
}

func supportsHTTPJSON(shape Shape) bool {
	return shape == ShapeCtxRequestResponse || shape == ShapeMultiDomainArgs || shape == ShapeNoResponse
}

func propertyHasDetail(properties []reportv2.PropertyEvidence, propertyID, verdict, needle string) bool {
	for _, property := range properties {
		if property.PropertyID == propertyID && property.Verdict == verdict && strings.Contains(property.Detail, needle) {
			return true
		}
	}
	return false
}

func stableEvidence(evidence []string) []string {
	out := append([]string(nil), evidence...)
	sort.Strings(out)
	return out
}

func containsRule(rules []string, rule string) bool {
	for _, candidate := range rules {
		if candidate == rule {
			return true
		}
	}
	return false
}

func uniqueRules(rules []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(rules))
	for _, rule := range rules {
		if seen[rule] {
			continue
		}
		seen[rule] = true
		out = append(out, rule)
	}
	return out
}
