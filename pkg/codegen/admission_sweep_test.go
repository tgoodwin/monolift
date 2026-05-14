package codegen

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
)

var (
	traceTarget = flag.String("trace-target", "", "file:line target for admission check")
	sourceDir   = flag.String("source-dir", "", "source directory for admission check")
)

// TestAdmission runs the full admission pipeline (activation → cut → report →
// AdmitCut → BuildPlan → AdmitPlan) for a single corpus trace specified via
// -trace-target and -source-dir flags. Used by the corpus sweep runner.
func TestAdmission(t *testing.T) {
	if *traceTarget == "" || *sourceDir == "" {
		t.Skip("requires -trace-target and -source-dir flags")
	}

	root := repoRoot(t)
	absSource := filepath.Join(root, *sourceDir)

	// Make the target file:line absolute relative to the source dir so
	// buildExtractionReport resolves it correctly.
	file, line, err := activation.ParseTarget(*traceTarget)
	if err != nil {
		t.Fatalf("invalid trace-target: %v", err)
	}
	absTarget := fmt.Sprintf("%s:%d", filepath.Join(absSource, file), line)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Step 1: activation analysis
	analyzer := activation.NewAnalyzer(activation.Config{
		Dir:           absSource,
		Packages:      []string{"./..."},
		Target:        absTarget,
		Timeout:       60 * time.Second,
		Augment:       activation.ModeAll,
		ScopePackages: true,
	})
	result, err := analyzer.Analyze(ctx)
	if err != nil {
		t.Fatalf("activation analysis failed: %v", err)
	}
	if result == nil || !result.Found || result.Path == nil {
		t.Fatalf("activation path not found for %s", *traceTarget)
	}

	// Step 2: cut analysis
	cut, err := activation.AnalyzeCut(result, nil)
	if err != nil {
		t.Fatalf("cut analysis failed: %v", err)
	}

	// Step 3: extraction report
	opts := LiftOptions{
		Source: absSource,
		Target: absTarget,
	}
	report, err := buildExtractionReport(opts, cut)
	if err != nil {
		t.Fatalf("extraction report failed: %v", err)
	}

	// Step 4: AdmitCut
	cutAdmission := AdmitCut(report, *cut)
	if !cutAdmission.Accepted {
		t.Fatalf("refusal: %s", cutAdmission.Error())
	}

	// Step 5: BuildPlan + AdmitPlan
	plan, err := BuildPlan(report, *cut)
	if err != nil {
		t.Fatalf("build plan failed: %v", err)
	}
	applyLiftOptions(plan, opts)
	planAdmission := AdmitPlan(plan, cutAdmission)
	if !planAdmission.Accepted {
		t.Fatalf("refusal: %s", planAdmission.Error())
	}

	fmt.Printf("ADMITTED: %s (boundary params: %d, reconstructed: %d, results: %d)\n",
		plan.CutPoint.FuncName, len(plan.BoundaryParams), len(plan.ReconstructedParams), len(plan.Results))
}
