package codegen

import (
	"bytes"
	"context"
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
	artifacts := append(artifactsFromRendered("server", serverFiles), artifactsFromRendered("client_stub", clientFiles)...)
	if _, err := WriteArtifacts(plan, artifacts, ""); err != nil {
		t.Fatal(err)
	}
	if stat, err := os.Stat(plan.ManifestPath); err != nil || stat.Size() == 0 {
		t.Fatalf("manifest not written or empty: stat=%+v err=%v", stat, err)
	}

	serverPkg := "./cmd/" + plan.ServiceName
	runGo(t, plan.OutputDir, "build", serverPkg)
	runGo(t, plan.OutputDir, "vet", serverPkg)
	clientPkg := plan.CutPoint.PackagePath
	runGo(t, plan.SourceModuleRoot, "build", clientPkg)
	runGo(t, plan.SourceModuleRoot, "vet", clientPkg)
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
	artifacts := append(artifactsFromRendered("server", serverFiles), artifactsFromRendered("client_stub", clientFiles)...)
	if _, err := WriteArtifacts(plan, artifacts, ""); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(filepath.Dir(plan.ServerPath), "network_roundtrip_test.go"), `package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"miniflux.app/v2/internal/reader/sanitizer"
)

func TestSanitizeHTMLNetworkRoundTrip(t *testing.T) {
	state, err := initState()
	if err != nil {
		t.Fatal(err)
	}
	listener := newPipeListener("monolift.test")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			client, server := net.Pipe()
			select {
			case listener.conns <- server:
				return client, nil
			case <-listener.done:
				_ = client.Close()
				_ = server.Close()
				return nil, net.ErrClosed
			case <-ctx.Done():
				_ = client.Close()
				_ = server.Close()
				return nil, ctx.Err()
			}
		},
	}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() {
		http.DefaultTransport = previousTransport
		transport.CloseIdleConnections()
	}()
	server := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: NewHandler(state)},
	}
	server.Start()
	defer server.Close()
	t.Setenv("MONOLIFT_LIFT_SANITIZEHTML", "on")
	t.Setenv("MONOLIFT_LIFT_FAILMODE", "closed")
	t.Setenv("MONOLIFT_SANITIZEHTML_ENDPOINT", server.URL+"/invoke")

	options := &sanitizer.SanitizerOptions{OpenLinksInNewTab: true}
	input := "<p>Hello</p><script>alert(1)</script><a href=\"/next\">next</a>"
	got := sanitizer.SanitizeHTML_monolift("http://example.org/base/", input, options)
	want := sanitizer.SanitizeHTML("http://example.org/base/", input, options)
	if got != want {
		t.Fatalf("remote SanitizeHTML = %q, want %q", got, want)
	}
	if got == "" {
		t.Fatal("remote SanitizeHTML returned empty output")
	}
}

type pipeListener struct {
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
	addr  pipeAddr
}

func newPipeListener(addr string) *pipeListener {
	return &pipeListener{
		conns: make(chan net.Conn),
		done:  make(chan struct{}),
		addr:  pipeAddr(addr),
	}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *pipeListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return l.addr
}

type pipeAddr string

func (a pipeAddr) Network() string { return "tcp" }

func (a pipeAddr) String() string { return string(a) }
`)
	runGo(t, filepath.Dir(plan.ServerPath), "test", "-run=TestSanitizeHTMLNetworkRoundTrip", "-count=1", ".")
}

func TestRefreshFeedCodegenCompilesWithStateReconstruction(t *testing.T) {
	root := repoRoot(t)
	sourceCopy := copySourceToTemp(t, filepath.Join(root, "evaluation", "miniflux"))
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
	artifacts := append(artifactsFromRendered("server", serverFiles), artifactsFromRendered("client_stub", clientFiles)...)
	if _, err := WriteArtifacts(plan, artifacts, ""); err != nil {
		t.Fatal(err)
	}
	serverSource, err := os.ReadFile(plan.ServerPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`sql.Open("postgres", os.Getenv("DATABASE_URL"))`,
		"storage.NewStorage(storeDB)",
		"Store *storage.Storage",
	} {
		if !strings.Contains(string(serverSource), want) {
			t.Fatalf("generated RefreshFeed server missing %q:\n%s", want, serverSource)
		}
	}

	serverPkg := "./cmd/" + plan.ServiceName
	runGo(t, plan.OutputDir, "build", serverPkg)
	runGo(t, plan.OutputDir, "vet", serverPkg)
	clientPkg := plan.CutPoint.PackagePath
	runGo(t, plan.SourceModuleRoot, "build", clientPkg)
	runGo(t, plan.SourceModuleRoot, "vet", clientPkg)
}

func TestLiftCommandSmokeDeterministic(t *testing.T) {
	root := repoRoot(t)
	origSource := filepath.Join(root, "evaluation", "miniflux")
	tests := []struct {
		name    string
		relTarget string
		service string
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

			clientDir := filepath.Join(source, tt.relClientDir)
			clientPath := filepath.Join(clientDir, "monolift_lift_"+envServiceName(sanitizeServiceName(tt.service))+".go")
			opts := LiftOptions{
				Source:      source,
				Target:      filepath.Join(source, tt.relTarget),
				Output:      output,
				ServiceName: tt.service,
			}
			runLiftForSmoke(t, opts)
			first := snapshotGeneratedFiles(t, output, clientPath)
			runLiftForSmoke(t, opts)
			second := snapshotGeneratedFiles(t, output, clientPath)
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
