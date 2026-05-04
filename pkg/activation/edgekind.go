package activation

import "strings"

// EdgeKind is the canonical activation edge taxonomy used by the analyzer and
// evaluator. The values are stable because evaluation reports serialize them.
type EdgeKind string

const (
	DirectCall                   EdgeKind = "DirectCall"
	ConcreteMethodCall           EdgeKind = "ConcreteMethodCall"
	StructFieldFuncValue         EdgeKind = "StructFieldFuncValue"
	PackageVarFuncValue          EdgeKind = "PackageVarFuncValue"
	MapFuncValue                 EdgeKind = "MapFuncValue"
	InterfaceDispatch            EdgeKind = "InterfaceDispatch"
	GoroutineLaunch              EdgeKind = "GoroutineLaunch"
	HTTPHandlerRegistration      EdgeKind = "HTTPHandlerRegistration"
	ChannelFlow                  EdgeKind = "ChannelFlow"
	ClosureCapture               EdgeKind = "ClosureCapture"
	CallbackRegistration         EdgeKind = "CallbackRegistration"
	StructLiteralFieldAssignment EdgeKind = "StructLiteralFieldAssignment"
	Unsupported                  EdgeKind = "Unsupported"
)

// EdgeKindMapping records how a synthesis trace edge string was normalized.
type EdgeKindMapping struct {
	Raw        string
	Kind       EdgeKind
	Diagnostic string
}

var exactTraceEdgeKinds = map[string]EdgeKind{
	"direct-function-call":                                   DirectCall,
	"direct-function-call (chained)":                         DirectCall,
	"method-call-on-concrete-type":                           ConcreteMethodCall,
	"method-call-on-concrete-field-type":                     ConcreteMethodCall,
	"method-value-call":                                      ConcreteMethodCall,
	"promoted-method-call-through-embedded-field":            ConcreteMethodCall,
	"type-asserted-method-call":                              ConcreteMethodCall,
	"interface-method-dispatch":                              InterfaceDispatch,
	"map-lookup-interface-method-dispatch":                   InterfaceDispatch,
	"closure-captured-interface-dispatch":                    InterfaceDispatch,
	"function-value-in-struct-field":                         StructFieldFuncValue,
	"function-value-via-struct-field":                        StructFieldFuncValue,
	"function-value-stored-in-struct-field":                  StructFieldFuncValue,
	"indirect-call-through-struct-field":                     StructFieldFuncValue,
	"init-time-function-field-dispatch":                      StructFieldFuncValue,
	"interface-typed-struct-field-write":                     StructFieldFuncValue,
	"struct-literal-field-assignment":                        StructLiteralFieldAssignment,
	"goroutine-launch":                                       GoroutineLaunch,
	"goroutine-launch-of-concrete-method":                    GoroutineLaunch,
	"goroutine-launch-on-concrete-method":                    GoroutineLaunch,
	"goroutine-launch-of-named-function":                     GoroutineLaunch,
	"goroutine-launch-of-closure":                            GoroutineLaunch,
	"goroutine-launched-closure":                             GoroutineLaunch,
	"goroutine-launch-of-anonymous-closure":                  GoroutineLaunch,
	"goroutine-launch-with-closure":                          GoroutineLaunch,
	"goroutine-launch-with-closure-capture":                  GoroutineLaunch,
	"channel-send-receive":                                   ChannelFlow,
	"channel-receive-to-concrete-method-call":                ChannelFlow,
	"channel-receive-type-switch":                            ChannelFlow,
	"channel-typed-flow":                                     ChannelFlow,
	"asynchronous-queue-handoff":                             ChannelFlow,
	"http-handler-registration":                              HTTPHandlerRegistration,
	"http-server-handler-dispatch":                           HTTPHandlerRegistration,
	"http-handler-registration-via-wrapper-closure":          HTTPHandlerRegistration,
	"method-value-handler-registration":                      HTTPHandlerRegistration,
	"callback-registration":                                  CallbackRegistration,
	"registered-callback-dispatch":                           CallbackRegistration,
	"callback-argument-dispatch":                             CallbackRegistration,
	"method-value-as-callback":                               CallbackRegistration,
	"closure-callback-registration":                          CallbackRegistration,
	"closure-passed-as-callback-arg":                         CallbackRegistration,
	"callback-registration (method-value + closure-wrapper)": CallbackRegistration,
	"closure-capture":                                        ClosureCapture,
	"closure-capture-of-interface-value":                     ClosureCapture,
	"closure-capture-of-struct-field":                        ClosureCapture,
	"closure-capture-into-struct-field":                      ClosureCapture,
	"closure-captured-function-value":                        ClosureCapture,
	"closure-captured-function-call":                         ClosureCapture,
	"closure-passed-as-argument":                             ClosureCapture,
	"closure-passed-as-variadic-argument":                    ClosureCapture,
	"local-closure-call":                                     ClosureCapture,
	"immediate-closure-callback":                             ClosureCapture,
	"function-value-as-argument":                             CallbackRegistration,
	"function-value-passed-as-argument":                      CallbackRegistration,
	"function-value-as-parameter":                            CallbackRegistration,
	"function-passed-as-parameter-then-invoked":              CallbackRegistration,
	"function-value-call-via-parameter":                      CallbackRegistration,
	"function-value-parameter-call":                          CallbackRegistration,
	"function-value-parameter-invocation":                    CallbackRegistration,
	"function-value-argument-call":                           CallbackRegistration,
	"function-value-as-argument-stored-in-struct-field":      StructFieldFuncValue,
	"call-through-package-level-function-variable":           PackageVarFuncValue,
	"package-level-function-variable-call":                   PackageVarFuncValue,
	"package-level-var-dispatch":                             PackageVarFuncValue,
	"map-indexed-function-value-call":                        MapFuncValue,
	"map-keyed-function-value-call":                          MapFuncValue,
	"function-value-in-map":                                  MapFuncValue,
}

// CanonicalizeTraceEdgeKind maps edge_type strings from the SPRINT-0034 JSON
// traces into the canonical taxonomy. Unknown long-tail variants become
// Unsupported and carry a diagnostic that reports the original edge string.
func CanonicalizeTraceEdgeKind(raw string) EdgeKindMapping {
	cleaned := cleanTraceEdgeKind(raw)
	if cleaned == "" || cleaned == "entrypoint" {
		return EdgeKindMapping{Raw: raw, Kind: DirectCall}
	}
	if kind, ok := exactTraceEdgeKinds[cleaned]; ok {
		return EdgeKindMapping{Raw: raw, Kind: kind}
	}

	switch {
	case strings.Contains(cleaned, "http-handler") || strings.Contains(cleaned, "handler-registration"):
		return EdgeKindMapping{Raw: raw, Kind: HTTPHandlerRegistration}
	case strings.Contains(cleaned, "callback"):
		return EdgeKindMapping{Raw: raw, Kind: CallbackRegistration}
	case strings.Contains(cleaned, "struct-literal-field-assignment"):
		return EdgeKindMapping{Raw: raw, Kind: StructLiteralFieldAssignment}
	case strings.Contains(cleaned, "struct-field") || strings.Contains(cleaned, "function-field"):
		return EdgeKindMapping{Raw: raw, Kind: StructFieldFuncValue}
	case strings.Contains(cleaned, "package-level") || strings.Contains(cleaned, "global-variable"):
		return EdgeKindMapping{Raw: raw, Kind: PackageVarFuncValue}
	case strings.Contains(cleaned, "map-indexed") || strings.Contains(cleaned, "map-keyed") || strings.Contains(cleaned, "registry"):
		return EdgeKindMapping{Raw: raw, Kind: MapFuncValue}
	case strings.Contains(cleaned, "channel") || strings.Contains(cleaned, "queue"):
		return EdgeKindMapping{Raw: raw, Kind: ChannelFlow}
	case strings.Contains(cleaned, "goroutine"):
		return EdgeKindMapping{Raw: raw, Kind: GoroutineLaunch}
	case strings.Contains(cleaned, "closure"):
		return EdgeKindMapping{Raw: raw, Kind: ClosureCapture}
	case strings.Contains(cleaned, "interface"):
		return EdgeKindMapping{Raw: raw, Kind: InterfaceDispatch}
	case strings.Contains(cleaned, "concrete"):
		return EdgeKindMapping{Raw: raw, Kind: ConcreteMethodCall}
	case strings.Contains(cleaned, "direct"):
		return EdgeKindMapping{Raw: raw, Kind: DirectCall}
	default:
		return EdgeKindMapping{
			Raw:        raw,
			Kind:       Unsupported,
			Diagnostic: "unsupported synthesis trace edge type: " + raw,
		}
	}
}

func cleanTraceEdgeKind(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, "`")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ReplaceAll(cleaned, " + ", "+")
	cleaned = strings.ReplaceAll(cleaned, " +", "+")
	cleaned = strings.ReplaceAll(cleaned, "+ ", "+")
	return cleaned
}
