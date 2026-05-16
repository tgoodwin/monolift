package codegen

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
)

func TestSanitizeHTMLFullPipeline(t *testing.T) {
	root := repoRoot(t)
	source := copySourceToTemp(t, filepath.Join(root, "evaluation", "miniflux"))
	target := filepath.Join(source, "internal", "reader", "sanitizer", "sanitizer.go") + ":217"
	output := filepath.Join(source, ".monolift-sanitizehtml")

	opts := LiftOptions{
		Source:      source,
		Target:      target,
		Output:      output,
		ServiceName: "sanitizehtml",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	result, err := runActivation(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Found || result.Path == nil {
		t.Fatalf("activation path not found: %+v", result)
	}
	cut, err := activation.AnalyzeCut(result, nil)
	if err != nil {
		t.Fatal(err)
	}
	if cut.Recommended == nil {
		t.Fatal("cut recommendation = nil")
	}
	if cut.Recommended.NodeKey.FuncName != "SanitizeHTML" {
		t.Fatalf("recommended cut = %s, want SanitizeHTML", cut.Recommended.NodeKey.FuncName)
	}
	report, err := buildExtractionReport(opts, cut)
	if err != nil {
		t.Fatal(err)
	}
	cutAdmission := AdmitCut(report, *cut)
	if !cutAdmission.Accepted {
		t.Fatalf("cut admission refused: %s", cutAdmission.Error())
	}
	plan, err := BuildPlan(report, *cut)
	if err != nil {
		t.Fatal(err)
	}
	if err := attachIncomingCall(plan, result.Path, cut.Recommended.Step); err != nil {
		t.Fatal(err)
	}
	applyLiftOptions(plan, opts)
	plan.Admission = AdmitPlan(plan, cutAdmission)
	if !plan.Admission.Accepted {
		t.Fatalf("plan admission refused: %s", plan.Admission.Error())
	}
	serverFiles, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	dockerFiles, err := RenderDockerfiles(plan)
	if err != nil {
		t.Fatal(err)
	}
	kubernetesFiles, err := RenderKubernetes(plan)
	if err != nil {
		t.Fatal(err)
	}
	artifacts := artifactsFromRendered("server", serverFiles)
	artifacts = append(artifacts,
		Artifact{Path: plan.ExtractedDockerfilePath, Kind: "dockerfile_extracted", Content: dockerFiles[plan.ExtractedDockerfilePath]},
		Artifact{Path: plan.HostDockerfilePath, Kind: "dockerfile_host", Content: dockerFiles[plan.HostDockerfilePath]},
		Artifact{Path: plan.ExtractedDeploymentPath, Kind: "k8s_deployment_extracted", Content: kubernetesFiles[plan.ExtractedDeploymentPath]},
		Artifact{Path: plan.ExtractedServicePath, Kind: "k8s_service_extracted", Content: kubernetesFiles[plan.ExtractedServicePath]},
		Artifact{Path: plan.HostDeploymentPath, Kind: "k8s_deployment_host", Content: kubernetesFiles[plan.HostDeploymentPath]},
		Artifact{Path: plan.HostServicePath, Kind: "k8s_service_host", Content: kubernetesFiles[plan.HostServicePath]},
	)
	entries, err := writeArtifactFiles(plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	stubContent := clientFiles[plan.ClientPath]
	patchedFile, err := PatchCutFunction(plan, stubContent)
	if err != nil {
		t.Fatal(err)
	}
	// Write adapter after patching so monoliftOriginal<Func> exists.
	adapterFiles, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}
	for adapterPath, content := range adapterFiles {
		if err := os.WriteFile(adapterPath, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	entries = append(entries, ManifestEntry{Path: plan.ClientPath, Kind: "client_stub"})
	manifest, err := writeManifest(plan, entries, patchedFile)
	if err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(plan.ManifestPath); err != nil || stat.Size() == 0 {
		t.Fatalf("manifest not written or empty: stat=%+v err=%v", stat, err)
	}
	for _, path := range []string{
		plan.ServerPath,
		plan.ClientPath,
		plan.ExtractedDockerfilePath,
		plan.HostDockerfilePath,
		plan.ExtractedDeploymentPath,
		plan.ExtractedServicePath,
		plan.HostDeploymentPath,
		plan.HostServicePath,
	} {
		if stat, err := os.Stat(path); err != nil || stat.Size() == 0 {
			t.Fatalf("artifact not written or empty: path=%s stat=%+v err=%v", path, stat, err)
		}
	}
	data, err := os.ReadFile(plan.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Deploy.EndpointURL == "" || decoded.Deploy.EnvVarPrefix != "MONOLIFT_LIFT_SANITIZEHTML" {
		t.Fatalf("manifest deploy metadata = %+v", decoded.Deploy)
	}
	assertManifestKinds(t, manifest.Artifacts, []string{
		"server",
		"client_stub",
		"dockerfile_extracted",
		"dockerfile_host",
		"k8s_deployment_extracted",
		"k8s_service_extracted",
		"k8s_deployment_host",
		"k8s_service_host",
	})

	serverPkg := "./cmd/" + plan.ServiceName
	runGo(t, plan.OutputDir, "build", serverPkg)
	runGo(t, plan.OutputDir, "vet", serverPkg)
	clientPkg := plan.CutPoint.PackagePath
	runGo(t, plan.SourceModuleRoot, "build", clientPkg)
	runGo(t, plan.SourceModuleRoot, "vet", clientPkg)
}

func assertManifestKinds(t *testing.T, entries []ManifestEntry, kinds []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Kind] = true
	}
	for _, kind := range kinds {
		if !seen[kind] {
			t.Fatalf("manifest missing artifact kind %s: %+v", kind, entries)
		}
	}
}

func TestSanitizeHTMLNetworkRoundTrip(t *testing.T) {
	root := repoRoot(t)
	sourceCopy := copySourceToTemp(t, filepath.Join(root, "evaluation", "miniflux"))
	fixture := SanitizeHTMLFixtureWithSource(root, sourceCopy)
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(sourceCopy, ".monolift-sanitizehtml-network")
	applyLiftOptions(plan, LiftOptions{Output: output, ServiceName: "sanitizehtml"})
	plan.Admission = AdmissionVerdict{Accepted: true, Reasons: []string{"network test"}}

	serverFiles, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	serverArtifacts := artifactsFromRendered("server", serverFiles)
	if _, err := writeArtifactFiles(plan, serverArtifacts); err != nil {
		t.Fatal(err)
	}
	stubContent := clientFiles[plan.ClientPath]
	if _, err := PatchCutFunction(plan, stubContent); err != nil {
		t.Fatal(err)
	}
	// Write adapter after patching so monoliftOriginal<Func> exists.
	adapterFiles, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}
	for adapterPath, content := range adapterFiles {
		if err := os.WriteFile(adapterPath, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, filepath.Join(filepath.Dir(plan.ServerPath), "network_roundtrip_test.go"), `package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"miniflux.app/v2/internal/reader/sanitizer"
)

func TestSanitizeHTMLNetworkRoundTrip(t *testing.T) {
	options := &sanitizer.SanitizerOptions{OpenLinksInNewTab: true}
	input := "<p>Hello</p><script>alert(1)</script><a href=\"/next\">next</a>"
	want := sanitizer.SanitizeHTML("http://example.org/base/", input, options)
	if want == "" {
		t.Fatal("local SanitizeHTML returned empty output")
	}

	state, err := initState()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(state))
	defer server.Close()

	payload, _ := json.Marshal(map[string]any{
		"base_url": "http://example.org/base/",
		"input": input,
		"sanitizer_options": options,
	})
	resp, err := http.Post(server.URL+"/invoke", "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("POST /invoke status = %d", resp.StatusCode)
	}
	var result struct { Result string `+"`json:\"result\"`"+` }
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Result != want {
		t.Fatalf("POST /invoke result = %q, want %q", result.Result, want)
	}

	callsResp, err := http.Get(server.URL+"/calls")
	if err != nil {
		t.Fatal(err)
	}
	defer callsResp.Body.Close()
	var calls callsResponse
	if err := json.NewDecoder(callsResp.Body).Decode(&calls); err != nil {
		t.Fatal(err)
	}
	if calls.Count < 1 {
		t.Fatalf("/calls count = %d, want >= 1", calls.Count)
	}
}
`)
	runGo(t, filepath.Dir(plan.ServerPath), "test", "-run=TestSanitizeHTMLNetworkRoundTrip", "-count=1", ".")
}

func TestRefreshFeedCodegenCompilesWithStateReconstruction(t *testing.T) {
	root := repoRoot(t)
	sourceCopy := copySourceToTemp(t, filepath.Join(root, "evaluation", "miniflux"))
	assertGoModRequires(t, sourceCopy, "github.com/lib/pq")
	fixture := RefreshFeedFixtureWithSource(root, sourceCopy)
	plan, err := BuildPlan(fixture.Report, fixture.Cut)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(sourceCopy, ".monolift-refreshfeed")
	applyLiftOptions(plan, LiftOptions{Output: output, ServiceName: "refreshfeed"})
	plan.Admission = AdmissionVerdict{Accepted: true, Reasons: []string{"refreshfeed codegen test"}}

	serverFiles, err := RenderServer(plan)
	if err != nil {
		t.Fatal(err)
	}
	clientFiles, err := RenderClient(plan)
	if err != nil {
		t.Fatal(err)
	}
	serverArtifacts := artifactsFromRendered("server", serverFiles)
	if _, err := writeArtifactFiles(plan, serverArtifacts); err != nil {
		t.Fatal(err)
	}
	stubContent := clientFiles[plan.ClientPath]
	if _, err := PatchCutFunction(plan, stubContent); err != nil {
		t.Fatal(err)
	}
	// Write adapter after patching so monoliftOriginal<Func> exists.
	adapterFiles, err := RenderAdapter(plan)
	if err != nil {
		t.Fatal(err)
	}
	for adapterPath, content := range adapterFiles {
		if err := os.WriteFile(adapterPath, content, 0644); err != nil {
			t.Fatal(err)
		}
	}
	serverSource, err := os.ReadFile(plan.ServerPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
		"storage.NewStorage(storeDB)",
	} {
		if !strings.Contains(string(serverSource), want) {
			t.Fatalf("generated RefreshFeed server missing %q:\n%s", want, serverSource)
		}
	}
	if !strings.Contains(string(serverSource), "Store") || !strings.Contains(string(serverSource), "*storage.Storage") {
		t.Fatalf("generated RefreshFeed server missing Store *storage.Storage field:\n%s", serverSource)
	}

	serverPkg := "./cmd/" + plan.ServiceName
	runGo(t, plan.OutputDir, "build", serverPkg)
	runGo(t, plan.OutputDir, "vet", serverPkg)
	clientPkg := plan.CutPoint.PackagePath
	runGo(t, plan.SourceModuleRoot, "build", clientPkg)
	runGo(t, plan.SourceModuleRoot, "vet", clientPkg)
}

func assertGoModRequires(t *testing.T, moduleRoot, modulePath string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), modulePath) {
		t.Fatalf("go.mod missing %s", modulePath)
	}
}

func TestTargetFileForReportResolvesRelativeTargetAgainstSource(t *testing.T) {
	got, err := targetFileForReport("/repo/evaluation/miniflux", "internal/reader/handler/handler.go")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean("/repo/evaluation/miniflux/internal/reader/handler/handler.go")
	if got != want {
		t.Fatalf("target file = %q, want %q", got, want)
	}
}

func TestLiftCommandSmokeDeterministic(t *testing.T) {
	root := repoRoot(t)
	origSource := filepath.Join(root, "evaluation", "miniflux")
	tests := []struct {
		name         string
		relTarget    string
		service      string
		relClientDir string
	}{
		{
			name:         "sanitizehtml",
			relTarget:    "internal/reader/sanitizer/sanitizer.go:217",
			service:      "smoke-sanitizehtml",
			relClientDir: "internal/reader/sanitizer",
		},
		{
			name:         "refreshfeed",
			relTarget:    "internal/reader/handler/handler.go:207",
			service:      "smoke-refreshfeed",
			relClientDir: "internal/reader/handler",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := copySourceToTemp(t, origSource)
			output := filepath.Join(source, ".monolift-smoke-"+tt.name)

			opts := LiftOptions{
				Source:      source,
				Target:      filepath.Join(source, tt.relTarget),
				Output:      output,
				ServiceName: tt.service,
			}
			runLiftForSmoke(t, opts)
			first := snapshotGeneratedFiles(t, output)
			runLiftForSmoke(t, opts)
			second := snapshotGeneratedFiles(t, output)
			if len(first) == 0 {
				t.Fatal("no generated files captured")
			}
			if len(first) != len(second) {
				t.Fatalf("generated file count changed from %d to %d", len(first), len(second))
			}
			for path, firstBytes := range first {
				secondBytes, ok := second[path]
				if !ok {
					t.Fatalf("generated file %s missing after second run", path)
				}
				if len(firstBytes) == 0 {
					t.Fatalf("generated file %s is empty", path)
				}
				if !bytes.Equal(firstBytes, secondBytes) {
					t.Fatalf("generated file %s changed across repeated runs", path)
				}
			}
		})
	}
}

func runLiftForSmoke(t *testing.T, opts LiftOptions) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	if err := RunLift(ctx, opts); err != nil {
		t.Fatal(err)
	}
}

func snapshotGeneratedFiles(t *testing.T, output string, extraPaths ...string) map[string][]byte {
	t.Helper()
	snapshot := map[string][]byte{}
	err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(rel)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range extraPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		snapshot["external/"+filepath.Base(path)] = data
	}
	return snapshot
}

func runGo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Env = withEnvValue(os.Environ(), "GOCACHE", "/tmp/monolift-gocache")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v failed in %s: %v\n%s", args, dir, err, out)
	}
}

func copySourceToTemp(t *testing.T, source string) string {
	t.Helper()
	tmp := t.TempDir()
	cmd := exec.Command("cp", "-a", source+"/.", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to copy %s to %s: %v\n%s", source, tmp, err, out)
	}
	return tmp
}
