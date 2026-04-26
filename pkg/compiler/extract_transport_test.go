package compiler

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestEmitContextsIncludesMinifluxEstimateReadingTime(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "test", "e2e", "targets", "miniflux", "golden", "report.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	report, err := reportv2.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	contexts := emitContexts(*report)
	if len(contexts) != 1 {
		t.Fatalf("len(contexts)=%d want 1", len(contexts))
	}
	ctx := contexts[0]
	if ctx.SymbolImportPath != "miniflux.app/v2/internal/reader/readingtime" ||
		ctx.ObjectName != "EstimateReadingTime" ||
		ctx.ServiceName != "monolift-extracted-estimatereadingtime" ||
		ctx.EnvVarPrefix != "MONOLIFT_LIFT_ESTIMATEREADINGTIME" {
		t.Fatalf("bad miniflux context: %+v", ctx)
	}
	if got, want := len(ctx.ParamFields), 3; got != want {
		t.Fatalf("len(ParamFields)=%d want %d", got, want)
	}
	if got, want := ctx.ResultFields[0].GoType, "int"; got != want {
		t.Fatalf("result GoType=%q want %q", got, want)
	}
}
