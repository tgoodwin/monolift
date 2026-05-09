package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

type Compiler struct {
	Path      string
	OutputDir string
}

type CompileResult struct {
	ArtifactsDir string
	Report       *reportv2.Report
	RawStderr    string
	RawStdout    string
	ExitCode     int
	Activation   *codegen.LiftResult
	SourceRoot   string
}

func (c Compiler) Run(ctx context.Context, target TargetCase) (CompileResult, error) {
	outputDir := c.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(os.TempDir(), "monolift-e2e", target.Name, fmt.Sprintf("%d", time.Now().UnixNano()), "compile")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return CompileResult{ArtifactsDir: outputDir, ExitCode: -1}, err
	}
	if target.ActivationLift != nil {
		return c.runActivationLift(ctx, target, outputDir)
	}

	path := c.Path
	if path == "" {
		path = CompilerPath()
	}
	args := []string{"--target=" + target.Name, "--output=" + outputDir}
	for _, sourceDir := range target.SourceDirs {
		args = append(args, "--source="+sourceDir)
	}
	if target.EntryPathProbePackage != "" {
		args = append(args, "--entrypath-probe-package="+target.EntryPathProbePackage)
	}
	for _, root := range target.EntryPathProbeRoots {
		args = append(args, "--entrypath-probe-root="+root)
	}
	result, err := RunCommand(ctx, path, args...)
	compileResult := CompileResult{
		ArtifactsDir: outputDir,
		RawStderr:    result.Stderr,
		RawStdout:    result.Stdout,
		ExitCode:     result.ExitCode,
	}
	if err != nil {
		return compileResult, err
	}

	reportData, err := os.ReadFile(filepath.Join(outputDir, "closure-report.json"))
	if err != nil {
		return compileResult, err
	}
	report, err := reportv2.Parse(reportData)
	if err != nil {
		return compileResult, err
	}
	compileResult.Report = report
	return compileResult, nil
}

func (c Compiler) runActivationLift(ctx context.Context, target TargetCase, outputDir string) (CompileResult, error) {
	if len(target.SourceDirs) == 0 {
		return CompileResult{ArtifactsDir: outputDir, ExitCode: -1}, fmt.Errorf("activation lift target has no source dirs")
	}
	sourceRoot := FromRepoRoot(target.SourceDirs[0])
	copiedRoot := filepath.Join(outputDir, "lifted")
	if err := os.MkdirAll(copiedRoot, 0o755); err != nil {
		return CompileResult{ArtifactsDir: outputDir, ExitCode: -1, SourceRoot: copiedRoot}, err
	}
	copyResult, err := RunCommand(ctx, "cp", "-a", sourceRoot+"/.", copiedRoot)
	if err != nil {
		return CompileResult{
			ArtifactsDir: outputDir,
			RawStderr:    copyResult.Stderr,
			RawStdout:    copyResult.Stdout,
			ExitCode:     copyResult.ExitCode,
			SourceRoot:   copiedRoot,
		}, err
	}

	spec := target.ActivationLift
	liftTarget, err := activationTargetInCopy(spec.Target, copiedRoot)
	if err != nil {
		return CompileResult{ArtifactsDir: outputDir, ExitCode: -1, SourceRoot: copiedRoot}, err
	}
	result, err := codegen.RunLiftWithResult(ctx, codegen.LiftOptions{
		Source:            copiedRoot,
		Target:            liftTarget,
		Output:            ".",
		ServiceName:       spec.ServiceName,
		Deploy:            spec.Deploy,
		WriteMonolithStub: true,
	})
	compileResult := CompileResult{
		ArtifactsDir: outputDir,
		ExitCode:     0,
		Activation:   result,
		SourceRoot:   copiedRoot,
	}
	if err != nil {
		compileResult.ExitCode = 1
		return compileResult, err
	}
	if err := writeActivationSupportArtifacts(target, outputDir, copiedRoot); err != nil {
		compileResult.ExitCode = 1
		return compileResult, err
	}
	reportData, err := json.MarshalIndent(result.Report, "", "  ")
	if err != nil {
		compileResult.ExitCode = 1
		return compileResult, err
	}
	reportData = append(reportData, '\n')
	if err := os.WriteFile(filepath.Join(outputDir, "closure-report.json"), reportData, 0o644); err != nil {
		compileResult.ExitCode = 1
		return compileResult, err
	}
	compileResult.Report = &result.Report
	return compileResult, nil
}

func activationTargetInCopy(target, copiedRoot string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("activation lift target is required")
	}
	idx := strings.LastIndex(target, ":")
	if idx < 0 {
		return "", fmt.Errorf("activation lift target must be file:line, got %q", target)
	}
	file := target[:idx]
	line := target[idx+1:]
	if file == "" || line == "" {
		return "", fmt.Errorf("activation lift target must be file:line, got %q", target)
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(copiedRoot, file)
	}
	return file + ":" + line, nil
}

func writeActivationSupportArtifacts(target TargetCase, artifactsDir, sourceRoot string) error {
	if target.Name == "activation-caddy-cleanpath" {
		if err := copyActivationCaddyStatic(sourceRoot); err != nil {
			return err
		}
	}
	for _, service := range target.LiftedOracleServices {
		if err := writeActivationOracleArtifacts(target, artifactsDir, sourceRoot, service); err != nil {
			return err
		}
	}
	return nil
}

func copyActivationCaddyStatic(sourceRoot string) error {
	src := FromRepoRoot("test/e2e/targets/caddy/static")
	dst := filepath.Join(sourceRoot, "static")
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	result, err := RunCommand(context.Background(), "cp", "-a", src, dst)
	if err != nil {
		return fmt.Errorf("copy caddy static assets: %s: %w", TailLines(result.Stderr+"\n"+result.Stdout, 20), err)
	}
	return nil
}

func writeActivationOracleArtifacts(target TargetCase, artifactsDir, sourceRoot string, service ExtractedServiceSpec) error {
	mainSource, ok := activationOracleMain(target.Name, service.Name)
	if !ok {
		return fmt.Errorf("no activation oracle source for target=%s service=%s", target.Name, service.Name)
	}
	contextRoot := service.ContextRoot
	if contextRoot == "" {
		rel, err := filepath.Rel(artifactsDir, sourceRoot)
		if err != nil {
			return err
		}
		contextRoot = filepath.ToSlash(rel)
	}
	contextDir, err := activationArtifactPath(artifactsDir, contextRoot)
	if err != nil {
		return err
	}
	mainPath := filepath.Join(contextDir, "cmd", service.Name, "main.go")
	formatted, err := format.Source([]byte(mainSource))
	if err != nil {
		return fmt.Errorf("format oracle %s: %w", service.Name, err)
	}
	if err := writeActivationFile(mainPath, formatted); err != nil {
		return err
	}
	dockerfilePath, err := activationArtifactPath(artifactsDir, service.Dockerfile)
	if err != nil {
		return err
	}
	if err := writeActivationFile(dockerfilePath, []byte(activationOracleDockerfile(goModVersionForContext(contextDir), service.Name))); err != nil {
		return err
	}
	deploymentPath, err := activationArtifactPath(artifactsDir, service.DeploymentYAML)
	if err != nil {
		return err
	}
	if err := writeActivationFile(deploymentPath, []byte(activationOracleDeployment(service))); err != nil {
		return err
	}
	servicePath, err := activationArtifactPath(artifactsDir, service.ServiceYAML)
	if err != nil {
		return err
	}
	return writeActivationFile(servicePath, []byte(activationOracleService(service)))
}

func activationArtifactPath(artifactsDir, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("activation artifact path is empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("activation artifact path must be relative: %s", rel)
	}
	path := filepath.Join(artifactsDir, filepath.FromSlash(rel))
	cleanRoot, err := filepath.Abs(artifactsDir)
	if err != nil {
		return "", err
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	back, err := filepath.Rel(cleanRoot, cleanPath)
	if err != nil {
		return "", err
	}
	if back == "." || back == ".." || strings.HasPrefix(back, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("activation artifact path %s escapes %s", rel, artifactsDir)
	}
	return path, nil
}

func writeActivationFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var activationGoDirectivePattern = regexp.MustCompile(`(?m)^go[ \t]+([0-9]+(?:\.[0-9]+){1,2})[ \t]*$`)

func goModVersionForContext(contextDir string) string {
	data, err := os.ReadFile(filepath.Join(contextDir, "go.mod"))
	if err != nil {
		return "1.24"
	}
	match := activationGoDirectivePattern.FindSubmatch(data)
	if len(match) != 2 {
		return "1.24"
	}
	return string(match[1])
}

func activationOracleDockerfile(goVersion, commandName string) string {
	return fmt.Sprintf(`FROM golang:%s AS builder

WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /out/oracle ./cmd/%s

FROM gcr.io/distroless/static-debian12
COPY --from=builder /out/oracle /oracle
EXPOSE 8081
ENTRYPOINT ["/oracle"]
`, goVersion, commandName)
}

func activationOracleDeployment(service ExtractedServiceSpec) string {
	readinessPath := service.ReadinessPath
	if readinessPath == "" {
		readinessPath = "/healthz"
	}
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
        - name: oracle
          image: %s
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 8081
          readinessProbe:
            httpGet:
              path: %s
              port: 8081
            periodSeconds: 2
`, service.Name, service.Name, service.Name, service.ImageTag, readinessPath)
}

func activationOracleService(service ExtractedServiceSpec) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
spec:
  type: ClusterIP
  selector:
    app: %s
  ports:
    - name: http
      port: 8081
      targetPort: 8081
`, service.Name, service.Name)
}

func activationOracleMain(targetName, serviceName string) (string, bool) {
	switch {
	case targetName == "activation-miniflux-sanitizehtml" && serviceName == "monolift-oracle-sanitizehtml":
		return minifluxSanitizeHTMLOracleMain, true
	case targetName == "activation-caddy-cleanpath" && serviceName == "monolift-oracle-cleanpath":
		return caddyCleanPathOracleMain, true
	case targetName == "activation-gitea-pathescapesegments" && serviceName == "monolift-oracle-pathescapesegments":
		return giteaPathEscapeSegmentsOracleMain, true
	case targetName == "activation-listmonk-sanitizeuri" && serviceName == "monolift-oracle-sanitizeuri":
		return listmonkSanitizeURIOracleMain, true
	default:
		return "", false
	}
}

const minifluxSanitizeHTMLOracleMain = `package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"miniflux.app/v2/internal/reader/sanitizer"
)

type invokeRequest struct {
	BaseURL          string                      ` + "`json:\"base_url\"`" + `
	Input            string                      ` + "`json:\"input\"`" + `
	SanitizerOptions *sanitizer.SanitizerOptions ` + "`json:\"sanitizer_options\"`" + `
}

type invokeResponse struct {
	Result string ` + "`json:\"result\"`" + `
}

func main() {
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/invoke", handleInvoke)
	mux.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invokeResponse{
		Result: sanitizer.SanitizeHTML(in.BaseURL, in.Input, in.SanitizerOptions),
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}
`

const caddyCleanPathOracleMain = `package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

type invokeRequest struct {
	P               string ` + "`json:\"p\"`" + `
	CollapseSlashes bool   ` + "`json:\"collapse_slashes\"`" + `
}

type invokeResponse struct {
	Result string ` + "`json:\"result\"`" + `
}

func main() {
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/invoke", handleInvoke)
	mux.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invokeResponse{
		Result: caddyhttp.CleanPath(in.P, in.CollapseSlashes),
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}
`

const giteaPathEscapeSegmentsOracleMain = `package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"code.gitea.io/gitea/modules/util"
)

type invokeRequest struct {
	Path string ` + "`json:\"path\"`" + `
}

type invokeResponse struct {
	Result string ` + "`json:\"result\"`" + `
}

func main() {
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/invoke", handleInvoke)
	mux.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invokeResponse{
		Result: util.PathEscapeSegments(in.Path),
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}
`

const listmonkSanitizeURIOracleMain = `package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/knadh/listmonk/internal/utils"
)

type invokeRequest struct {
	U string ` + "`json:\"u\"`" + `
}

type invokeResponse struct {
	Result string ` + "`json:\"result\"`" + `
}

func main() {
	addr := os.Getenv("MONOLIFT_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/invoke", handleInvoke)
	mux.HandleFunc("/healthz", handleHealthz)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	defer r.Body.Close()
	var in invokeRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, invokeResponse{
		Result: utils.SanitizeURI(in.U),
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write json: %v", err)
	}
}
`

func FormatCompileFailure(target TargetCase, result CompileResult, err error) error {
	got := "missing"
	if result.Report != nil {
		got = result.Report.Pragma.Options["verdict"]
	}
	return StageError(
		3,
		target.Name,
		KindCompiler,
		"compile exit=%d verdict=got_%s want_%s stderr: %s: %v",
		result.ExitCode,
		got,
		target.ExpectedVerdict,
		TailLines(result.RawStderr, 20),
		err,
	)
}
