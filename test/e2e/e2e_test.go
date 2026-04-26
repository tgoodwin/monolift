//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/test/e2e/harness"
	"github.com/tgoodwin/monolift/test/e2e/targets/caddy"
	"github.com/tgoodwin/monolift/test/e2e/targets/gitea"
	"github.com/tgoodwin/monolift/test/e2e/targets/listmonk"
	"github.com/tgoodwin/monolift/test/e2e/targets/mattermost"
	"github.com/tgoodwin/monolift/test/e2e/targets/miniflux"
	"github.com/tgoodwin/monolift/test/e2e/targets/pocketbase"
	"github.com/tgoodwin/monolift/test/e2e/targets/pragma"
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
	targets = append(targets, pragma.Targets()...)
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
	if err := assertRootSelection(target, compileResult); err != nil {
		t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "root selection assertion failed: %v", err))
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

	if err := buildAndLoadLiftedImages(ctx, builder, target, compileResult.ArtifactsDir); err != nil {
		t.Fatalf("%v", err)
	}

	liftedManifests := liftedManifestPaths(target, compileResult.ArtifactsDir)
	if err := deployer.Apply(ctx, liftedNS, liftedManifests); err != nil {
		t.Fatalf("%v", err)
	}
	if err := deployer.WaitReady(ctx, liftedNS, 180*time.Second); err != nil {
		t.Fatalf("%v", err)
	}
	liftedService := target.ServiceName
	if target.LiftedHostBuild != nil {
		liftedService = "caddy-lifted"
	}
	liftedPF, err := harness.StartPortForward(ctx, target.Name, liftedNS, liftedService, target.ServicePort)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer liftedPF.Stop()
	var liftedTranscript harness.Transcript
	if len(target.LiftedExtractedServices) > 0 {
		liftedTranscript, err = runLiftedWithCallDeltas(ctx, target, liftedNS, liftedPF.URL)
	} else {
		liftedTranscript, err = target.Workload.Action(ctx, liftedPF.URL)
	}
	if err != nil {
		t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "lifted action failed: %v", err))
	}
	if len(target.LiftedExtractedServices) > 0 {
		if err := assertExtractedServiceLogs(ctx, target, liftedNS, []string{"LIFT_INVOKE id=", "/static/hello.txt", "/proxy", "/headers"}); err != nil {
			t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "lifted logs assertion failed: %v", err))
		}
	}
	if err := (harness.Transcript{}).Compare(baselineTranscript, liftedTranscript, target.Invariants); err != nil {
		t.Fatalf("%v", harness.StageError(9, target.Name, harness.KindWorkload, "transcript compare failed: %v", err))
	}
	if len(target.LiftedExtractedServices) > 0 {
		if err := assertEnvOffAndFailModes(ctx, deployer, target, liftedNS, liftedPF.URL, liftedTranscript); err != nil {
			t.Fatalf("%v", harness.StageError(9, target.Name, harness.KindWorkload, "negative lifted assertions failed: %v", err))
		}
	}
}

type requestWorkload interface {
	Paths() []string
	Request(ctx context.Context, host, path string) (harness.Step, error)
}

func runLiftedWithCallDeltas(ctx context.Context, target harness.TargetCase, liftedNS, liftedURL string) (harness.Transcript, error) {
	workload, ok := target.Workload.(requestWorkload)
	if !ok {
		return harness.Transcript{}, fmt.Errorf("target workload does not support per-request execution")
	}
	service := target.LiftedExtractedServices[0]
	pf, err := harness.StartPortForward(ctx, target.Name, liftedNS, service.Name, 8081)
	if err != nil {
		return harness.Transcript{}, err
	}
	defer pf.Stop()

	transcript := harness.Transcript{Steps: make([]harness.Step, 0, len(workload.Paths()))}
	total := int64(0)
	for _, path := range workload.Paths() {
		before, err := readCalls(ctx, pf.URL)
		if err != nil {
			return harness.Transcript{}, err
		}
		step, err := workload.Request(ctx, liftedURL, path)
		if err != nil {
			return harness.Transcript{}, err
		}
		after, err := readCalls(ctx, pf.URL)
		if err != nil {
			return harness.Transcript{}, err
		}
		delta := after - before
		if delta < 1 {
			return harness.Transcript{}, fmt.Errorf("%s /calls delta=%d want >=1", path, delta)
		}
		total += delta
		transcript.Steps = append(transcript.Steps, step)
	}
	if total < 3 || total > 50 {
		return harness.Transcript{}, fmt.Errorf("aggregate /calls delta=%d want 3 <= total <= 50", total)
	}
	if err := assertExtractedInvocations(ctx, pf.URL, invocationPaths(workload.Paths()), target.Oracle); err != nil {
		return harness.Transcript{}, err
	}
	return transcript, nil
}

func invocationPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if i := strings.Index(path, "?"); i >= 0 {
			path = path[:i]
		}
		out = append(out, path)
	}
	return out
}

func readCalls(ctx context.Context, serviceURL string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL+"/calls", nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

type invocationRecord struct {
	ID              int64  `json:"id"`
	InvocationID    string `json:"invocation_id"`
	P               string `json:"p"`
	CollapseSlashes bool   `json:"collapse_slashes"`
	Result          string `json:"result"`
}

func assertExtractedInvocations(ctx context.Context, serviceURL string, expectedPaths []string, oracle harness.SymbolInvoker) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL+"/invocations", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Records []invocationRecord `json:"records"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	for _, path := range expectedPaths {
		found := false
		for _, record := range out.Records {
			if record.P == path {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no invocation record for path %s in %d records", path, len(out.Records))
		}
	}
	if oracle == nil {
		return fmt.Errorf("target has no oracle")
	}
	for _, record := range out.Records {
		got, err := oracle.Invoke(map[string]any{"p": record.P, "collapse_slashes": record.CollapseSlashes})
		if err != nil {
			return err
		}
		if got != record.Result {
			return fmt.Errorf("oracle mismatch for %q collapse=%v: record=%q oracle=%v", record.P, record.CollapseSlashes, record.Result, got)
		}
	}
	return nil
}

func assertExtractedServiceLogs(ctx context.Context, target harness.TargetCase, ns string, expected []string) error {
	service := target.LiftedExtractedServices[0]
	result, err := kubectlResult(ctx, ns, "logs", "deployment/"+service.Name)
	if err != nil {
		return fmt.Errorf("kubectl logs: %w: %s", err, result.Stderr)
	}
	logs := result.Stdout + result.Stderr
	for _, needle := range expected {
		if !strings.Contains(logs, needle) {
			return fmt.Errorf("logs missing %q", needle)
		}
	}
	return nil
}

func assertEnvOffAndFailModes(ctx context.Context, deployer harness.Deployer, target harness.TargetCase, ns, liftedURL string, envOnTranscript harness.Transcript) error {
	service := target.LiftedExtractedServices[0]
	servicePF, err := harness.StartPortForward(ctx, target.Name, ns, service.Name, 8081)
	if err != nil {
		return err
	}
	defer servicePF.Stop()

	if err := kubectl(ctx, ns, "set", "env", "deployment/caddy-lifted", "MONOLIFT_LIFT_CLEANPATH-"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/caddy-lifted", "--timeout=120s"); err != nil {
		return err
	}
	envOffPF, err := harness.StartPortForward(ctx, target.Name, ns, "caddy-lifted", target.ServicePort)
	if err != nil {
		return err
	}
	before, err := readCalls(ctx, servicePF.URL)
	if err != nil {
		envOffPF.Stop()
		return err
	}
	envOffTranscript, err := target.Workload.Action(ctx, envOffPF.URL)
	envOffPF.Stop()
	if err != nil {
		return err
	}
	after, err := readCalls(ctx, servicePF.URL)
	if err != nil {
		return err
	}
	if after-before != 0 {
		return fmt.Errorf("env-off /calls delta=%d want 0", after-before)
	}
	if err := (harness.Transcript{}).Compare(envOnTranscript, envOffTranscript, target.Invariants); err != nil {
		return fmt.Errorf("env-off transcript mismatch: %w", err)
	}

	if err := kubectl(ctx, ns, "set", "env", "deployment/caddy-lifted", "MONOLIFT_LIFT_CLEANPATH=on", "MONOLIFT_LIFT_FAILMODE=closed", "MONOLIFT_LIFT_CLEANPATH_ENDPOINT=http://127.0.0.1:1/invoke"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/caddy-lifted", "--timeout=120s"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "scale", "deployment/"+service.Name, "--replicas=0"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "wait", "--for=delete", "pod", "-l", "app="+service.Name, "--timeout=120s"); err != nil {
		return err
	}
	if err := deployer.WaitReady(ctx, ns, 120*time.Second); err != nil {
		return err
	}
	closedPF, err := harness.StartPortForward(ctx, target.Name, ns, "caddy-lifted", target.ServicePort)
	if err != nil {
		return err
	}
	closedStep, err := workloadStep(ctx, target, closedPF.URL, "/headers")
	closedPF.Stop()
	if err != nil {
		return err
	}
	if closedStep.Status != http.StatusNotFound {
		return fmt.Errorf("fail-closed status=%d want 404", closedStep.Status)
	}
	if err := kubectl(ctx, ns, "scale", "deployment/"+service.Name, "--replicas=1"); err != nil {
		return err
	}
	if err := deployer.WaitReady(ctx, ns, 120*time.Second); err != nil {
		return err
	}

	if err := kubectl(ctx, ns, "set", "env", "deployment/caddy-lifted", "MONOLIFT_LIFT_FAILMODE=open", "MONOLIFT_LIFT_CLEANPATH_ENDPOINT=http://127.0.0.1:1/invoke"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/caddy-lifted", "--timeout=120s"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "scale", "deployment/"+service.Name, "--replicas=0"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "wait", "--for=delete", "pod", "-l", "app="+service.Name, "--timeout=120s"); err != nil {
		return err
	}
	if err := deployer.WaitReady(ctx, ns, 120*time.Second); err != nil {
		return err
	}
	openPF, err := harness.StartPortForward(ctx, target.Name, ns, "caddy-lifted", target.ServicePort)
	if err != nil {
		return err
	}
	openBefore, err := readCalls(ctx, servicePF.URL)
	if err != nil {
		openBefore = 0
	}
	openStep, err := workloadStep(ctx, target, openPF.URL, "/headers")
	openPF.Stop()
	if err != nil {
		return err
	}
	if openStep.Status != http.StatusOK {
		return fmt.Errorf("fail-open status=%d want 200", openStep.Status)
	}
	openAfter, err := readCalls(ctx, servicePF.URL)
	if err == nil && openAfter-openBefore != 0 {
		return fmt.Errorf("fail-open /calls delta=%d want 0", openAfter-openBefore)
	}
	if err := kubectl(ctx, ns, "scale", "deployment/"+service.Name, "--replicas=1"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "set", "env", "deployment/caddy-lifted", "MONOLIFT_LIFT_FAILMODE=closed", "MONOLIFT_LIFT_CLEANPATH_ENDPOINT=http://monolift-extracted-cleanpath:8081/invoke"); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/caddy-lifted", "--timeout=120s"); err != nil {
		return err
	}
	if err := deployer.WaitReady(ctx, ns, 120*time.Second); err != nil {
		return err
	}
	restoredCaddyPF, err := harness.StartPortForward(ctx, target.Name, ns, "caddy-lifted", target.ServicePort)
	if err != nil {
		return err
	}
	defer restoredCaddyPF.Stop()
	restoredPF, err := harness.StartPortForward(ctx, target.Name, ns, service.Name, 8081)
	if err != nil {
		return err
	}
	defer restoredPF.Stop()
	restoredBefore, err := readCalls(ctx, restoredPF.URL)
	if err != nil {
		return err
	}
	if _, err := workloadStep(ctx, target, restoredCaddyPF.URL, "/headers"); err != nil {
		return err
	}
	restoredAfter, err := readCalls(ctx, restoredPF.URL)
	if err != nil {
		return err
	}
	if restoredAfter-restoredBefore < 1 {
		return fmt.Errorf("restored /calls delta=%d want >=1", restoredAfter-restoredBefore)
	}
	return nil
}

func firstWorkloadStep(ctx context.Context, target harness.TargetCase, liftedURL string) (harness.Step, error) {
	workload, ok := target.Workload.(requestWorkload)
	if !ok {
		return harness.Step{}, fmt.Errorf("target workload does not support per-request execution")
	}
	return workload.Request(ctx, liftedURL, workload.Paths()[0])
}

func workloadStep(ctx context.Context, target harness.TargetCase, liftedURL, path string) (harness.Step, error) {
	workload, ok := target.Workload.(requestWorkload)
	if !ok {
		return harness.Step{}, fmt.Errorf("target workload does not support per-request execution")
	}
	return workload.Request(ctx, liftedURL, path)
}

func kubectl(ctx context.Context, ns string, args ...string) error {
	result, err := kubectlResult(ctx, ns, args...)
	if err != nil {
		return fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, result.Stderr)
	}
	return nil
}

func kubectlResult(ctx context.Context, ns string, args ...string) (harness.CommandResult, error) {
	kubeconfigResult, err := harness.RunCommand(ctx, "kind", "get", "kubeconfig", "--name", harness.DefaultClusterName)
	if err != nil {
		return kubeconfigResult, err
	}
	file, err := os.CreateTemp("", "monolift-e2e-kubeconfig-*")
	if err != nil {
		return harness.CommandResult{}, err
	}
	defer os.Remove(file.Name())
	if _, err := file.WriteString(kubeconfigResult.Stdout); err != nil {
		_ = file.Close()
		return harness.CommandResult{}, err
	}
	if err := file.Close(); err != nil {
		return harness.CommandResult{}, err
	}
	full := append([]string{"-n", ns}, args...)
	full = append([]string{"--kubeconfig", file.Name()}, full...)
	return harness.RunCommand(ctx, "kubectl", full...)
}

func buildAndLoadLiftedImages(ctx context.Context, builder harness.ImageBuilder, target harness.TargetCase, artifactsDir string) error {
	if target.LiftedHostBuild == nil {
		if target.Dockerfile != "" {
			if err := builder.Build(ctx, target.Dockerfile, target.ContextDir, target.ImageTag); err != nil {
				return err
			}
			return builder.LoadToKind(ctx, target.ImageTag)
		}
		return nil
	}

	spec := *target.LiftedHostBuild
	if err := buildGeneratedImage(ctx, builder, artifactsDir, spec.Dockerfile, spec.ContextRoot, spec.ImageTag); err != nil {
		return err
	}
	for _, service := range target.LiftedExtractedServices {
		if err := buildGeneratedImage(ctx, builder, artifactsDir, service.Dockerfile, service.ContextRoot, service.ImageTag); err != nil {
			return err
		}
	}
	generatedBuilder := builder
	if err := generatedBuilder.LoadToKind(ctx, spec.ImageTag); err != nil {
		return err
	}
	for _, service := range target.LiftedExtractedServices {
		if err := generatedBuilder.LoadToKind(ctx, service.ImageTag); err != nil {
			return err
		}
	}
	return nil
}

func buildGeneratedImage(ctx context.Context, builder harness.ImageBuilder, artifactsDir, dockerfile, contextRoot, tag string) error {
	generatedBuilder := builder
	generatedBuilder.SourceDirs = []string{filepath.Join(artifactsDir, contextRoot)}
	return generatedBuilder.Build(ctx, filepath.Join(artifactsDir, dockerfile), filepath.Join(artifactsDir, contextRoot), tag)
}

func liftedManifestPaths(target harness.TargetCase, artifactsDir string) []string {
	if target.LiftedHostBuild == nil {
		liftedManifests, _ := filepath.Glob(filepath.Join(artifactsDir, "lifted", "*.yaml"))
		sort.Strings(liftedManifests)
		return liftedManifests
	}
	paths := make([]string, 0, len(target.BaselineManifests)+2+len(target.LiftedExtractedServices)*2)
	for _, manifest := range target.BaselineManifests {
		base := filepath.Base(manifest)
		if base == "deployment.yaml" || base == "service.yaml" {
			continue
		}
		paths = append(paths, manifest)
	}
	paths = append(paths,
		filepath.Join(artifactsDir, "lifted", "manifests", "caddy-lifted-deployment.yaml"),
		filepath.Join(artifactsDir, "lifted", "manifests", "caddy-lifted-service.yaml"),
	)
	for _, service := range target.LiftedExtractedServices {
		paths = append(paths,
			filepath.Join(artifactsDir, service.DeploymentYAML),
			filepath.Join(artifactsDir, service.ServiceYAML),
		)
	}
	return paths
}

func assertVerdict(target harness.TargetCase, result harness.CompileResult) error {
	required := make([]harness.DiagnosticCode, 0, len(target.RequiredDiagnostics))
	for _, code := range target.RequiredDiagnostics {
		required = append(required, harness.DiagnosticCode(code))
	}
	if target.ExpectedVerdict == "refuse-blocking" {
		return (harness.Verdict{}).AssertRefuse(result.Report, required)
	}
	if target.ExpectedVerdict == "accept-with-warnings" {
		return (harness.Verdict{}).AssertAcceptWithWarnings(result.Report, required)
	}
	return (harness.Verdict{}).AssertAccept(result.Report)
}

func assertRootSelection(target harness.TargetCase, result harness.CompileResult) error {
	if result.Report == nil {
		return nil
	}
	if target.ExpectedRootShape != "" && result.Report.Root.Shape != target.ExpectedRootShape {
		return fmt.Errorf("root.shape=%q want %q", result.Report.Root.Shape, target.ExpectedRootShape)
	}
	if target.ExpectedTransport != "" && result.Report.Root.DefaultTransport != target.ExpectedTransport {
		return fmt.Errorf("root.defaultTransport=%q want %q", result.Report.Root.DefaultTransport, target.ExpectedTransport)
	}
	if target.ExpectedArchetypeKind != "" && result.Report.Root.ArchetypeKind != target.ExpectedArchetypeKind {
		return fmt.Errorf("root.archetype_kind=%q want %q", result.Report.Root.ArchetypeKind, target.ExpectedArchetypeKind)
	}
	if target.ExpectedPrimary.Archetype != "" {
		if result.Report.Root.Primary == nil {
			return fmt.Errorf("root.primary missing")
		}
		if err := assertArchetypeChoice("root.primary", *result.Report.Root.Primary, target.ExpectedPrimary); err != nil {
			return err
		}
	}
	if len(target.ExpectedAlternatives) > 0 {
		if len(result.Report.Root.Alternatives) != len(target.ExpectedAlternatives) {
			return fmt.Errorf("root.alternatives len=%d want %d", len(result.Report.Root.Alternatives), len(target.ExpectedAlternatives))
		}
		for i, expected := range target.ExpectedAlternatives {
			if err := assertArchetypeChoice(fmt.Sprintf("root.alternatives[%d]", i), result.Report.Root.Alternatives[i], expected); err != nil {
				return err
			}
		}
	}
	if target.ExpectedAdapterKind != "" && !hasAdapter(result.Report.Adapters, target.ExpectedAdapterKind, target.ExpectedAdapterID) {
		return fmt.Errorf("adapters missing kind=%q id=%q", target.ExpectedAdapterKind, target.ExpectedAdapterID)
	}
	for _, fact := range target.RequiredRootFacts {
		if !hasRootFact(result.Report.Root.Properties, fact.PropertyID, fact.Verdict) {
			return fmt.Errorf("root.properties missing %s=%s", fact.PropertyID, fact.Verdict)
		}
	}
	return nil
}

func assertArchetypeChoice(path string, got reportv2.ArchetypeChoice, expected harness.ExpectedArchetypeChoice) error {
	if got.Archetype != expected.Archetype {
		return fmt.Errorf("%s.archetype=%q want %q", path, got.Archetype, expected.Archetype)
	}
	if expected.ContributingArchetypes != nil && !reflect.DeepEqual(got.ContributingArchetypes, expected.ContributingArchetypes) {
		return fmt.Errorf("%s.contributing_archetypes=%v want %v", path, got.ContributingArchetypes, expected.ContributingArchetypes)
	}
	if got.Alias != expected.Alias {
		return fmt.Errorf("%s.alias=%q want %q", path, got.Alias, expected.Alias)
	}
	if expected.Emittable != nil && got.Emittable != *expected.Emittable {
		return fmt.Errorf("%s.emittable=%v want %v", path, got.Emittable, *expected.Emittable)
	}
	if expected.RuntimeSelectable != nil && got.RuntimeSelectable != *expected.RuntimeSelectable {
		return fmt.Errorf("%s.runtime_selectable=%v want %v", path, got.RuntimeSelectable, *expected.RuntimeSelectable)
	}
	if expected.RationaleTierEqual != "" && got.RationaleTier != expected.RationaleTierEqual {
		return fmt.Errorf("%s.rationale_tier=%q want %q", path, got.RationaleTier, expected.RationaleTierEqual)
	}
	if expected.RationaleNonEmpty && got.Rationale == "" {
		return fmt.Errorf("%s.rationale empty", path)
	}
	return nil
}

func hasAdapter(adapters []reportv2.Adapter, kind, id string) bool {
	for _, adapter := range adapters {
		if adapter.Kind == kind && (id == "" || adapter.ID == id) {
			return true
		}
	}
	return false
}

func hasRootFact(properties []reportv2.PropertyEvidence, propertyID, verdict string) bool {
	for _, property := range properties {
		if property.PropertyID == propertyID && property.Verdict == verdict {
			return true
		}
	}
	return false
}
