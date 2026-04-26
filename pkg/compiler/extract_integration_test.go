package compiler

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestExtractCaddyReverseProxyProducesNonEmptyValidatedReport(t *testing.T) {
	if testing.Short() {
		t.Skip("SSA-heavy; load real evaluation corpus")
	}
	t.Parallel()

	sources := []string{filepath.Join("..", "..", "evaluation", "caddy")}
	pragmas, diagnostics, err := Parse(sources)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("Parse diagnostics = %+v, want none", diagnostics)
	}

	report, extractDiagnostics, err := Extract(sources, pragmas)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("report diagnostics = %+v, want none", report.Diagnostics)
	}
	if got := report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}

	gotDiagnostics := sortedDiagnosticFacts(extractDiagnostics)
	wantDiagnostics := []diagnosticFact{
		{Code: "MLV2_CHANNEL_BOUNDARY", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "channel-typed boundary value"},
		{Code: "MLV2_REFLECTION_DISPATCH", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "reachable symbol reflect.Addr"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "channel values are not serializable"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "function values are not serializable"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync.Mutex"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync.Once"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync.RWMutex"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync/atomic.Bool"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync/atomic.Int32"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync/atomic.Int64"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync/atomic.align64"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "sync primitive sync/atomic.noCopy"},
		{Code: "MLV2_SERIALIZATION_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "unsafe.Pointer is not serializable"},
		{Code: "MLV2_SHAPE_UNSUPPORTED", Span: "evaluation/caddy/modules/caddyhttp/reverseproxy/reverseproxy.go:101", Message: "func-typed boundary value"},
	}
	if !reflect.DeepEqual(gotDiagnostics, wantDiagnostics) {
		t.Fatalf("extract diagnostics = %#v, want %#v", gotDiagnostics, wantDiagnostics)
	}
	if got, want := distinctDiagnosticCodes(extractDiagnostics), []string{
		"MLV2_CHANNEL_BOUNDARY",
		"MLV2_REFLECTION_DISPATCH",
		"MLV2_SERIALIZATION_UNSUPPORTED",
		"MLV2_SHAPE_UNSUPPORTED",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("distinct diagnostic codes = %v, want %v", got, want)
	}
	assertNoDuplicateDiagnosticFacts(t, extractDiagnostics)

	if got := report.Root.Identity; got.ModulePath != "github.com/caddyserver/caddy/v2" ||
		got.PackagePath != "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy" ||
		got.ObjectName != "Handler" || got.Kind != "type" {
		t.Fatalf("root identity = %+v, want reverseproxy.Handler type", got)
	}
	if report.Root.Shape != "http-handler" {
		t.Fatalf("root.shape = %q, want http-handler", report.Root.Shape)
	}
	if report.Root.DefaultTransport != "handler" {
		t.Fatalf("root.defaultTransport = %q, want handler", report.Root.DefaultTransport)
	}
	if len(report.Closure.IncludedSymbols) == 0 {
		t.Fatal("closure.includedSymbols is empty")
	}
	if len(report.ExternalDeps) == 0 {
		t.Fatal("externalDependencies is empty")
	}
	if got, want := adapterByKind(report.Adapters, "handler").ID, "caddy-middleware-handler"; got != want {
		t.Fatalf("handler adapter id = %q, want %q", got, want)
	}
	registryAdapter := adapterByKind(report.Adapters, "registry")
	if !reflect.DeepEqual(registryAdapter.CanonicalShapes, []string{"http-handler"}) {
		t.Fatalf("registry canonical shapes = %v, want [http-handler]", registryAdapter.CanonicalShapes)
	}
	if len(report.State) != 1 {
		t.Fatalf("state rows = %v, want one aggregated inferred row", report.State)
	}
	if got := report.State[0]; !reflect.DeepEqual(got.Classes, []string{"immutable-captured-config"}) || got.Disposition != "replicated" {
		t.Fatalf("state row = %+v, want immutable-captured-config replicated", got)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := reportv2.Validate(data); err != nil {
		t.Fatalf("reportv2.Validate: %v", err)
	}
}

func TestExtractPocketBaseRefusesForEmbeddedDBAndClosureSize(t *testing.T) {
	if testing.Short() {
		t.Skip("SSA-heavy; load real evaluation corpus")
	}
	t.Parallel()

	sources := []string{filepath.Join("..", "..", "evaluation", "pocketbase")}
	pragmas, diagnostics, err := Parse(sources)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("Parse diagnostics = %+v, want none", diagnostics)
	}

	report, extractDiagnostics, err := Extract(sources, pragmas)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}

	if got, want := sortedDiagnosticFacts(extractDiagnostics), []diagnosticFact{
		{Code: "MLV2_CLOSURE_TOO_LARGE", Span: "evaluation/pocketbase/core/app.go:29", Message: "root closure exceeds the bounded precision threshold"},
		{Code: "MLV2_EMBEDDED_DB_APP_ROOT", Span: "evaluation/pocketbase/core/app.go:29", Message: "embedded database app root selected as lift root"},
		{Code: "MLV2_NO_ERROR_CHANNEL", Span: "evaluation/pocketbase/core/app.go:29", Message: "not every exposed operation on the root is liftable"},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("extract diagnostics = %#v, want %#v", got, want)
	}
	if got, want := distinctDiagnosticCodes(extractDiagnostics), []string{"MLV2_CLOSURE_TOO_LARGE", "MLV2_EMBEDDED_DB_APP_ROOT", "MLV2_NO_ERROR_CHANNEL"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("distinct diagnostic codes = %v, want %v", got, want)
	}
	assertNoDuplicateDiagnosticFacts(t, extractDiagnostics)
	gotRows := []string{}
	for _, item := range report.State {
		gotRows = append(gotRows, item.Symbol.ObjectName+"="+item.Disposition)
	}
	wantRows := []string{
		"BaseApp.auxConcurrentDB=refused",
		"BaseApp.auxNonconcurrentDB=refused",
		"BaseApp.concurrentDB=refused",
		"BaseApp.nonconcurrentDB=refused",
	}
	if !reflect.DeepEqual(gotRows, wantRows) {
		t.Fatalf("state rows = %v, want %v", gotRows, wantRows)
	}
}

func TestExtractTransportHandlerMismatchRefusesWithShapeUnsupportedOnly(t *testing.T) {
	t.Parallel()

	sources := []string{filepath.Join("..", "..", "test", "e2e", "targets", "pragma", "fixtures", "shape-transport-handler-mismatch")}
	pragmas, diagnostics, err := Parse(sources)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("Parse diagnostics = %+v, want none", diagnostics)
	}

	report, extractDiagnostics, err := Extract(sources, pragmas)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}

	gotCodes := []string{}
	for _, diagnostic := range extractDiagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	if !reflect.DeepEqual(gotCodes, []string{"MLV2_SHAPE_UNSUPPORTED"}) {
		t.Fatalf("extract diagnostics = %v, want [MLV2_SHAPE_UNSUPPORTED]", gotCodes)
	}
}

func adapterByKind(adapters []reportv2.Adapter, kind string) reportv2.Adapter {
	for _, adapter := range adapters {
		if adapter.Kind == kind {
			return adapter
		}
	}
	return reportv2.Adapter{}
}

type diagnosticFact struct {
	Code    string
	Span    string
	Message string
}

func sortedDiagnosticFacts(diagnostics []Diagnostic) []diagnosticFact {
	out := make([]diagnosticFact, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnosticFact{
			Code:    diagnostic.Code,
			Span:    normalizedDiagnosticSpan(diagnostic),
			Message: diagnostic.Message,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if out[i].Span != out[j].Span {
			return out[i].Span < out[j].Span
		}
		return out[i].Message < out[j].Message
	})
	return out
}

func distinctDiagnosticCodes(diagnostics []Diagnostic) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if seen[diagnostic.Code] {
			continue
		}
		seen[diagnostic.Code] = true
		out = append(out, diagnostic.Code)
	}
	sort.Strings(out)
	return out
}

func assertNoDuplicateDiagnosticFacts(t *testing.T, diagnostics []Diagnostic) {
	t.Helper()
	counts := map[diagnosticFact]int{}
	for _, fact := range sortedDiagnosticFacts(diagnostics) {
		counts[fact]++
	}
	for fact, count := range counts {
		if count != 1 {
			t.Fatalf("diagnostic fact %+v count=%d, want 1", fact, count)
		}
	}
}

func normalizedDiagnosticSpan(d Diagnostic) string {
	path := filepath.ToSlash(d.Span.Filename)
	for strings.HasPrefix(path, "../") {
		path = strings.TrimPrefix(path, "../")
	}
	return path + ":" + strconv.Itoa(d.Span.Line)
}
