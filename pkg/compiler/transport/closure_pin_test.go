package transport

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestCaddyReportPinsCleanPathInClosure(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "e2e", "targets", "caddy", "golden", "report.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	report, err := reportv2.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	for _, symbol := range report.Closure.IncludedSymbols {
		identity := symbol.Identity
		if identity.PackagePath == "github.com/caddyserver/caddy/v2/modules/caddyhttp" &&
			identity.ObjectName == "CleanPath" &&
			identity.Kind == "function" {
			return
		}
	}
	t.Fatal("caddy report closure.includedSymbols does not contain caddyhttp.CleanPath")
}

func TestMinifluxReportPinsEstimateReadingTimeInClosure(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "test", "e2e", "targets", "miniflux", "golden", "report.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	report, err := reportv2.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	for _, symbol := range report.Closure.IncludedSymbols {
		identity := symbol.Identity
		if identity.ModulePath == "miniflux.app/v2" &&
			identity.PackagePath == "miniflux.app/v2/internal/reader/readingtime" &&
			identity.ObjectName == "EstimateReadingTime" &&
			identity.Kind == "function" &&
			symbol.Span.FileRelativePath == "internal/reader/readingtime/readingtime.go" &&
			symbol.Span.LineStart == 17 {
			return
		}
	}
	t.Fatal("miniflux report closure.includedSymbols does not contain readingtime.EstimateReadingTime at line 17")
}
