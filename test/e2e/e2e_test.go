//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/test/e2e/harness"
	activation_caddy_cleanpath "github.com/tgoodwin/monolift/test/e2e/targets/activation_caddy_cleanpath"
	activation_gitea_argon2hash "github.com/tgoodwin/monolift/test/e2e/targets/activation_gitea_argon2hash"
	activation_gitea_pathescapesegments "github.com/tgoodwin/monolift/test/e2e/targets/activation_gitea_pathescapesegments"
	activation_listmonk_sanitizeuri "github.com/tgoodwin/monolift/test/e2e/targets/activation_listmonk_sanitizeuri"
	activation_mattermost_pbkdf2hash "github.com/tgoodwin/monolift/test/e2e/targets/activation_mattermost_pbkdf2hash"
	activation_mattermost_publiclinkhash "github.com/tgoodwin/monolift/test/e2e/targets/activation_mattermost_publiclinkhash"
	activation_miniflux_refreshfeed "github.com/tgoodwin/monolift/test/e2e/targets/activation_miniflux_refreshfeed"
	activation_miniflux_sanitizehtml "github.com/tgoodwin/monolift/test/e2e/targets/activation_miniflux_sanitizehtml"
	activation_miniflux_striptags "github.com/tgoodwin/monolift/test/e2e/targets/activation_miniflux_striptags"
	activation_pocketbase_columnify "github.com/tgoodwin/monolift/test/e2e/targets/activation_pocketbase_columnify"
	activation_pocketbase_passwordvalidate "github.com/tgoodwin/monolift/test/e2e/targets/activation_pocketbase_passwordvalidate"
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
		activation_caddy_cleanpath.Target(),
		pocketbase.Target(),
		miniflux.Target(),
		activation_miniflux_refreshfeed.Target(),
		activation_miniflux_sanitizehtml.Target(),
		activation_miniflux_striptags.Target(),
		listmonk.Target(),
		gitea.Target(),
		activation_gitea_argon2hash.Target(),
		activation_gitea_pathescapesegments.Target(),
		activation_listmonk_sanitizeuri.Target(),
		activation_pocketbase_columnify.Target(),
		activation_pocketbase_passwordvalidate.Target(),
		activation_mattermost_pbkdf2hash.Target(),
		activation_mattermost_publiclinkhash.Target(),
		mattermost.Target(),
	}
	targets = append(targets, pragma.Targets()...)
	batch := &harness.BatchResult{}
	t.Cleanup(func() {
		t.Log(batch.SummaryTable())
	})
	for _, target := range targets {
		target := target
		t.Run(target.Name, func(t *testing.T) {
			if target.SkipReason != "" {
				batch.Record(harness.BatchEntry{
					Target: target.Name,
					Status: harness.BatchSkipped,
					Stage:  "skip",
					Error:  target.SkipReason,
				})
				t.Skip(target.SkipReason)
			}
			start := time.Now()
			runTarget(t, cluster, runID, target)
			duration := time.Since(start)
			status := harness.BatchPass
			if t.Failed() {
				status = harness.BatchE2EFail
			}
			batch.Record(harness.BatchEntry{
				Target:   target.Name,
				Status:   status,
				Stage:    "complete",
				Duration: duration,
			})
		})
	}
}

func updateGoldenRequested() bool {
	return *updateGolden || os.Getenv(harness.EnvUpdateGolden) == "1"
}

func runTarget(t *testing.T, cluster harness.Cluster, runID string, target harness.TargetCase) {
	t.Helper()
	perTargetTimeout := harness.DefaultPerTargetTimeout
	ctx, cancel := context.WithTimeout(context.Background(), perTargetTimeout)
	defer cancel()

	tracker := harness.NewStageTracker(target.Name)

	// Log which stage was active if the context deadline fires.
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			t.Logf("TIMEOUT: %s", tracker.TimeoutMessage(perTargetTimeout))
		}
	}()

	tracker.Enter(0, "cluster-ensure")
	if err := cluster.Ensure(ctx); err != nil {
		t.Fatalf("%v", harness.StageError(0, target.Name, harness.KindHarness, "cluster ensure failed: %v", err))
	}

	deployer := harness.Deployer{Cluster: cluster, Target: target.Name}
	baselineNS := harness.Namespace("baseline", target.Name, runID)
	liftedNS := harness.Namespace("lifted", target.Name, runID)

	// Deferred cleanup: use t.Cleanup + context.Background() to guarantee
	// namespace deletion even on panic, timeout, or t.Fatal. Runs after all
	// defers in this function have completed.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		_ = deployer.DeleteNamespace(cleanupCtx, baselineNS, 30*time.Second)
		_ = deployer.DeleteNamespace(cleanupCtx, liftedNS, 30*time.Second)
	})

	// Recover from panics so one target cannot crash the batch test process.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic in target %s: %v", target.Name, r)
		}
	}()

	tracker.Enter(0, "create-namespaces")
	if err := deployer.CreateNamespace(ctx, baselineNS); err != nil {
		t.Fatalf("%v", err)
	}
	if target.StopAtStage > 4 {
		if err := deployer.CreateNamespace(ctx, liftedNS); err != nil {
			t.Fatalf("%v", err)
		}
	}

	builder := harness.ImageBuilder{Cluster: cluster, Target: target.Name, SourceDirs: target.SourceDirs}
	if target.Dockerfile != "" {
		tracker.Enter(0, "build-baseline-image")
		if err := builder.Build(ctx, target.Dockerfile, target.ContextDir, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
		if err := builder.LoadToKind(ctx, target.ImageTag); err != nil {
			t.Fatalf("%v", err)
		}
	}

	var baselineTranscript harness.Transcript
	if len(target.BaselineManifests) > 0 {
		tracker.Enter(1, "baseline-deploy")
		if err := deployManifestPhases(ctx, deployer, baselineNS, baselineManifestPhases(target), readyTimeout(target.BaselineReadyTimeout)); err != nil {
			t.Fatalf("%v", harness.StageError(1, target.Name, harness.KindHarness, "baseline deploy failed: %v", err))
		}
		pf, err := harness.StartPortForward(ctx, target.Name, baselineNS, target.ServiceName, target.ServicePort)
		if err != nil {
			t.Fatalf("%v", err)
		}
		defer pf.Stop()
		tracker.Enter(2, "baseline-workload")
		if err := target.Workload.Setup(ctx, pf.URL); err != nil {
			t.Fatalf("%v", harness.StageError(2, target.Name, harness.KindWorkload, "baseline setup failed: %v", err))
		}
		baselineTranscript, err = target.Workload.Action(ctx, pf.URL)
		if err != nil {
			t.Fatalf("%v", harness.StageError(2, target.Name, harness.KindWorkload, "baseline action failed: %v", err))
		}
	}

	tracker.Enter(3, "compile")
	compileDir := filepath.Join(os.TempDir(), "monolift-e2e", target.Name, runID, "compile")
	compileResult, err := (harness.Compiler{OutputDir: compileDir}).Run(ctx, target)
	if err != nil {
		t.Fatalf("%v", harness.FormatCompileFailure(target, compileResult, err))
	}
	if err := applyActivationCompileResult(&target, compileResult); err != nil {
		t.Fatalf("%v", harness.StageError(4, target.Name, harness.KindCompiler, "activation artifact wiring failed: %v", err))
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

	tracker.Enter(5, "build-lifted-images")
	if err := assertExtractedDeploymentsDormant(target, compileResult.ArtifactsDir); err != nil {
		t.Fatalf("%v", harness.StageError(7, target.Name, harness.KindHarness, "recursion-safety static assertion failed: %v", err))
	}
	if err := buildAndLoadLiftedImages(ctx, builder, target, compileResult.ArtifactsDir); err != nil {
		t.Fatalf("%v", err)
	}

	tracker.Enter(7, "lifted-deploy")
	liftedManifests := liftedManifestPaths(target, compileResult.ArtifactsDir)
	if err := deployManifestPhases(ctx, deployer, liftedNS, liftedManifestPhases(target, liftedManifests), readyTimeout(target.LiftedReadyTimeout)); err != nil {
		t.Fatalf("%v", err)
	}
	liftedService := target.ServiceName
	if target.LiftedHostBuild != nil {
		liftedService = target.LiftedHostBuild.ServiceName
	}
	liftedPF, err := harness.StartPortForward(ctx, target.Name, liftedNS, liftedService, target.ServicePort)
	if err != nil {
		t.Fatalf("%v", err)
	}
	defer liftedPF.Stop()
	tracker.Enter(8, "lifted-workload")
	if len(target.LiftedExtractedServices) > 0 {
		if err := assertExtractedServicesDormantRuntime(ctx, target, liftedNS); err != nil {
			t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "recursion-safety runtime assertion failed: %v", err))
		}
	}
	if err := target.Workload.Setup(ctx, liftedPF.URL); err != nil {
		t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "lifted setup failed: %v", err))
	}
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
		if err := assertExtractedServiceLogs(ctx, target, liftedNS, expectedExtractedLogNeedles(target)); err != nil {
			t.Fatalf("%v", harness.StageError(8, target.Name, harness.KindWorkload, "lifted logs assertion failed: %v", err))
		}
	}
	tracker.Enter(9, "transcript-compare")
	if err := (harness.Transcript{}).Compare(baselineTranscript, liftedTranscript, target.Invariants); err != nil {
		t.Fatalf("%v", harness.StageError(9, target.Name, harness.KindWorkload, "transcript compare failed: %v", err))
	}
	if len(target.LiftedExtractedServices) > 0 {
		tracker.Enter(9, "env-off-fail-modes")
		if err := assertEnvOffAndFailModes(ctx, deployer, target, liftedNS, liftedPF.URL, liftedTranscript); err != nil {
			t.Fatalf("%v", harness.StageError(9, target.Name, harness.KindWorkload, "negative lifted assertions failed: %v", err))
		}
	}
}

func applyActivationCompileResult(target *harness.TargetCase, compileResult harness.CompileResult) error {
	if target.ActivationLift == nil {
		return nil
	}
	if compileResult.Activation == nil || compileResult.Activation.Manifest == nil || compileResult.Activation.Plan == nil {
		return fmt.Errorf("missing activation lift result")
	}
	plan := compileResult.Activation.Plan
	manifest := compileResult.Activation.Manifest
	if target.ActivationLift.ExpectedEnvVarPrefix != "" && manifest.Deploy.EnvVarPrefix != target.ActivationLift.ExpectedEnvVarPrefix {
		return fmt.Errorf("manifest env prefix=%q want %q", manifest.Deploy.EnvVarPrefix, target.ActivationLift.ExpectedEnvVarPrefix)
	}
	contextRoot, err := filepath.Rel(compileResult.ArtifactsDir, compileResult.SourceRoot)
	if err != nil {
		return err
	}
	hostDockerfile, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "dockerfile_host", plan.HostDockerfilePath))
	if err != nil {
		return err
	}
	hostDeployment, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "k8s_deployment_host", plan.HostDeploymentPath))
	if err != nil {
		return err
	}
	hostService, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "k8s_service_host", plan.HostServicePath))
	if err != nil {
		return err
	}
	extractedDockerfile, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "dockerfile_extracted", plan.ExtractedDockerfilePath))
	if err != nil {
		return err
	}
	extractedDeployment, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "k8s_deployment_extracted", plan.ExtractedDeploymentPath))
	if err != nil {
		return err
	}
	extractedService, err := relArtifactPath(compileResult.ArtifactsDir, artifactPath(manifest, "k8s_service_extracted", plan.ExtractedServicePath))
	if err != nil {
		return err
	}
	target.LiftedHostBuild = &harness.HostBuildSpec{
		Dockerfile:     hostDockerfile,
		ContextRoot:    contextRoot,
		ImageTag:       manifest.Deploy.HostImage,
		ServiceName:    manifest.Deploy.HostResourceName,
		DeploymentYAML: hostDeployment,
		ServiceYAML:    hostService,
	}
	target.LiftedExtractedServices = []harness.ExtractedServiceSpec{{
		Name:           manifest.Deploy.ExtractedResourceName,
		Dockerfile:     extractedDockerfile,
		ContextRoot:    contextRoot,
		ImageTag:       manifest.Deploy.ExtractedImage,
		DeploymentYAML: extractedDeployment,
		ServiceYAML:    extractedService,
		ReadinessPath:  "/healthz",
	}}
	return nil
}

func artifactPath(manifest *codegen.Manifest, kind, fallback string) string {
	if manifest != nil {
		for _, entry := range manifest.Artifacts {
			if entry.Kind == kind {
				return entry.Path
			}
		}
	}
	return fallback
}

func relArtifactPath(root, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("missing artifact path")
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("artifact path %s is outside artifacts dir %s", path, root)
	}
	return rel, nil
}

func baselineManifestPhases(target harness.TargetCase) [][]string {
	if len(target.BaselineManifestPhases) > 0 {
		return target.BaselineManifestPhases
	}
	return [][]string{target.BaselineManifests}
}

func deployManifestPhases(ctx context.Context, deployer harness.Deployer, ns string, phases [][]string, timeout time.Duration) error {
	for _, manifests := range phases {
		if len(manifests) == 0 {
			continue
		}
		if err := deployer.Apply(ctx, ns, manifests); err != nil {
			return err
		}
		if err := deployer.WaitReady(ctx, ns, timeout); err != nil {
			return err
		}
	}
	return nil
}

func readyTimeout(configured time.Duration) time.Duration {
	if configured > 0 {
		return configured
	}
	return 180 * time.Second
}

type requestWorkload interface {
	Paths() []string
	Request(ctx context.Context, host, path string) (harness.Step, error)
}

type extractedRuntime struct {
	spec   harness.ExtractedServiceSpec
	symbol string
	pf     harness.PortForward
}

type oracleRuntime struct {
	spec   harness.ExtractedServiceSpec
	symbol string
	pf     harness.PortForward
}

func assertExtractedDeploymentsDormant(target harness.TargetCase, artifactsDir string) error {
	liftEnv := regexp.MustCompile(`MONOLIFT_LIFT_[A-Z_]+:`)
	for _, service := range target.LiftedExtractedServices {
		data, err := os.ReadFile(filepath.Join(artifactsDir, service.DeploymentYAML))
		if err != nil {
			return err
		}
		if liftEnv.Match(data) || strings.Contains(string(data), "MONOLIFT_LIFT_") {
			return fmt.Errorf("%s contains MONOLIFT_LIFT_* env", service.DeploymentYAML)
		}
	}
	return nil
}

func assertExtractedServicesDormantRuntime(ctx context.Context, target harness.TargetCase, liftedNS string) error {
	services, err := startExtractedPortForwards(ctx, target, liftedNS)
	if err != nil {
		return err
	}
	defer stopExtractedPortForwards(services)

	for _, service := range services {
		before, err := readCalls(ctx, service.pf.URL)
		if err != nil {
			return err
		}
		got, err := postInvoke(ctx, service.pf.URL, invokePayload(target, service.symbol))
		if err != nil {
			return err
		}
		after, err := readCalls(ctx, service.pf.URL)
		if err != nil {
			return err
		}
		if after-before != 1 {
			return fmt.Errorf("%s direct /invoke /calls delta=%d want 1", service.spec.Name, after-before)
		}
		if target.Oracle != nil {
			want, err := target.Oracle.Invoke(oracleArgs(service.symbol, invokePayload(target, service.symbol)))
			if err != nil {
				return err
			}
			if got != want {
				return fmt.Errorf("%s direct /invoke result=%v want %v", service.spec.Name, got, want)
			}
		} else if got == nil {
			return fmt.Errorf("%s direct /invoke returned nil result", service.spec.Name)
		}
	}
	return nil
}

func runLiftedWithCallDeltas(ctx context.Context, target harness.TargetCase, liftedNS, liftedURL string) (harness.Transcript, error) {
	workload, ok := target.Workload.(requestWorkload)
	if !ok {
		return harness.Transcript{}, fmt.Errorf("target workload does not support per-request execution")
	}
	services, err := startExtractedPortForwards(ctx, target, liftedNS)
	if err != nil {
		return harness.Transcript{}, err
	}
	defer stopExtractedPortForwards(services)
	oracles, err := startOraclePortForwards(ctx, target, liftedNS)
	if err != nil {
		return harness.Transcript{}, err
	}
	defer stopOraclePortForwards(oracles)

	transcript := harness.Transcript{Steps: make([]harness.Step, 0, len(workload.Paths()))}
	totals := make(map[string]int64, len(services))
	for _, path := range workload.Paths() {
		before := make(map[string]int64, len(services))
		for _, service := range services {
			count, err := readCalls(ctx, service.pf.URL)
			if err != nil {
				return harness.Transcript{}, err
			}
			before[service.spec.Name] = count
		}
		step, err := workload.Request(ctx, liftedURL, path)
		if err != nil {
			return harness.Transcript{}, err
		}
		for _, service := range services {
			after, err := readCalls(ctx, service.pf.URL)
			if err != nil {
				return harness.Transcript{}, err
			}
			delta := after - before[service.spec.Name]
			if delta < 1 {
				return harness.Transcript{}, fmt.Errorf("%s %s /calls delta=%d want >=1", service.spec.Name, path, delta)
			}
			totals[service.spec.Name] += delta
		}
		transcript.Steps = append(transcript.Steps, step)
	}
	for _, service := range services {
		total := totals[service.spec.Name]
		if total < int64(len(workload.Paths())) || total > 50 {
			return harness.Transcript{}, fmt.Errorf("%s aggregate /calls delta=%d want %d <= total <= 50", service.spec.Name, total, len(workload.Paths()))
		}
	}
	for _, service := range services {
		if err := assertExtractedInvocations(ctx, target, service.pf.URL, service.symbol, invocationPaths(workload.Paths()), target.Oracle, oracles); err != nil {
			return harness.Transcript{}, err
		}
	}
	return transcript, nil
}

func startExtractedPortForwards(ctx context.Context, target harness.TargetCase, liftedNS string) ([]extractedRuntime, error) {
	services := make([]extractedRuntime, 0, len(target.LiftedExtractedServices))
	for _, spec := range target.LiftedExtractedServices {
		pf, err := harness.StartPortForward(ctx, target.Name, liftedNS, spec.Name, 8081)
		if err != nil {
			stopExtractedPortForwards(services)
			return nil, err
		}
		services = append(services, extractedRuntime{spec: spec, symbol: symbolForService(target, spec.Name), pf: pf})
	}
	return services, nil
}

func stopExtractedPortForwards(services []extractedRuntime) {
	for _, service := range services {
		service.pf.Stop()
	}
}

func startOraclePortForwards(ctx context.Context, target harness.TargetCase, liftedNS string) (map[string]oracleRuntime, error) {
	oracles := make(map[string]oracleRuntime, len(target.LiftedOracleServices))
	for _, spec := range target.LiftedOracleServices {
		pf, err := harness.StartPortForward(ctx, target.Name, liftedNS, spec.Name, 8081)
		if err != nil {
			stopOraclePortForwards(oracles)
			return nil, err
		}
		oracles[symbolForService(target, spec.Name)] = oracleRuntime{spec: spec, symbol: symbolForService(target, spec.Name), pf: pf}
	}
	return oracles, nil
}

func stopOraclePortForwards(oracles map[string]oracleRuntime) {
	for _, oracle := range oracles {
		oracle.pf.Stop()
	}
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
	ID                  int64          `json:"id"`
	InvocationID        string         `json:"invocation_id"`
	Params              map[string]any `json:"params"`
	P                   string         `json:"p"`
	CollapseSlashes     bool           `json:"collapse_slashes"`
	M                   string         `json:"m"`
	Result              any            `json:"result"`
	Content             string         `json:"content"`
	DefaultReadingSpeed int            `json:"default_reading_speed"`
	CjkReadingSpeed     int            `json:"cjk_reading_speed"`
	ReadingTime         int            `json:"reading_time"`
}

func assertExtractedInvocations(ctx context.Context, target harness.TargetCase, serviceURL, symbol string, expectedPaths []string, oracle harness.SymbolInvoker, oraclePods map[string]oracleRuntime) error {
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
	if len(out.Records) == 0 {
		return fmt.Errorf("%s has no invocation records", symbol)
	}
	if symbol == "cleanpath" {
		for _, path := range expectedPaths {
			found := false
			for _, record := range out.Records {
				p := record.P
				if p == "" {
					if v, ok := record.Params["p"]; ok {
						p, _ = v.(string)
					}
				}
				if p == path {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("no invocation record for path %s in %d records", path, len(out.Records))
			}
		}
	}
	if symbol == "sanitizemethod" {
		foundGET := false
		for _, record := range out.Records {
			m := record.M
			if m == "" {
				if v, ok := record.Params["m"]; ok {
					m, _ = v.(string)
				}
			}
			if m == http.MethodGet {
				foundGET = true
				break
			}
		}
		if !foundGET {
			return fmt.Errorf("no sanitizemethod invocation record for GET in %d records", len(out.Records))
		}
	}
	if oracle == nil {
		if _, ok := oraclePods[symbol]; !ok {
			return fmt.Errorf("target has no oracle for %s", symbol)
		}
	}
	for _, record := range out.Records {
		payload := invocationPayload(target, symbol, record)
		want := invocationResult(target, symbol, record)
		var got any
		var err error
		if oraclePod, ok := oraclePods[symbol]; ok {
			got, err = postInvoke(ctx, oraclePod.pf.URL, payload)
		} else {
			got, err = oracle.Invoke(oracleArgs(symbol, payload))
		}
		if err != nil {
			return err
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return fmt.Errorf("%s oracle mismatch for record=%+v: record=%v oracle=%v", symbol, record, want, got)
		}
	}
	return nil
}

func assertExtractedServiceLogs(ctx context.Context, target harness.TargetCase, ns string, expected []string) error {
	for _, service := range target.LiftedExtractedServices {
		result, err := kubectlResult(ctx, ns, "logs", "deployment/"+service.Name)
		if err != nil {
			return fmt.Errorf("kubectl logs %s: %w: %s", service.Name, err, result.Stderr)
		}
		logs := result.Stdout + result.Stderr
		if !strings.Contains(logs, "LIFT_INVOKE service="+service.Name) {
			return fmt.Errorf("%s logs missing LIFT_INVOKE service line", service.Name)
		}
		for _, needle := range expected {
			if service.Name == "monolift-extracted-sanitizemethod" && strings.HasPrefix(needle, "/") {
				continue
			}
			if !strings.Contains(logs, needle) {
				return fmt.Errorf("%s logs missing %q", service.Name, needle)
			}
		}
	}
	return nil
}

func expectedExtractedLogNeedles(target harness.TargetCase) []string {
	if target.Name == "caddy" {
		return []string{"/static/hello.txt", "/proxy", "/headers"}
	}
	return nil
}

func assertEnvOffAndFailModes(ctx context.Context, deployer harness.Deployer, target harness.TargetCase, ns, liftedURL string, envOnTranscript harness.Transcript) error {
	services, err := startExtractedPortForwards(ctx, target, ns)
	if err != nil {
		return err
	}
	defer stopExtractedPortForwards(services)

	hostDeployment := liftedHostDeployment(target)
	hostService := liftedHostService(target)
	if err := kubectl(ctx, ns, append([]string{"set", "env", "deployment/" + hostDeployment}, liftedEnvOffArgs(target)...)...); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/"+hostDeployment, "--timeout=120s"); err != nil {
		return err
	}
	if err := waitForLiftedHostReady(ctx, target, ns, 120*time.Second); err != nil {
		return err
	}
	envOffPF, err := harness.StartPortForward(ctx, target.Name, ns, hostService, target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, envOffPF.URL); err != nil {
		envOffPF.Stop()
		return err
	}
	before := make(map[string]int64, len(services))
	for _, service := range services {
		count, err := readCalls(ctx, service.pf.URL)
		if err != nil {
			envOffPF.Stop()
			return err
		}
		before[service.spec.Name] = count
	}
	envOffTranscript, err := target.Workload.Action(ctx, envOffPF.URL)
	envOffPF.Stop()
	if err != nil {
		return err
	}
	for _, service := range services {
		after, err := readCalls(ctx, service.pf.URL)
		if err != nil {
			return err
		}
		if after-before[service.spec.Name] != 0 {
			return fmt.Errorf("%s env-off /calls delta=%d want 0", service.spec.Name, after-before[service.spec.Name])
		}
	}
	if err := (harness.Transcript{}).Compare(envOnTranscript, envOffTranscript, target.Invariants); err != nil {
		return fmt.Errorf("env-off transcript mismatch: %w", err)
	}

	for _, service := range target.LiftedExtractedServices {
		if err := assertFailModesForService(ctx, deployer, target, ns, service); err != nil {
			return err
		}
	}
	return setLiftedEnv(ctx, target, ns, "closed")
}

func assertFailModesForService(ctx context.Context, deployer harness.Deployer, target harness.TargetCase, ns string, service harness.ExtractedServiceSpec) error {
	if target.ActivationLift != nil {
		return assertActivationFailModesForService(ctx, deployer, target, ns, service)
	}
	if target.Name == "miniflux" {
		return assertMinifluxFailModesForService(ctx, deployer, target, ns, service)
	}
	if err := setLiftedEnv(ctx, target, ns, "closed"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	closedPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, closedPF.URL); err != nil {
		closedPF.Stop()
		return err
	}
	closedStep, err := workloadStep(ctx, target, closedPF.URL, "/headers")
	closedPF.Stop()
	if err != nil {
		return err
	}
	wantClosed := http.StatusNotFound
	if symbolForService(target, service.Name) == "sanitizemethod" {
		wantClosed = http.StatusOK
	}
	if closedStep.Status != wantClosed {
		return fmt.Errorf("%s fail-closed status=%d want %d", service.Name, closedStep.Status, wantClosed)
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 1); err != nil {
		return err
	}
	if err := assertRestoredServiceCalls(ctx, target, ns, service); err != nil {
		return err
	}

	if err := setLiftedEnv(ctx, target, ns, "open"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	openPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, openPF.URL); err != nil {
		openPF.Stop()
		return err
	}
	openStep, err := workloadStep(ctx, target, openPF.URL, "/headers")
	openPF.Stop()
	if err != nil {
		return err
	}
	if openStep.Status != http.StatusOK {
		return fmt.Errorf("%s fail-open status=%d want 200", service.Name, openStep.Status)
	}
	return scaleExtractedService(ctx, deployer, ns, service, 1)
}

func assertActivationFailModesForService(ctx context.Context, deployer harness.Deployer, target harness.TargetCase, ns string, service harness.ExtractedServiceSpec) error {
	if err := setLiftedEnv(ctx, target, ns, "closed"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	closedPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, closedPF.URL); err != nil {
		closedPF.Stop()
		return err
	}
	closedStep, err := firstWorkloadStep(ctx, target, closedPF.URL)
	closedPF.Stop()
	if err != nil {
		return err
	}
	if closedStep.Status >= 500 {
		return fmt.Errorf("%s fail-closed status=%d want non-5xx sentinel response", service.Name, closedStep.Status)
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 1); err != nil {
		return err
	}
	if err := assertRestoredServiceCalls(ctx, target, ns, service); err != nil {
		return err
	}

	if err := setLiftedEnv(ctx, target, ns, "open"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	openPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, openPF.URL); err != nil {
		openPF.Stop()
		return err
	}
	openStep, err := firstWorkloadStep(ctx, target, openPF.URL)
	openPF.Stop()
	if err != nil {
		return err
	}
	if openStep.Status >= 500 {
		return fmt.Errorf("%s fail-open status=%d want local fallback non-5xx response", service.Name, openStep.Status)
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 1); err != nil {
		return err
	}
	return assertRestoredServiceCalls(ctx, target, ns, service)
}

func assertMinifluxFailModesForService(ctx context.Context, deployer harness.Deployer, target harness.TargetCase, ns string, service harness.ExtractedServiceSpec) error {
	if err := setLiftedEnv(ctx, target, ns, "closed"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	closedPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, closedPF.URL); err != nil {
		closedPF.Stop()
		return err
	}
	closedStep, err := firstWorkloadStep(ctx, target, closedPF.URL)
	closedPF.Stop()
	if err != nil {
		return err
	}
	if closedStep.Status != http.StatusCreated {
		return fmt.Errorf("%s fail-closed status=%d want 201", service.Name, closedStep.Status)
	}
	if got, ok := readingTimeFromStep(closedStep); !ok || got != -1 {
		return fmt.Errorf("%s fail-closed reading_time=%d ok=%v want -1", service.Name, got, ok)
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 1); err != nil {
		return err
	}
	if err := assertRestoredServiceCalls(ctx, target, ns, service); err != nil {
		return err
	}

	if err := setLiftedEnv(ctx, target, ns, "open"); err != nil {
		return err
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 0); err != nil {
		return err
	}
	openPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	if err := target.Workload.Setup(ctx, openPF.URL); err != nil {
		openPF.Stop()
		return err
	}
	openStep, err := firstWorkloadStep(ctx, target, openPF.URL)
	openPF.Stop()
	if err != nil {
		return err
	}
	if openStep.Status != http.StatusCreated {
		return fmt.Errorf("%s fail-open status=%d want 201", service.Name, openStep.Status)
	}
	if got, ok := readingTimeFromStep(openStep); !ok || got <= 0 {
		return fmt.Errorf("%s fail-open reading_time=%d ok=%v want positive", service.Name, got, ok)
	}
	if err := scaleExtractedService(ctx, deployer, ns, service, 1); err != nil {
		return err
	}
	return assertRestoredServiceCalls(ctx, target, ns, service)
}

func readingTimeFromStep(step harness.Step) (int, bool) {
	body, ok := step.BodyJSON.(map[string]any)
	if !ok {
		return 0, false
	}
	switch value := body["reading_time"].(type) {
	case int:
		return value, true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func assertRestoredServiceCalls(ctx context.Context, target harness.TargetCase, ns string, service harness.ExtractedServiceSpec) error {
	caddyPF, err := harness.StartPortForward(ctx, target.Name, ns, liftedHostService(target), target.ServicePort)
	if err != nil {
		return err
	}
	defer caddyPF.Stop()
	servicePF, err := harness.StartPortForward(ctx, target.Name, ns, service.Name, 8081)
	if err != nil {
		return err
	}
	defer servicePF.Stop()

	// Retry a few times to allow k8s endpoint propagation after scale-up.
	var lastDelta int64
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(3 * time.Second)
		}
		before, err := readCalls(ctx, servicePF.URL)
		if err != nil {
			return err
		}
		if err := target.Workload.Setup(ctx, caddyPF.URL); err != nil {
			return err
		}
		if target.Name == "miniflux" || target.ActivationLift != nil {
			if _, err := firstWorkloadStep(ctx, target, caddyPF.URL); err != nil {
				return err
			}
		} else {
			if _, err := workloadStep(ctx, target, caddyPF.URL, "/headers"); err != nil {
				return err
			}
		}
		after, err := readCalls(ctx, servicePF.URL)
		if err != nil {
			return err
		}
		lastDelta = after - before
		if lastDelta >= 1 {
			return nil
		}
	}
	return fmt.Errorf("%s restored /calls delta=%d want >=1", service.Name, lastDelta)
}

func setLiftedEnv(ctx context.Context, target harness.TargetCase, ns, failMode string) error {
	if err := kubectl(ctx, ns, append([]string{"set", "env", "deployment/" + liftedHostDeployment(target)}, liftedEnvOnArgs(target, failMode)...)...); err != nil {
		return err
	}
	if err := kubectl(ctx, ns, "rollout", "status", "deployment/"+liftedHostDeployment(target), "--timeout=120s"); err != nil {
		return err
	}
	return waitForLiftedHostReady(ctx, target, ns, 120*time.Second)
}

func waitForLiftedHostReady(ctx context.Context, target harness.TargetCase, ns string, timeout time.Duration) error {
	deployment := liftedHostDeployment(target)
	if err := waitForReadyPodForApp(ctx, target, ns, deployment, timeout); err != nil {
		return err
	}
	return waitForServiceEndpoints(ctx, target, ns, liftedHostService(target), timeout)
}

func waitForReadyPodForApp(ctx context.Context, target harness.TargetCase, ns, app string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	jsonpath := `{range .items[*]}{.metadata.name} {.status.phase} {.status.conditions[?(@.type=="Ready")].status}{"\n"}{end}`
	for {
		result, err := kubectlResult(ctx, ns, "get", "pods", "-l", "app="+app, "-o", "jsonpath="+jsonpath)
		if err == nil {
			for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 3 && fields[1] == "Running" && fields[len(fields)-1] == "True" {
					return nil
				}
			}
			last = strings.TrimSpace(result.Stdout)
			if last == "" {
				last = "no matching pods"
			}
		} else {
			last = result.Stderr
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s app %s has no ready pod after %s: %s", target.Name, app, timeout, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func waitForServiceEndpoints(ctx context.Context, target harness.TargetCase, ns, service string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last string
	for {
		result, err := kubectlResult(ctx, ns, "get", "endpoints", service, "-o", "jsonpath={.subsets[*].addresses[*].ip}")
		if err == nil && strings.TrimSpace(result.Stdout) != "" {
			return nil
		}
		if err != nil {
			last = result.Stderr
		} else {
			last = "no endpoint addresses"
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s service %s endpoints not ready after %s: %s", target.Name, service, timeout, last)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func liftedHostDeployment(target harness.TargetCase) string {
	if target.LiftedHostBuild != nil && target.LiftedHostBuild.ServiceName != "" {
		return target.LiftedHostBuild.ServiceName
	}
	return target.ServiceName
}

func liftedHostService(target harness.TargetCase) string {
	return liftedHostDeployment(target)
}

func liftedEnvOffArgs(target harness.TargetCase) []string {
	if target.ActivationLift != nil {
		return []string{activationEnvPrefix(target) + "-"}
	}
	switch target.Name {
	case "miniflux":
		return []string{"MONOLIFT_LIFT_ESTIMATEREADINGTIME-"}
	default:
		return []string{"MONOLIFT_LIFT_CLEANPATH-", "MONOLIFT_LIFT_SANITIZEMETHOD-"}
	}
}

func liftedEnvOnArgs(target harness.TargetCase, failMode string) []string {
	if target.ActivationLift != nil {
		serviceName := target.LiftedExtractedServices[0].Name
		prefix := activationEnvPrefix(target)
		return []string{
			prefix + "=on",
			"MONOLIFT_LIFT_FAILMODE=" + failMode,
			activationEndpointEnv(prefix) + "=http://" + serviceName + ":8081/invoke",
		}
	}
	switch target.Name {
	case "miniflux":
		return []string{
			"MONOLIFT_LIFT_ESTIMATEREADINGTIME=on",
			"MONOLIFT_LIFT_FAILMODE=" + failMode,
			"MONOLIFT_LIFT_ESTIMATEREADINGTIME_ENDPOINT=http://monolift-extracted-estimatereadingtime:8081/invoke",
		}
	default:
		return []string{
			"MONOLIFT_LIFT_CLEANPATH=on",
			"MONOLIFT_LIFT_SANITIZEMETHOD=on",
			"MONOLIFT_LIFT_FAILMODE=" + failMode,
			"MONOLIFT_LIFT_CLEANPATH_ENDPOINT=http://monolift-extracted-cleanpath:8081/invoke",
			"MONOLIFT_LIFT_SANITIZEMETHOD_ENDPOINT=http://monolift-extracted-sanitizemethod:8081/invoke",
		}
	}
}

func activationEnvPrefix(target harness.TargetCase) string {
	if target.ActivationLift != nil && target.ActivationLift.ExpectedEnvVarPrefix != "" {
		return target.ActivationLift.ExpectedEnvVarPrefix
	}
	symbol := "LIFTED"
	if len(target.LiftedExtractedServices) > 0 {
		symbol = strings.ToUpper(symbolForService(target, target.LiftedExtractedServices[0].Name))
	}
	return "MONOLIFT_LIFT_" + symbol
}

func activationEndpointEnv(prefix string) string {
	env := strings.TrimPrefix(prefix, "MONOLIFT_LIFT_")
	return "MONOLIFT_" + env + "_ENDPOINT"
}

func scaleExtractedService(ctx context.Context, deployer harness.Deployer, ns string, service harness.ExtractedServiceSpec, replicas int) error {
	if err := kubectl(ctx, ns, "scale", "deployment/"+service.Name, fmt.Sprintf("--replicas=%d", replicas)); err != nil {
		return err
	}
	if replicas == 0 {
		if err := kubectl(ctx, ns, "wait", "--for=delete", "pod", "-l", "app="+service.Name, "--timeout=120s"); err != nil {
			return err
		}
		return deployer.WaitReady(ctx, ns, 120*time.Second)
	}
	return deployer.WaitReady(ctx, ns, 120*time.Second)
}

func postInvoke(ctx context.Context, serviceURL string, payload map[string]any) (any, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, serviceURL+"/invoke", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("POST /invoke status=%d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if value, ok := out["reading_time"]; ok {
		return value, nil
	}
	return out["result"], nil
}

func invokePayload(target harness.TargetCase, symbol string) map[string]any {
	if payload, ok := target.InvokePayloads[symbol]; ok {
		return clonePayload(payload)
	}
	if target.ActivationLift != nil && len(target.ActivationLift.DirectInvocationProbePayload) > 0 {
		return clonePayload(target.ActivationLift.DirectInvocationProbePayload)
	}
	switch symbol {
	case "sanitizemethod":
		return map[string]any{"m": http.MethodGet}
	case "estimatereadingtime":
		return map[string]any{"content": "<p>direct invocation reading time content</p>", "default_reading_speed": 200, "cjk_reading_speed": 500}
	default:
		return map[string]any{"p": "/static/hello.txt", "collapse_slashes": true}
	}
}

func invocationPayload(target harness.TargetCase, symbol string, record invocationRecord) map[string]any {
	if len(record.Params) > 0 {
		return clonePayload(record.Params)
	}
	switch symbol {
	case "sanitizemethod":
		return map[string]any{"m": record.M}
	case "estimatereadingtime":
		return map[string]any{"content": record.Content, "default_reading_speed": record.DefaultReadingSpeed, "cjk_reading_speed": record.CjkReadingSpeed}
	default:
		return map[string]any{"p": record.P, "collapse_slashes": record.CollapseSlashes}
	}
}

func invocationResult(target harness.TargetCase, symbol string, record invocationRecord) any {
	if resultMap, ok := record.Result.(map[string]any); ok {
		if value, nested := resultMap["result"]; nested {
			return value
		}
		if len(resultMap) == 1 {
			for _, value := range resultMap {
				return value
			}
		}
		return resultMap
	}
	if symbol == "estimatereadingtime" {
		return record.ReadingTime
	}
	return record.Result
}

func clonePayload(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload))
	for key, value := range payload {
		out[key] = value
	}
	return out
}

func oracleArgs(symbol string, payload map[string]any) map[string]any {
	args := make(map[string]any, len(payload)+1)
	args["symbol"] = symbol
	for key, value := range payload {
		args[key] = value
	}
	return args
}

func symbolForService(target harness.TargetCase, name string) string {
	if symbol, ok := target.ServiceSymbols[name]; ok {
		return symbol
	}
	if strings.Contains(name, "sanitizemethod") {
		return "sanitizemethod"
	}
	if strings.Contains(name, "estimatereadingtime") {
		return "estimatereadingtime"
	}
	if target.ActivationLift != nil {
		name = strings.TrimPrefix(name, "monolift-extracted-")
		name = strings.TrimPrefix(name, "monolift-oracle-")
		name = strings.TrimPrefix(name, "monolift-")
		return strings.ReplaceAll(name, "-", "")
	}
	return "cleanpath"
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
	for _, service := range target.LiftedOracleServices {
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
	for _, service := range target.LiftedOracleServices {
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
	spec := *target.LiftedHostBuild
	if target.ActivationLift == nil {
		paths = append(paths, filepath.Join(artifactsDir, spec.DeploymentYAML), filepath.Join(artifactsDir, spec.ServiceYAML))
	}
	for _, service := range target.LiftedExtractedServices {
		paths = append(paths,
			filepath.Join(artifactsDir, service.DeploymentYAML),
			filepath.Join(artifactsDir, service.ServiceYAML),
		)
	}
	for _, service := range target.LiftedOracleServices {
		paths = append(paths,
			filepath.Join(artifactsDir, service.DeploymentYAML),
			filepath.Join(artifactsDir, service.ServiceYAML),
		)
	}
	if target.ActivationLift != nil {
		paths = append(paths, filepath.Join(artifactsDir, spec.DeploymentYAML), filepath.Join(artifactsDir, spec.ServiceYAML))
	}
	return paths
}

func liftedManifestPhases(target harness.TargetCase, manifests []string) [][]string {
	if target.LiftedHostBuild == nil {
		return [][]string{manifests}
	}
	infra := make([]string, 0, len(target.BaselineManifests))
	for _, manifest := range target.BaselineManifests {
		base := filepath.Base(manifest)
		if base == "deployment.yaml" || base == "service.yaml" {
			continue
		}
		infra = append(infra, manifest)
	}
	phases := make([][]string, 0, 2+len(target.LiftedExtractedServices))
	if len(infra) > 0 {
		phases = append(phases, infra)
	}
	phases = append(phases, manifests[len(infra):len(infra)+2])
	offset := len(infra) + 2
	for offset < len(manifests) {
		next := offset + 2
		if next > len(manifests) {
			next = len(manifests)
		}
		phases = append(phases, manifests[offset:next])
		offset = next
	}
	return phases
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
