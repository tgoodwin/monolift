package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tgoodwin/monolift/pkg/compiler"
	compilerdiagnostics "github.com/tgoodwin/monolift/pkg/compiler/diagnostics"
	"github.com/tgoodwin/monolift/pkg/compiler/entrypath"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit"
	"github.com/tgoodwin/monolift/pkg/compiler/transport/emit/liftpatch"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// startProfiling is gated by MONOLIFT_PROFILE_DIR. When set, we write
// {target}.cpu.pprof and {target}.heap.pprof plus {target}.memstats.json
// to that directory. Cheap and reversible — no flag plumbing.
func startProfiling(target string) func() {
	dir := os.Getenv("MONOLIFT_PROFILE_DIR")
	if dir == "" {
		return func() {}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "profile mkdir %s: %v\n", dir, err)
		return func() {}
	}
	cpuPath := filepath.Join(dir, target+".cpu.pprof")
	cpuFile, err := os.Create(cpuPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "profile cpu create: %v\n", err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(cpuFile); err != nil {
		fmt.Fprintf(os.Stderr, "profile cpu start: %v\n", err)
		_ = cpuFile.Close()
		return func() {}
	}
	start := time.Now()
	return func() {
		pprof.StopCPUProfile()
		_ = cpuFile.Close()
		// Heap profile: snapshot at peak (after analysis, before exit).
		runtime.GC()
		heapPath := filepath.Join(dir, target+".heap.pprof")
		if hf, err := os.Create(heapPath); err == nil {
			_ = pprof.Lookup("heap").WriteTo(hf, 0)
			_ = hf.Close()
		}
		// MemStats snapshot for human-readable summary.
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		summary := map[string]any{
			"target":          target,
			"elapsed_seconds": time.Since(start).Seconds(),
			"heap_alloc_mb":   float64(ms.HeapAlloc) / (1 << 20),
			"heap_sys_mb":     float64(ms.HeapSys) / (1 << 20),
			"total_alloc_mb":  float64(ms.TotalAlloc) / (1 << 20),
			"sys_mb":          float64(ms.Sys) / (1 << 20),
			"num_gc":          ms.NumGC,
			"pause_total_ms":  float64(ms.PauseTotalNs) / 1e6,
			"num_goroutines":  runtime.NumGoroutine(),
			"go_max_procs":    runtime.GOMAXPROCS(0),
		}
		if data, err := json.MarshalIndent(summary, "", "  "); err == nil {
			_ = os.WriteFile(filepath.Join(dir, target+".memstats.json"), append(data, '\n'), 0o644)
		}
	}
}

// e2e-compile loads the full host module via go/packages and consumes ~3 GB
// of RAM. Concurrent invocations from parallel `go test` runs OOM-killed the
// agent driving SPRINT-0019. Take an exclusive flock at startup so concurrent
// invocations queue instead of stacking. The lock releases automatically when
// the process exits (no explicit unlock needed).
func acquireStartupLock() {
	lockPath := filepath.Join(os.TempDir(), "monolift-e2e-compile.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e-compile: open lock %s: %v\n", lockPath, err)
		os.Exit(2)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		fmt.Fprintf(os.Stderr, "e2e-compile: flock %s: %v\n", lockPath, err)
		os.Exit(2)
	}
	// Intentionally leak the FD: closing it would release the lock. The OS
	// reclaims it at process exit.
}

type sourceFlags []string

func (s *sourceFlags) String() string {
	return fmt.Sprint([]string(*s))
}

func (s *sourceFlags) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	target := flag.String("target", "", "target fixture name")
	output := flag.String("output", "", "output directory")
	entryPathProbePackage := flag.String("entrypath-probe-package", "", "optional package pattern for entrypath probe")
	var sources sourceFlags
	var entryPathProbeRoots sourceFlags
	flag.Var(&sources, "source", "source directory")
	flag.Var(&entryPathProbeRoots, "entrypath-probe-root", "optional entrypath probe region root")
	flag.Parse()

	if *target == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: e2e-compile --target=<name> --output=<dir> [--source=<dir>...]")
		os.Exit(2)
	}

	stopProfiling := startProfiling(*target)
	defer stopProfiling()

	acquireStartupLock()

	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "e2e-compile target %s: no --source paths\n", *target)
		os.Exit(1)
	}
	resolvedSources := resolveSources(sources)
	if err := emitPragmaReport(*target, *output, resolvedSources); err != nil {
		fmt.Fprintf(os.Stderr, "e2e-compile target %s: %v\n", *target, err)
		os.Exit(1)
	}
	if len(entryPathProbeRoots) > 0 {
		if err := emitEntryPathProbe(*output, resolvedSources, *entryPathProbePackage, entryPathProbeRoots); err != nil {
			fmt.Fprintf(os.Stderr, "e2e-compile target %s: %v\n", *target, err)
			os.Exit(1)
		}
	}
	if emitsLiftedTree(*target) {
		if err := emitLiftedTree(*target, *output, resolvedSources); err != nil {
			fmt.Fprintf(os.Stderr, "e2e-compile target %s: %v\n", *target, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stdout, "e2e-compile parsed %s to %s\n", *target, *output)
}

func emitsLiftedTree(target string) bool {
	switch target {
	case "caddy", "miniflux":
		return true
	default:
		return false
	}
}

func emitLiftedTree(target, output string, sources []string) error {
	pragmas, _, err := compiler.Parse(sources)
	if err != nil {
		return err
	}
	if target == "miniflux" && len(pragmas) == 0 {
		pragma, err := minifluxReadingTimePragma(sources)
		if err != nil {
			return err
		}
		pragmas = []*compiler.Pragma{pragma}
	}
	_, artifacts, _, err := compiler.ExtractWithTransport(sources, pragmas)
	if err != nil {
		return err
	}
	if len(artifacts) == 0 {
		return nil
	}
	hostSource, err := findHostSource(target, sources)
	if err != nil {
		return err
	}

	lifted := filepath.Join(output, "lifted")
	hostPatch := filepath.Join(lifted, "host-patch")
	if err := os.RemoveAll(lifted); err != nil {
		return err
	}
	if err := os.CopyFS(hostPatch, os.DirFS(hostSource)); err != nil {
		return fmt.Errorf("copy host-patch: %w", err)
	}
	if target == "caddy" {
		if err := os.CopyFS(filepath.Join(lifted, "static"), os.DirFS(filepath.Join(repoRoot(), "test", "e2e", "targets", "caddy", "static"))); err != nil {
			return fmt.Errorf("copy static assets: %w", err)
		}
	}

	manifest := []string{}
	if target == "caddy" {
		manifest = append(manifest, "static/hello.txt")
	}
	for _, artifact := range artifacts {
		if len(artifact.HostPatchOps) > 0 {
			if err := applyHostPatchArtifact(lifted, hostPatch, artifact); err != nil {
				return err
			}
			for _, op := range artifact.HostPatchOps {
				packageDir, err := packageDirFor(hostPatch, op.PackageImportPath)
				if err != nil {
					return err
				}
				relPkg := mustRel(hostPatch, packageDir)
				for _, path := range op.GeneratedFiles {
					manifest = append(manifest, filepath.ToSlash(filepath.Join("host-patch", relPkg, path)))
				}
				manifest = append(manifest, filepath.ToSlash(filepath.Join("host-patch", relPkg, "LIFTPATCH.json")))
			}
			continue
		}
		for path, data := range artifact.Files {
			outRoot := lifted
			manifestPath := path
			if strings.HasPrefix(path, "cmd/") {
				outRoot = hostPatch
				manifestPath = filepath.ToSlash(filepath.Join("host-patch", path))
			}
			out := filepath.Join(outRoot, filepath.FromSlash(path))
			if strings.HasPrefix(path, "cmd/monolift-extracted-") {
				if _, err := os.Stat(filepath.Dir(out)); err == nil {
					return fmt.Errorf("cmd target collision: %s already exists", filepath.Dir(out))
				} else if !os.IsNotExist(err) {
					return err
				}
			}
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				return err
			}
			manifest = append(manifest, manifestPath)
		}
	}
	extraFiles := map[string]string{
		"Dockerfile.host": hostDockerfile(target),
	}
	if target == "miniflux" {
		extraFiles["host-patch/cmd/monolift-oracle-estimatereadingtime/main.go"] = minifluxOracleMain()
		extraFiles["Dockerfile.oracle-estimatereadingtime"] = oracleDockerfile("monolift-oracle-estimatereadingtime", "oracle")
		extraFiles["manifests/oracle-estimatereadingtime-deployment.yaml"] = oracleDeployment()
		extraFiles["manifests/oracle-estimatereadingtime-service.yaml"] = oracleService()
	}
	deploymentName, deployment := hostDeployment(target)
	serviceName, service := hostService(target)
	if deploymentName != "" {
		extraFiles[deploymentName] = deployment
	}
	if serviceName != "" {
		extraFiles[serviceName] = service
	}
	for path, data := range extraFiles {
		out := filepath.Join(lifted, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, []byte(data), 0o644); err != nil {
			return err
		}
		manifest = append(manifest, path)
	}
	sort.Strings(manifest)
	data, err := json.MarshalIndent(struct {
		Files []string `json:"files"`
	}{Files: manifest}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(lifted, "MANIFEST.json"), append(data, '\n'), 0o644)
}

func applyHostPatchArtifact(lifted, hostPatch string, artifact emit.Artifact) error {
	useRegionPatch := len(artifact.HostPatchOps) > 1
	for _, op := range artifact.HostPatchOps {
		if op.ReceiverType != "" {
			useRegionPatch = true
			break
		}
	}
	if useRegionPatch {
		return applyRegionHostPatchArtifact(hostPatch, artifact)
	}
	for _, op := range artifact.HostPatchOps {
		packageDir, err := packageDirFor(hostPatch, op.PackageImportPath)
		if err != nil {
			return err
		}
		generated := make([]liftpatch.GeneratedFile, 0, len(op.GeneratedFiles))
		for _, rel := range op.GeneratedFiles {
			data, ok := artifact.Files[rel]
			if !ok {
				return fmt.Errorf("host patch generated file %s missing from artifact", rel)
			}
			data = withPackageDecl(data, filepath.Base(packageDir))
			generated = append(generated, liftpatch.GeneratedFile{
				Path:    filepath.Join(packageDir, rel),
				Content: data,
			})
		}
		_, err = liftpatch.PatchSymbolBody(liftpatch.PatchRequest{
			ModuleRoot:        hostPatch,
			PackageImportPath: op.PackageImportPath,
			PackageDir:        packageDir,
			FuncName:          op.FuncName,
			ExpectedSignature: op.ExpectedSignature,
			PreludeSpec:       liftpatch.PreludeSpec{GoSource: op.PreludeSource},
			GeneratedFiles:    generated,
			SentinelIdent:     op.SentinelIdent,
		})
		if err != nil {
			return err
		}
	}
	_ = lifted
	return nil
}

func applyRegionHostPatchArtifact(hostPatch string, artifact emit.Artifact) error {
	req := liftpatch.RegionPatchRequest{RegionName: artifact.Manifest.ServiceName}
	for _, op := range artifact.HostPatchOps {
		packageDir, err := packageDirFor(hostPatch, op.PackageImportPath)
		if err != nil {
			return err
		}
		generated := make([]liftpatch.GeneratedFile, 0, len(op.GeneratedFiles))
		for _, rel := range op.GeneratedFiles {
			data, ok := artifact.Files[rel]
			if !ok {
				return fmt.Errorf("host patch generated file %s missing from artifact", rel)
			}
			data = withPackageDecl(data, filepath.Base(packageDir))
			generated = append(generated, liftpatch.GeneratedFile{
				Path:    filepath.Join(packageDir, rel),
				Content: data,
			})
		}
		req.Symbols = append(req.Symbols, liftpatch.PatchSymbolRequest{
			PackageImportPath: op.PackageImportPath,
			PackageDir:        packageDir,
			FuncName:          op.FuncName,
			ReceiverType:      op.ReceiverType,
			ExpectedSignature: op.ExpectedSignature,
			Prelude:           liftpatch.PreludeSpec{GoSource: op.PreludeSource},
			GeneratedFiles:    generated,
			SentinelIdent:     op.SentinelIdent,
		})
	}
	result, err := liftpatch.PatchRegion(req)
	if err != nil {
		return err
	}
	if result.Refused != nil {
		return fmt.Errorf("region patch refused: %s: %s", result.Refused.Kind, result.Refused.Message)
	}
	return nil
}

func packageDirFor(hostPatch, importPath string) (string, error) {
	moduleDirs := map[string]string{
		"github.com/caddyserver/caddy/v2":            "caddy",
		"github.com/mattermost/mattermost/server/v8": "mattermost/server",
		"github.com/pocketbase/pocketbase":           "pocketbase",
		"miniflux.app/v2":                            "miniflux",
	}
	for modulePath := range moduleDirs {
		rel, ok := strings.CutPrefix(importPath, modulePath)
		if !ok {
			continue
		}
		rel = strings.TrimPrefix(rel, "/")
		return filepath.Join(hostPatch, filepath.FromSlash(rel)), nil
	}
	return "", fmt.Errorf("unsupported package import path %s", importPath)
}

func withPackageDecl(src []byte, packageName string) []byte {
	return []byte(strings.Replace(string(src), "package caddyhttp\n", "package "+packageName+"\n", 1))
}

func mustRel(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return rel
}

func findHostSource(target string, sources []string) (string, error) {
	wantBase := target
	if target == "miniflux" {
		wantBase = "miniflux"
	}
	for _, source := range sources {
		clean := filepath.Clean(source)
		if filepath.Base(clean) == wantBase && strings.Contains(filepath.ToSlash(clean), "evaluation/"+wantBase) {
			return clean, nil
		}
	}
	return "", fmt.Errorf("evaluation/%s source not found", wantBase)
}

func hostDockerfile(target string) string {
	if target == "miniflux" {
		return `FROM golang:1.26.0 AS builder

WORKDIR /src/miniflux
COPY ./host-patch /src/miniflux
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/miniflux .

FROM alpine:3.20
RUN adduser -D -H miniflux
COPY --from=builder /out/miniflux /usr/bin/miniflux
USER miniflux
EXPOSE 8080
ENTRYPOINT ["/usr/bin/miniflux"]
`
	}
	return `FROM golang:1.25 AS builder

WORKDIR /src/caddy
COPY ./host-patch /src/caddy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/caddy ./cmd/caddy

FROM alpine:3.20
RUN adduser -D -H caddy
COPY --from=builder /out/caddy /usr/bin/caddy
COPY ./static /srv/static
USER caddy
EXPOSE 8080
ENTRYPOINT ["/usr/bin/caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
`
}

func minifluxOracleMain() string {
	return `package main

import (
	"encoding/json"
	"log"
	"net/http"

	"miniflux.app/v2/internal/reader/readingtime"
)

type invokeRequest struct {
	Content             string ` + "`json:\"content\"`" + `
	DefaultReadingSpeed int    ` + "`json:\"default_reading_speed\"`" + `
	CjkReadingSpeed     int    ` + "`json:\"cjk_reading_speed\"`" + `
}

type invokeResponse struct {
	ReadingTime int ` + "`json:\"reading_time\"`" + `
}

func main() {
	http.HandleFunc("/invoke", handleInvoke)
	http.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invokeResponse{
		ReadingTime: readingtime.EstimateReadingTime(in.Content, in.DefaultReadingSpeed, in.CjkReadingSpeed),
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}
`
}

func oracleDockerfile(commandTarget, binaryName string) string {
	return fmt.Sprintf(`FROM golang:1.26.0 AS builder

WORKDIR /src
COPY . /src
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/%[2]s ./cmd/%[1]s

FROM gcr.io/distroless/static
COPY --from=builder /out/%[2]s /%[2]s
EXPOSE 8081
ENTRYPOINT ["/%[2]s"]
`, commandTarget, binaryName)
}

func hostDeployment(target string) (string, string) {
	if target == "miniflux" {
		return "manifests/miniflux-lifted-deployment.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: miniflux-lifted
spec:
  replicas: 1
  selector:
    matchLabels:
      app: miniflux-lifted
  template:
    metadata:
      labels:
        app: miniflux-lifted
    spec:
      containers:
        - name: miniflux
          image: monolift-e2e/miniflux-lifted:e2e
          imagePullPolicy: IfNotPresent
          env:
            - name: DATABASE_URL
              value: postgres://miniflux:miniflux@postgres:5432/miniflux?sslmode=disable
            - name: RUN_MIGRATIONS
              value: "1"
            - name: CREATE_ADMIN
              value: "1"
            - name: ADMIN_USERNAME
              value: admin
            - name: ADMIN_PASSWORD
              value: password
            - name: FETCHER_ALLOW_PRIVATE_NETWORKS
              value: "1"
            - name: LISTEN_ADDR
              value: 0.0.0.0:8080
            - name: MONOLIFT_LIFT_ESTIMATEREADINGTIME
              value: "on"
            - name: MONOLIFT_LIFT_FAILMODE
              value: "closed"
            - name: MONOLIFT_LIFT_ESTIMATEREADINGTIME_ENDPOINT
              value: "http://monolift-extracted-estimatereadingtime:8081/invoke"
          ports:
            - containerPort: 8080
          readinessProbe:
            httpGet:
              path: /healthcheck
              port: 8080
            periodSeconds: 2
`
	}
	if target != "caddy" {
		return "", ""
	}
	return "manifests/caddy-lifted-deployment.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: caddy-lifted
spec:
  replicas: 1
  selector:
    matchLabels:
      app: caddy-lifted
  template:
    metadata:
      labels:
        app: caddy-lifted
    spec:
      containers:
        - name: caddy
          image: monolift-e2e/caddy-lifted:e2e
          imagePullPolicy: IfNotPresent
          env:
            - name: MONOLIFT_LIFT_CLEANPATH
              value: "on"
            - name: MONOLIFT_LIFT_SANITIZEMETHOD
              value: "on"
            - name: MONOLIFT_LIFT_FAILMODE
              value: "closed"
            - name: MONOLIFT_LIFT_CLEANPATH_ENDPOINT
              value: "http://monolift-extracted-cleanpath:8081/invoke"
            - name: MONOLIFT_LIFT_SANITIZEMETHOD_ENDPOINT
              value: "http://monolift-extracted-sanitizemethod:8081/invoke"
          ports:
            - containerPort: 8080
          volumeMounts:
            - name: caddyfile
              mountPath: /etc/caddy
          readinessProbe:
            tcpSocket:
              port: 8080
            periodSeconds: 2
      volumes:
        - name: caddyfile
          configMap:
            name: caddyfile
`
}

func oracleDeployment() string {
	return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: monolift-oracle-estimatereadingtime
spec:
  replicas: 1
  selector:
    matchLabels:
      app: monolift-oracle-estimatereadingtime
  template:
    metadata:
      labels:
        app: monolift-oracle-estimatereadingtime
    spec:
      containers:
        - name: oracle
          image: monolift-e2e/oracle-estimatereadingtime:e2e
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 8081
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8081
            periodSeconds: 2
`
}

func oracleService() string {
	return `apiVersion: v1
kind: Service
metadata:
  name: monolift-oracle-estimatereadingtime
spec:
  selector:
    app: monolift-oracle-estimatereadingtime
  ports:
    - name: http
      port: 8081
      targetPort: 8081
`
}

func hostService(target string) (string, string) {
	if target == "miniflux" {
		return "manifests/miniflux-lifted-service.yaml", `apiVersion: v1
kind: Service
metadata:
  name: miniflux-lifted
spec:
  selector:
    app: miniflux-lifted
  ports:
    - name: http
      port: 8080
      targetPort: 8080
`
	}
	if target != "caddy" {
		return "", ""
	}
	return "manifests/caddy-lifted-service.yaml", `apiVersion: v1
kind: Service
metadata:
  name: caddy-lifted
spec:
  selector:
    app: caddy-lifted
  ports:
    - name: http
      port: 8080
      targetPort: 8080
`
}

func repoRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
	}
}

func resolveSources(sources []string) []string {
	resolved := make([]string, 0, len(sources))
	root := repoRoot()
	for _, source := range sources {
		if filepath.IsAbs(source) {
			resolved = append(resolved, source)
			continue
		}
		if _, err := os.Stat(source); err == nil {
			resolved = append(resolved, source)
			continue
		}
		resolved = append(resolved, filepath.Join(root, source))
	}
	return resolved
}

func emitPragmaReport(target, output string, sources []string) error {
	pragmas, diagnostics, err := compiler.Parse(sources)
	if err != nil {
		return err
	}
	if target == "mattermost" && len(pragmas) > 0 {
		if err := resolveMattermostOverlayPragmas(sources, pragmas); err != nil {
			return err
		}
	}
	if usesPragmaOnlyReport(target, diagnostics) {
		report := buildPragmaReport(target, pragmas, diagnostics)
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(output, "closure-report.json"), append(data, '\n'), 0o644)
	}
	if target == "miniflux" && len(pragmas) == 0 {
		pragma, err := minifluxReadingTimePragma(sources)
		if err != nil {
			return err
		}
		pragmas = []*compiler.Pragma{pragma}
	}
	if target == "mattermost" && len(pragmas) == 0 {
		pragma, err := mattermostHubPragma(sources)
		if err != nil {
			return err
		}
		pragmas = []*compiler.Pragma{pragma}
	}
	report, diagnostics, err := compiler.Extract(sources, pragmas)
	if err != nil {
		return err
	}
	report.Diagnostics = mustTranslateDiagnostics(diagnostics, compilerdiagnostics.Options{ModuleRoot: report.BuildConfig.ModuleRoot})
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "closure-report.json"), append(data, '\n'), 0o644)
}

func emitEntryPathProbe(output string, sources []string, pattern string, roots []string) error {
	moduleRoot := firstModuleRoot(sources)
	if moduleRoot == "" {
		return fmt.Errorf("entrypath probe: no module root found in %v", sources)
	}
	if pattern == "" {
		pattern = "."
	}
	cfg := &packages.Config{
		Mode:  packages.LoadAllSyntax | packages.NeedModule,
		Dir:   moduleRoot,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return fmt.Errorf("entrypath probe load: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("entrypath probe load reported package errors")
	}
	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	mainPkg := firstMainPackage(ssaPkgs)
	if mainPkg == nil {
		return fmt.Errorf("entrypath probe: main package not found for %s", pattern)
	}
	regionRoots, err := resolveEntryPathRoots(prog, roots)
	if err != nil {
		return err
	}
	result, err := entrypath.Probe(prog, mainPkg, regionRoots)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, "entrypath-probe.json"), append(data, '\n'), 0o644)
}

func firstModuleRoot(sources []string) string {
	for _, source := range sources {
		if _, err := os.Stat(filepath.Join(source, "go.mod")); err == nil {
			return source
		}
	}
	return ""
}

func firstMainPackage(pkgs []*ssa.Package) *ssa.Package {
	for _, pkg := range pkgs {
		if pkg != nil && pkg.Pkg != nil && pkg.Pkg.Name() == "main" {
			return pkg
		}
	}
	if len(pkgs) > 0 {
		return pkgs[0]
	}
	return nil
}

func resolveEntryPathRoots(prog *ssa.Program, specs []string) ([]*ssa.Function, error) {
	functions := make([]*ssa.Function, 0, len(ssautil.AllFunctions(prog)))
	for fn := range ssautil.AllFunctions(prog) {
		if fn != nil {
			functions = append(functions, fn)
		}
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].String() < functions[j].String() })
	var roots []*ssa.Function
	for _, spec := range specs {
		fn := findEntryPathFunction(functions, spec)
		if fn == nil {
			return nil, fmt.Errorf("entrypath probe root %q not found", spec)
		}
		roots = append(roots, fn)
	}
	return roots, nil
}

func findEntryPathFunction(functions []*ssa.Function, spec string) *ssa.Function {
	for _, fn := range functions {
		if entryPathFunctionMatches(fn, spec) {
			return fn
		}
	}
	return nil
}

func entryPathFunctionMatches(fn *ssa.Function, spec string) bool {
	objectName := entryPathFunctionObjectName(fn)
	packagePath := ""
	if fn.Package() != nil && fn.Package().Pkg != nil {
		packagePath = fn.Package().Pkg.Path()
	}
	return spec == objectName ||
		spec == packagePath+"."+objectName ||
		strings.HasSuffix(packagePath+"."+objectName, "."+spec) ||
		fn.String() == spec
}

func entryPathFunctionObjectName(fn *ssa.Function) string {
	if fn == nil {
		return ""
	}
	if fn.Signature != nil && fn.Signature.Recv() != nil {
		return entryPathReceiverName(fn.Signature.Recv().Type()) + "." + fn.Name()
	}
	return fn.Name()
}

func entryPathReceiverName(typ types.Type) string {
	switch t := typ.(type) {
	case *types.Pointer:
		return "(*" + entryPathReceiverName(t.Elem()) + ")"
	case *types.Named:
		return t.Obj().Name()
	default:
		return types.TypeString(typ, func(*types.Package) string { return "" })
	}
}

func resolveMattermostOverlayPragmas(sources []string, pragmas []*compiler.Pragma) error {
	platformDir := ""
	for _, source := range sources {
		candidate := filepath.Join(source, "channels", "app", "platform")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			platformDir = candidate
			break
		}
	}
	if platformDir == "" {
		return fmt.Errorf("mattermost platform source not found in %v", sources)
	}
	for _, pragma := range pragmas {
		if pragma == nil {
			continue
		}
		switch pragma.DeclName {
		case "Hub":
			pragma.Surface = compiler.SurfaceStruct
			pragma.DeclKind = "struct"
			pragma.DeclIdentity = "Hub"
			pragma.Span.Filename = filepath.Join(platformDir, "web_hub.go")
			pragma.Span.Line = 77
			pragma.Span.EndLine = 101
		case "WebConn":
			pragma.Surface = compiler.SurfaceStruct
			pragma.DeclKind = "struct"
			pragma.DeclIdentity = "WebConn"
			pragma.Span.Filename = filepath.Join(platformDir, "web_conn.go")
			pragma.Span.Line = 88
			pragma.Span.EndLine = 149
		}
	}
	return nil
}

func usesPragmaOnlyReport(target string, diagnostics []compiler.Diagnostic) bool {
	switch target {
	case "pragma-parse", "pragma-unknown-key", "pragma-invalid-surface", "pragma-misattached", "pragma-duplicate", "pragma-unknown-verb", "pragma-v1-deprecated":
		return true
	}
	for _, diagnostic := range diagnostics {
		switch diagnostic.Code {
		case compiler.CodeParse, compiler.CodeUnknownKey, compiler.CodeInvalidKeyForSurface, compiler.CodeMisattached, compiler.CodeDuplicate, compiler.CodeRegionConflict, compiler.CodeUnknownVerb, compiler.CodeV1Deprecated:
			return true
		}
	}
	return false
}

func minifluxReadingTimePragma(sources []string) (*compiler.Pragma, error) {
	for _, source := range sources {
		path := filepath.Join(source, "internal", "reader", "readingtime", "readingtime.go")
		if _, err := os.Stat(path); err == nil {
			return &compiler.Pragma{
				Name:    "estimate-reading-time",
				Surface: compiler.SurfaceFunction,
				Options: map[string]string{
					"state":     "stateless",
					"transport": "http-json",
					"verdict":   "accept",
				},
				Span: compiler.Span{
					Filename: path,
					Line:     16,
					EndLine:  17,
				},
				DeclName: "EstimateReadingTime",
				DeclKind: "function",
			}, nil
		}
	}
	return nil, fmt.Errorf("miniflux readingtime source not found in %v", sources)
}

func mattermostHubPragma(sources []string) (*compiler.Pragma, error) {
	for _, source := range sources {
		path := filepath.Join(source, "channels", "app", "platform", "web_hub.go")
		if _, err := os.Stat(path); err == nil {
			return &compiler.Pragma{
				Name:    "mattermost-hub",
				Surface: compiler.SurfaceStruct,
				Options: map[string]string{
					"methods": "Start,Broadcast,Register,Unregister,CheckConn",
					"state":   "affinity",
					"verdict": "accept",
				},
				Span: compiler.Span{
					Filename: path,
					Line:     77,
					EndLine:  101,
				},
				DeclName: "Hub",
				DeclKind: "struct",
			}, nil
		}
	}
	return nil, fmt.Errorf("mattermost web hub source not found in %v", sources)
}

func buildPragmaReport(target string, pragmas []*compiler.Pragma, diagnostics []compiler.Diagnostic) reportv2.Report {
	pragma := firstPragma(pragmas)
	verdict := verdictForDiagnostics(diagnostics)
	surface := string(compiler.SurfaceFunction)
	name := target
	options := map[string]string{"verdict": verdict}
	span := reportv2.SourceSpan{FileRelativePath: "unknown", LineStart: 1, LineEnd: 1}
	diagOptions := compilerdiagnostics.Options{ModuleRoot: repoRoot()}

	if pragma != nil {
		surface = string(pragma.Surface)
		if surface == string(compiler.SurfaceUnknown) {
			surface = string(compiler.SurfaceFunction)
		}
		if pragma.Name != "" {
			name = pragma.Name
		}
		for key, value := range pragma.Options {
			options[key] = value
		}
		options["verdict"] = verdict
		span = mustTranslateSpan(pragma.Span, diagOptions)
	} else if len(diagnostics) > 0 {
		span = mustTranslateSpan(diagnostics[0].Span, diagOptions)
	}

	identity := reportv2.SymbolIdentity{
		ModulePath:  "github.com/tgoodwin/monolift/test/e2e",
		PackagePath: "pragma-fixture/" + target,
		ObjectName:  name,
		Kind:        rootKindForSurface(surface),
	}
	return reportv2.Report{
		SchemaVersion: reportv2.SchemaVersion,
		BuildConfig: reportv2.BuildConfig{
			GOOS:          "linux",
			GOARCH:        "amd64",
			CGOEnabled:    false,
			BuildTags:     []string{},
			ModuleRoot:    repoRoot(),
			WorkspaceMode: "single-module",
			Tests:         false,
		},
		Analysis: reportv2.Analysis{
			Algorithm:     "stub-pragma-parser",
			Deterministic: true,
		},
		Pragma: reportv2.Pragma{
			Name:    name,
			Surface: surface,
			Span:    span,
			Options: options,
		},
		Root: reportv2.Root{
			Identity:          identity,
			ExposedOperations: []reportv2.SymbolIdentity{identity},
		},
		Closure: reportv2.Closure{
			IncludedSymbols: []reportv2.SymbolEntry{{Identity: identity, Span: span, RuleIDs: []string{"SPRINT-0005-pragma-only"}}},
			ExcludedSymbols: []reportv2.SymbolEntry{},
			WiringPaths:     []reportv2.WiringPath{},
		},
		State:        []reportv2.StateItem{},
		Adapters:     []reportv2.Adapter{},
		ExternalDeps: []reportv2.ExternalDep{},
		Pruning: reportv2.Pruning{
			Bounded:  true,
			Frontier: []reportv2.SymbolEntry{},
		},
		Diagnostics: mustTranslateDiagnostics(diagnostics, diagOptions),
	}
}

func firstPragma(pragmas []*compiler.Pragma) *compiler.Pragma {
	for _, pragma := range pragmas {
		if pragma != nil {
			return pragma
		}
	}
	return nil
}

func verdictForDiagnostics(diagnostics []compiler.Diagnostic) string {
	hasWarning := false
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == compiler.SeverityError {
			return "refuse-blocking"
		}
		if diagnostic.Code == compiler.CodeV1Deprecated {
			hasWarning = true
		}
	}
	if hasWarning {
		return "accept-with-warnings"
	}
	return "accept"
}

func rootKindForSurface(surface string) string {
	switch surface {
	case string(compiler.SurfaceInterface):
		return "interface"
	case string(compiler.SurfaceStruct):
		return "type"
	case string(compiler.SurfaceMethod):
		return "method"
	default:
		return "function"
	}
}

func mustTranslateDiagnostics(diags []compiler.Diagnostic, opts compilerdiagnostics.Options) []reportv2.Diagnostic {
	translated, err := compilerdiagnostics.TranslateAll(diags, opts)
	if err != nil {
		sanitized := make([]compiler.Diagnostic, len(diags))
		copy(sanitized, diags)
		for i := range sanitized {
			sanitized[i].Span.Column = 1
			sanitized[i].Span.EndColumn = 1
		}
		translated, retryErr := compilerdiagnostics.TranslateAll(sanitized, opts)
		if retryErr != nil {
			panic(err)
		}
		return translated
	}
	return translated
}

func mustTranslateSpan(span compiler.Span, opts compilerdiagnostics.Options) reportv2.SourceSpan {
	translated, err := compilerdiagnostics.TranslateSpan(span, opts)
	if err != nil {
		panic(err)
	}
	return translated
}
