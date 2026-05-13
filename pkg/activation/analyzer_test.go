package activation

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAnalyzeSkipsAugmentWhenRTAReachable(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/simple")
	cfg := Config{Dir: dir, Packages: []string{"."}, Target: "main.go:34", Augment: ModeAll, SkipAugmentWhenRTAReachable: true}
	result, err := NewAnalyzer(cfg).Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("target not found")
	}
	if !result.SkippedAugment {
		t.Fatal("SkippedAugment = false, want true")
	}
	if len(result.SubTimings) != 0 {
		t.Fatalf("SubTimings length = %d, want 0 for skipped augment", len(result.SubTimings))
	}

	rtaResult, err := NewAnalyzer(Config{Dir: dir, Packages: []string{"."}, Target: "main.go:34", Augment: ModeRTAOnly, SkipAugmentWhenRTAReachable: true}).Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Path.Steps) != len(rtaResult.Path.Steps) {
		t.Fatalf("path length = %d, want RTA-only path length %d", len(result.Path.Steps), len(rtaResult.Path.Steps))
	}
	cut, err := AnalyzeCut(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	rtaCut, err := AnalyzeCut(rtaResult, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cut.Recommended.Step != rtaCut.Recommended.Step {
		t.Fatalf("cut step = %d, want RTA-only cut step %d", cut.Recommended.Step, rtaCut.Recommended.Step)
	}
}

func TestAnalyzeRunsAugmentWhenTargetRequiresAugmentation(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "pkg/activation/testdata/mapfunc/direct")
	result, err := NewAnalyzer(Config{Dir: dir, Packages: []string{"."}, Target: "main.go:22", Augment: ModeAll}).Analyze(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found {
		t.Fatal("target not found")
	}
	if result.SkippedAugment {
		t.Fatal("SkippedAugment = true, want false")
	}
	if len(result.SubTimings) == 0 {
		t.Fatal("SubTimings is empty, want augmentation sub-pass timings")
	}
}
