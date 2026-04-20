//go:build e2e

package e2e

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/test/e2e/harness"
	"github.com/tgoodwin/monolift/test/e2e/targets/caddy"
	"github.com/tgoodwin/monolift/test/e2e/targets/gitea"
	"github.com/tgoodwin/monolift/test/e2e/targets/listmonk"
	"github.com/tgoodwin/monolift/test/e2e/targets/mattermost"
	"github.com/tgoodwin/monolift/test/e2e/targets/miniflux"
	"github.com/tgoodwin/monolift/test/e2e/targets/pocketbase"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite e2e golden reports from current compiler output")

func TestE2E(t *testing.T) {
	if !harness.E2EEnabled() {
		t.Skip("MONOLIFT_E2E=1 required")
	}

	runID := harness.NewRunID()
	cluster := harness.NewCluster()
	targets := []harness.TargetCase{
		caddy.Target(),
		pocketbase.Target(),
		miniflux.Target(),
		listmonk.Target(),
		gitea.Target(),
		mattermost.Target(),
	}
	for _, target := range targets {
		target := target
		t.Run(target.Name, func(t *testing.T) {
			if target.SkipReason != "" {
				t.Skip(target.SkipReason)
			}
			runTarget(t, cluster, runID, target)
		})
	}
}

func updateGoldenRequested() bool {
	return *updateGolden || os.Getenv(harness.EnvUpdateGolden) == "1"
}

func runTarget(t *testing.T, cluster harness.Cluster, runID string, target harness.TargetCase) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := cluster.Ensure(ctx); err != nil {
		t.Fatalf("%v", harness.StageError(0, target.Name, harness.KindHarness, "cluster ensure failed: %v", err))
	}

	deployer := harness.Deployer{Cluster: cluster, Target: target.Name}
	baselineNS := harness.Namespace("baseline", target.Name, runID)
	liftedNS := harness.Namespace("lifted", target.Name, runID)
	if err := deployer.CreateNamespace(ctx, baselineNS); err != nil {
		t.Fatalf("%v", err)
	}
	if target.StopAtStage > 4 {
		if err := deployer.CreateNamespace(ctx, liftedNS); err != nil {
			t.Fatalf("%v", err)
		}
	}
	defer func() {
		_ = deployer.DeleteNamespace(context.Background(), baselineNS, 30*time.Second)
		_ = deployer.DeleteNamespace(context.Background(), liftedNS, 30*time.Second)
	}()

	builder := harness.ImageBuilder{Cluster: cluster, Target: target.Name, SourceDirs: target.SourceDirs}
	if target.Dockerfile != "" {
		if err := builder.Build(ctx, target.Dockerfile, target.ContextDir, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
		if err := builder.LoadToKind(ctx, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
	}

	var baselineTranscript harness.Transcript
	if len(target.BaselineManifests) > 0 {
		if err := deployer.Apply(ctx, baselineNS, target.BaselineManifests); err != nil {
			t.Fatalf("%v", harness.StageError(1, target.Name, harness.KindHarness, "baseline deploy failed: %v", err))
		}
		if err := deployer.WaitReady(ctx, baselineNS, 180*time.Second); err != nil {
			t.Fatalf("%v", harness.StageError(1, target.Name, harness.KindHarness, "baseline wait failed: %v", err))
		}
		pf, err := harness.StartPortForward(ctx, target.Name, baselineNS, target.ServiceName, target.ServicePort)
		if err != nil {
			t.Fatalf("%v", err)
		}
		defer pf.Stop()
		if err := target.Workload.Setup(ctx, pf.URL); err != nil {
			t.Fatalf("%v", harness.StageError(2, target.Name, harness.KindWorkload, "baseline setup failed: %v", err))
		}
		baselineTranscript, err = target.Workload.Action(ctx, pf.URL)
		if err != nil {
			t.Fatalf("%v", harness.StageError(2, target.Name, harness.KindWorkload, "baseline action failed: %v", err))
		}
	}

	compileDir := filepath.Join(os.TempDir(), "monolift-e2e", target.Name, runID, "compile")
	compileResult, err := (harness.Compiler{OutputDir: compileDir}).Run(ctx, target)
	if err != nil {
		t.Fatalf("%v", harness.FormatCompileFailure(target, compileResult, err))
	}

	if err := assertVerdict(target, compileResult); err != nil {
		t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "verdict assertion failed: %v", err))
	}
	if target.GoldenReport != "" {
		golden, err := harness.LoadGolden(target.GoldenReport)
		if err != nil {
			t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "load golden failed: %v", err))
		}
		if err := (harness.Report{}).CompareNormativeSubset(golden, compileResult.Report); err != nil {
			if updateGoldenRequested() {
				if writeErr := harness.WriteGolden(target.GoldenReport, compileResult.Report); writeErr != nil {
					t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "update golden failed: %v", writeErr))
				}
				t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "golden updated; review and commit manually: %v", err))
			}
			t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "%v", err))
		}
	}
	if target.StopAtStage <= 4 {
		return
	}

	if target.Dockerfile != "" {
		if err := builder.Build(ctx, target.Dockerfile, target.ContextDir, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
		if err := builder.LoadToKind(ctx, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
	}

	liftedManifests, err := filepath.Glob(filepath.Join(compileResult.ArtifactsDir, "lifted", "*.yaml"))
	if err != nil {
		t.Fatalf("%v", harness.StageError(7, target.Name, harness.KindHarness, "glob lifted manifests failed: %v", err))
	}
	sort.Strings(liftedManifests)
	if err := deployer.Apply(ctx, liftedNS, liftedManifests); err != nil {
		t.Fatalf("%v", err)
	}
	if err := deployer.WaitReady(ctx, liftedNS, 180*time.Second); err != nil {
		t.Fatalf("%v", err)
	}
	liftedPF, err := harness.StartPortForward(ctx, target.Name, liftedNS, target.ServiceName, target.ServicePort)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer liftedPF.Stop()
	liftedTranscript, err := target.Workload.Action(ctx, liftedPF.URL)
	if err != nil {
		t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "lifted action failed: %v", err))
	}
	if err := (harness.Transcript{}).Compare(baselineTranscript, liftedTranscript, target.Invariants); err != nil {
		t.Fatalf("%v", harness.StageError(9, target.Name, harness.KindWorkload, "transcript compare failed: %v", err))
	}
}

func assertVerdict(target harness.TargetCase, result harness.CompileResult) error {
	if target.ExpectedVerdict == "refuse-blocking" {
		required := make([]harness.DiagnosticCode, 0, len(target.RequiredDiagnostics))
		for _, code := range target.RequiredDiagnostics {
			required = append(required, harness.DiagnosticCode(code))
		}
		return (harness.Verdict{}).AssertRefuse(result.Report, required)
	}
	return (harness.Verdict{}).AssertAccept(result.Report)
}
