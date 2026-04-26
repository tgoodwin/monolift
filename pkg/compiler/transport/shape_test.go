package shape

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/extract"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestClassifyHTTPHandlerShapes(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("HTTPHandler", extract.SurfaceStruct, map[string]string{"methods": "ServeHTTP"}, 8))
	if result.Root.Shape != ShapeHTTPHandler {
		t.Fatalf("root shape=%q want %q", result.Root.Shape, ShapeHTTPHandler)
	}
	if got := result.PerOperation[0].Operation.ObjectName; got != "HTTPHandler.ServeHTTP" && got != "(*HTTPHandler).ServeHTTP" {
		t.Fatalf("operation objectName=%q want HTTPHandler.ServeHTTP or (*HTTPHandler).ServeHTTP", got)
	}

	funcResult := classifyFixture(t, fixturePragma("RawHandler", extract.SurfaceFunction, nil, 14))
	if funcResult.Root.Shape != ShapeHTTPHandler {
		t.Fatalf("raw handler shape=%q want %q", funcResult.Root.Shape, ShapeHTTPHandler)
	}

	negative := classifyFixture(t, fixturePragma("BadServe", extract.SurfaceStruct, map[string]string{"methods": "ServeHTTP"}, 16))
	if negative.Root.Shape == ShapeHTTPHandler {
		t.Fatalf("negative shape=%q want not %q", negative.Root.Shape, ShapeHTTPHandler)
	}
}

func TestClassifyCtxRequestResponse(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("RequestReply", extract.SurfaceFunction, nil, 22))
	if result.Root.Shape != ShapeCtxRequestResponse {
		t.Fatalf("shape=%q want %q", result.Root.Shape, ShapeCtxRequestResponse)
	}

	negative := classifyFixture(t, fixturePragma("BadRequestReply", extract.SurfaceFunction, nil, 24))
	if negative.Root.Shape == ShapeCtxRequestResponse {
		t.Fatalf("negative shape=%q want not %q", negative.Root.Shape, ShapeCtxRequestResponse)
	}
}

func TestClassifyMultiDomainArgs(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("ManyArgs", extract.SurfaceFunction, nil, 36))
	if result.Root.Shape != ShapeMultiDomainArgs {
		t.Fatalf("shape=%q want %q", result.Root.Shape, ShapeMultiDomainArgs)
	}
}

func TestClassifyNoResponse(t *testing.T) {
	t.Parallel()

	errorOnly := classifyFixture(t, fixturePragma("ErrorOnly", extract.SurfaceFunction, nil, 28))
	if errorOnly.Root.Shape != ShapeNoResponse {
		t.Fatalf("error-only shape=%q want %q", errorOnly.Root.Shape, ShapeNoResponse)
	}
	if len(errorOnly.Diagnostics) != 0 {
		t.Fatalf("error-only diagnostics=%v want none", errorOnly.Diagnostics)
	}

	empty := classifyFixture(t, fixturePragma("EmptyReturn", extract.SurfaceFunction, nil, 30))
	if empty.Root.Shape != ShapeNoResponse {
		t.Fatalf("empty-return shape=%q want %q", empty.Root.Shape, ShapeNoResponse)
	}
	if len(empty.Diagnostics) != 1 || empty.Diagnostics[0].Code != "MLV2_NO_ERROR_CHANNEL" {
		t.Fatalf("empty-return diagnostics=%v want MLV2_NO_ERROR_CHANNEL", empty.Diagnostics)
	}
}

func TestClassifyBuilderChain(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("Builder", extract.SurfaceStruct, map[string]string{"methods": "WithValue"}, 19))
	if result.Root.Shape != ShapeBuilderChain {
		t.Fatalf("shape=%q want %q", result.Root.Shape, ShapeBuilderChain)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_BUILDER_CHAIN_ROOT" {
		t.Fatalf("diagnostics=%v want MLV2_BUILDER_CHAIN_ROOT", result.Diagnostics)
	}
}

func TestClassifyChannelConsumer(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("Consume", extract.SurfaceFunction, nil, 34))
	if result.Root.Shape != ShapeChannelConsumer {
		t.Fatalf("shape=%q want %q", result.Root.Shape, ShapeChannelConsumer)
	}
}

func TestClassifyUnsupported(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("Unsupported", extract.SurfaceFunction, nil, 40))
	if result.Root.Shape != ShapeUnsupported {
		t.Fatalf("shape=%q want %q", result.Root.Shape, ShapeUnsupported)
	}
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "MLV2_SHAPE_UNSUPPORTED" {
		t.Fatalf("diagnostics=%v want MLV2_SHAPE_UNSUPPORTED", result.Diagnostics)
	}
}

func TestAggregateStructSurfaceMixedHandlerAndDomain(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("MixedSurface", extract.SurfaceStruct, nil, 21))
	if result.Root.Shape != ShapeUnsupported {
		t.Fatalf("root shape=%q want %q", result.Root.Shape, ShapeUnsupported)
	}
	if len(result.Diagnostics) == 0 || result.Diagnostics[len(result.Diagnostics)-1].Code != "MLV2_STRUCT_SURFACE_UNSUPPORTED" {
		t.Fatalf("diagnostics=%v want MLV2_STRUCT_SURFACE_UNSUPPORTED", result.Diagnostics)
	}
}

func TestAggregateStructSurfaceChoosesMostRestrictiveDomainShape(t *testing.T) {
	t.Parallel()

	result := classifyFixture(t, fixturePragma("DomainRoot", extract.SurfaceStruct, nil, 27))
	if result.Root.Shape != ShapeMultiDomainArgs {
		t.Fatalf("root shape=%q want %q", result.Root.Shape, ShapeMultiDomainArgs)
	}
}

func TestValidateTransportAgainstShape(t *testing.T) {
	t.Parallel()

	req := fixturePragma("RequestReply", extract.SurfaceFunction, map[string]string{"transport": "grpc"}, 32)
	loaded, root, result := classifyFixtureForExtract(t, req)
	diagnostics := ValidatePragmaOptions(loaded, root, result)
	if len(diagnostics) != 1 || diagnostics[0].Code != "MLV2_TRANSPORT_RESERVED" {
		t.Fatalf("diagnostics=%v want MLV2_TRANSPORT_RESERVED", diagnostics)
	}

	req = fixturePragma("RequestReply", extract.SurfaceFunction, map[string]string{"transport": "handler"}, 32)
	loaded, root, result = classifyFixtureForExtract(t, req)
	diagnostics = ValidatePragmaOptions(loaded, root, result)
	if len(diagnostics) != 1 || diagnostics[0].Code != "MLV2_SHAPE_UNSUPPORTED" {
		t.Fatalf("diagnostics=%v want MLV2_SHAPE_UNSUPPORTED", diagnostics)
	}
	if !reflect.DeepEqual(diagnostics[0].RuleIDs, []string{"TA-HANDLER-1"}) {
		t.Fatalf("ruleIds=%v want [TA-HANDLER-1]", diagnostics[0].RuleIDs)
	}
}

func TestValidateStateAffinityKey(t *testing.T) {
	t.Parallel()

	req := fixturePragma("RequestReply", extract.SurfaceFunction, map[string]string{"state": "affinity"}, 32)
	loaded, root, result := classifyFixtureForExtract(t, req)
	diagnostics := ValidatePragmaOptions(loaded, root, result)
	if len(diagnostics) != 1 || diagnostics[0].Code != "MLV2_SESSION_AFFINITY_UNAVAILABLE" {
		t.Fatalf("diagnostics=%v want MLV2_SESSION_AFFINITY_UNAVAILABLE", diagnostics)
	}
}

func TestValidateMethodsAgainstRoot(t *testing.T) {
	t.Parallel()

	structReq := extract.Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []extract.Pragma{{
			Name:     "handler-root",
			Surface:  extract.SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options:  map[string]string{"methods": "Missing"},
			Span: extract.Span{
				Filename: filepath.Join("..", "testdata", "rootresolve", "root.go"),
				Line:     3,
				EndLine:  3,
			},
		}},
	}
	loaded, root, result := classifyFixtureForExtract(t, structReq)
	diagnostics := ValidatePragmaOptions(loaded, root, result)
	if len(diagnostics) != 1 || diagnostics[0].Code != "MLV2_STRUCT_SURFACE_UNSUPPORTED" {
		t.Fatalf("struct diagnostics=%v want MLV2_STRUCT_SURFACE_UNSUPPORTED", diagnostics)
	}

	ifaceReq := extract.Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []extract.Pragma{{
			Name:     "composite-root",
			Surface:  extract.SurfaceInterface,
			DeclName: "Composite",
			DeclKind: "interface",
			Options:  map[string]string{"methods": "Missing"},
			Span: extract.Span{
				Filename: filepath.Join("..", "testdata", "rootresolve", "root.go"),
				Line:     17,
				EndLine:  20,
			},
		}},
	}
	loaded, root, result = classifyFixtureForExtract(t, ifaceReq)
	diagnostics = ValidatePragmaOptions(loaded, root, result)
	if len(diagnostics) != 1 || diagnostics[0].Code != "MLV2_SHAPE_UNSUPPORTED" {
		t.Fatalf("interface diagnostics=%v want MLV2_SHAPE_UNSUPPORTED", diagnostics)
	}
}

func TestClassifyDeterministic(t *testing.T) {
	t.Parallel()

	req := fixturePragma("HTTPHandler", extract.SurfaceStruct, nil, 8)
	first := classifyFixture(t, req)
	second := classifyFixture(t, req)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("classify result differs between runs\nfirst=%#v\nsecond=%#v", first, second)
	}
}

func TestClassifyCaddyReverseProxyRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("SSA-heavy; load real evaluation corpus")
	}
	t.Parallel()

	req := extract.Request{
		Sources: []string{filepath.Join("..", "..", "..", "evaluation", "caddy")},
		Pragmas: []extract.Pragma{{
			Name:     "caddy-reverse-proxy",
			Surface:  extract.SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "type",
			Options: map[string]string{
				"name":     "caddy-reverse-proxy",
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: extract.Span{
				Filename: filepath.Join("..", "..", "..", "evaluation", "caddy", "modules", "caddyhttp", "reverseproxy", "reverseproxy.go"),
				Line:     101,
				EndLine:  101,
			},
		}},
	}
	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	program, err := extract.BuildProgram(loaded)
	if err != nil {
		t.Fatalf("BuildProgram: %v", err)
	}
	analyzed, err := extract.Analyze(req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	result, err := Classify(loaded, program, analyzed.Report.Root)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	if result.Root.Shape != ShapeHTTPHandler {
		t.Fatalf("root shape=%q want %q", result.Root.Shape, ShapeHTTPHandler)
	}
	if result.Root.DefaultTransport != "handler" {
		t.Fatalf("default transport=%q want handler", result.Root.DefaultTransport)
	}
	if !containsEvidence(result.Root.Evidence, "caddyhttp.MiddlewareHandler") {
		t.Fatalf("root evidence=%v want caddyhttp.MiddlewareHandler", result.Root.Evidence)
	}
}

func classifyFixture(t *testing.T, req extract.Request) Result {
	t.Helper()

	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	program, err := extract.BuildProgram(loaded)
	if err != nil {
		t.Fatalf("BuildProgram: %v", err)
	}
	analyzed, err := extract.Analyze(req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	result, err := Classify(loaded, program, analyzed.Report.Root)
	if err != nil {
		t.Fatalf("Classify: %v", err)
	}
	return result
}

func classifyFixtureForExtract(t *testing.T, req extract.Request) (*extract.LoadedModule, reportv2.Root, extract.ShapeResult) {
	t.Helper()

	loaded, err := extract.LoadModule(req)
	if err != nil {
		t.Fatalf("LoadModule: %v", err)
	}
	program, err := extract.BuildProgram(loaded)
	if err != nil {
		t.Fatalf("BuildProgram: %v", err)
	}
	analyzed, err := extract.Analyze(req)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	result, err := ForExtract(loaded, program, analyzed.Report.Root)
	if err != nil {
		t.Fatalf("ForExtract: %v", err)
	}
	return loaded, analyzed.Report.Root, result
}

func fixturePragma(decl string, surface extract.Surface, options map[string]string, line int) extract.Request {
	rootFile := filepath.Join("testdata", "fixtures", "root.go")
	return extract.Request{
		Sources: []string{filepath.Join("testdata", "fixtures")},
		Pragmas: []extract.Pragma{{
			Name:     decl,
			Surface:  surface,
			DeclName: decl,
			DeclKind: string(surface),
			Options:  options,
			Span: extract.Span{
				Filename: rootFile,
				Line:     line,
				EndLine:  line,
			},
		}},
	}
}

func containsEvidence(evidence []string, needle string) bool {
	for _, entry := range evidence {
		if strings.Contains(entry, needle) {
			return true
		}
	}
	return false
}
