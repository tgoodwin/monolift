package codegen

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
)

var (
	traceTarget       = flag.String("trace-target", "", "file:line target for admission check")
	sourceDir         = flag.String("source-dir", "", "source directory for admission check")
	admissionPackages = flag.String("admission-packages", "", "comma-separated package patterns to type-check; empty uses reverse-import scope. Whole-repo ./... is refused.")
	admissionTimeout  = flag.Duration("admission-timeout", 10*time.Minute, "overall timeout for the admission probe")
	activationTimeout = flag.Duration("activation-timeout", 8*time.Minute, "timeout for activation-path analysis inside the admission probe")
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

	packages, scopePackages := admissionPackageScope(t, *admissionPackages)

	ctx, cancel := context.WithTimeout(context.Background(), *admissionTimeout)
	defer cancel()

	// Step 1: activation analysis
	analyzer := activation.NewAnalyzer(activation.Config{
		Dir:           absSource,
		Packages:      packages,
		Target:        absTarget,
		Timeout:       *activationTimeout,
		Augment:       activation.ModeAll,
		ScopePackages: scopePackages,
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

func admissionPackageScope(t *testing.T, raw string) ([]string, bool) {
	t.Helper()
	packages, scopePackages, err := parseAdmissionPackageScope(raw)
	if err != nil {
		t.Fatal(err)
	}
	return packages, scopePackages
}

func parseAdmissionPackageScope(raw string) ([]string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true, nil
	}
	parts := strings.Split(raw, ",")
	patterns := make([]string, 0, len(parts))
	for _, part := range parts {
		pattern := strings.TrimSpace(part)
		if pattern == "" {
			continue
		}
		if pattern == "./..." {
			return nil, false, fmt.Errorf("focused admission must not type-check the whole repository; use reverse-import scope or explicit target/importer packages")
		}
		patterns = append(patterns, pattern)
	}
	if len(patterns) == 0 {
		return nil, true, nil
	}
	return patterns, false, nil
}

func TestParseAdmissionPackageScope(t *testing.T) {
	t.Run("default uses reverse import scope", func(t *testing.T) {
		packages, scoped, err := parseAdmissionPackageScope("")
		if err != nil {
			t.Fatal(err)
		}
		if len(packages) != 0 || !scoped {
			t.Fatalf("packages=%v scoped=%v, want nil true", packages, scoped)
		}
	})

	t.Run("explicit focused packages", func(t *testing.T) {
		packages, scoped, err := parseAdmissionPackageScope(" ./cmd/miniflux , ./internal/reader/... ")
		if err != nil {
			t.Fatal(err)
		}
		if scoped {
			t.Fatalf("scoped = true, want false")
		}
		want := []string{"./cmd/miniflux", "./internal/reader/..."}
		if strings.Join(packages, "|") != strings.Join(want, "|") {
			t.Fatalf("packages=%v, want %v", packages, want)
		}
	})

	t.Run("rejects whole repo", func(t *testing.T) {
		if _, _, err := parseAdmissionPackageScope("./..."); err == nil {
			t.Fatalf("expected error for whole-repo package scope")
		}
	})
}
