package compiler

import (
	"encoding/json"
	"path/filepath"
	"reflect"
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
	if len(extractDiagnostics) != 0 {
		t.Fatalf("Extract diagnostics = %+v, want none", extractDiagnostics)
	}
	if len(report.Diagnostics) != 0 {
		t.Fatalf("report diagnostics = %+v, want none", report.Diagnostics)
	}

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

	gotCodes := []string{}
	for _, diagnostic := range extractDiagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	wantCodes := []string{"MLV2_CLOSURE_TOO_LARGE", "MLV2_EMBEDDED_DB_APP_ROOT"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("extract diagnostics = %v, want %v", gotCodes, wantCodes)
	}
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
