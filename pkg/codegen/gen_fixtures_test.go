package codegen

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
)

func TestGenerateFixtures(t *testing.T) {
	if os.Getenv("MONOLIFT_UPDATE_FIXTURES") != "1" {
		t.Skip("set MONOLIFT_UPDATE_FIXTURES=1 to regenerate cached fixtures")
	}

	root := repoRoot(t)
	source := filepath.Join(root, "evaluation", "miniflux")
	outDir := filepath.Join(root, "pkg", "codegen", "testdata")

	specs := []struct {
		name      string
		relTarget string
		service   string
	}{
		{"sanitizehtml", "internal/reader/sanitizer/sanitizer.go:217", "sanitizehtml"},
		{"refreshfeed", "internal/reader/handler/handler.go:207", "refreshfeed"},
	}

	for _, spec := range specs {
		t.Run(spec.name, func(t *testing.T) {
			target := filepath.Join(source, spec.relTarget)
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			result, err := runActivation(ctx, LiftOptions{
				Source: source,
				Target: target,
			})
			if err != nil {
				t.Fatal(err)
			}

			cut, err := activation.AnalyzeCut(result, nil)
			if err != nil {
				t.Fatal(err)
			}
			if cut == nil || cut.Recommended == nil {
				t.Fatal("no recommended cut")
			}

			report, err := buildExtractionReport(LiftOptions{
				Source:      source,
				Target:      target,
				ServiceName: spec.service,
			}, cut)
			if err != nil {
				t.Fatal(err)
			}

			data := fixtureJSON{
				Report: report,
				Cut:    *cut,
			}
			raw, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			raw = append(raw, '\n')

			outPath := filepath.Join(outDir, spec.name+"_fixture.json")
			if err := os.WriteFile(outPath, raw, 0644); err != nil {
				t.Fatal(err)
			}
			t.Logf("wrote %s", outPath)
		})
	}
}
