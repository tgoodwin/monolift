package extract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/callgraph/cha"

	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

// MONOLIFT_CORPUS_TESTS=1 opts into corpus-scale tests that load evaluation
// targets. Keep them out of the default unit lane to avoid whole-corpus SSA
// builds during normal package runs.

func TestLoadModuleScopesToAnnotatedRootModule(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "ssaspike", "sample.go")
	loaded, err := loadModule(Request{
		Sources: []string{filepath.Join("..", "testdata", "ssaspike")},
		Pragmas: []Pragma{{
			Name:     "ssaspike",
			Surface:  Surface("function"),
			DeclName: "Use",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     1,
				EndLine:  1,
			},
		}},
	})
	if err != nil {
		t.Fatalf("loadModule: %v", err)
	}

	if got := filepath.Base(loaded.ModuleRoot); got != "ssaspike" {
		t.Fatalf("module root = %q, want ssaspike", loaded.ModuleRoot)
	}
	if loaded.RootPkg == nil {
		t.Fatal("root package is nil")
	}
	if !strings.HasPrefix(loaded.RootPkg.PkgPath, "example.com/ssaspike") {
		t.Fatalf("root package path = %q, want example.com/ssaspike...", loaded.RootPkg.PkgPath)
	}
	if len(loaded.Packages) == 0 {
		t.Fatal("loaded package graph is empty")
	}
	if len(loaded.RootPkg.CompiledGoFiles) < 2 {
		t.Fatalf("root package compiled files = %v, want multi-file package", loaded.RootPkg.CompiledGoFiles)
	}
}

func TestLoadModulePropagatesEnvAndBuildTags(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")
	t.Setenv("GOFLAGS", "-tags=monoliftspike")

	rootFile := filepath.Join("..", "testdata", "ssaspike", "sample.go")
	loaded, err := loadModule(Request{
		Sources: []string{filepath.Join("..", "testdata", "ssaspike")},
		Pragmas: []Pragma{{
			Name:     "ssaspike",
			Surface:  Surface("function"),
			DeclName: "Use",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     1,
				EndLine:  1,
			},
		}},
	})
	if err != nil {
		t.Fatalf("loadModule: %v", err)
	}

	if !loaded.CGOEnabled {
		t.Fatal("CGO_ENABLED did not propagate into loader state")
	}
	if loaded.GOOS != "linux" || loaded.GOARCH != "amd64" {
		t.Fatalf("loader platform = %s/%s, want linux/amd64", loaded.GOOS, loaded.GOARCH)
	}
	if !slices.Equal(loaded.BuildTags, []string{"monoliftspike"}) {
		t.Fatalf("build tags = %v, want [monoliftspike]", loaded.BuildTags)
	}
	if !compiledFilesContain(loaded.RootPkg.CompiledGoFiles, "tagged_on.go") {
		t.Fatalf("compiled files missing tagged_on.go: %v", loaded.RootPkg.CompiledGoFiles)
	}
	if !compiledFilesContain(loaded.RootPkg.CompiledGoFiles, "cgo_on.go") {
		t.Fatalf("compiled files missing cgo_on.go: %v", loaded.RootPkg.CompiledGoFiles)
	}
}

func TestLoaderToolchainUsesModuleWhenParentIsAuto(t *testing.T) {
	t.Setenv("GOTOOLCHAIN", "auto")
	if got := loaderToolchain("go1.26.0"); got != "go1.26.0" {
		t.Fatalf("loaderToolchain(auto, module)= %q want go1.26.0", got)
	}
	t.Setenv("GOTOOLCHAIN", "go1.25.4")
	if got := loaderToolchain("go1.26.0"); got != "go1.25.4" {
		t.Fatalf("loaderToolchain(explicit, module)= %q want go1.25.4", got)
	}
}

func compiledFilesContain(paths []string, base string) bool {
	for _, path := range paths {
		if filepath.Base(path) == base {
			return true
		}
	}
	return false
}

func TestBuildProgramProducesCallgraphReadySSA(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "ssaspike", "sample.go")
	loaded, err := loadModule(Request{
		Sources: []string{filepath.Join("..", "testdata", "ssaspike")},
		Pragmas: []Pragma{{
			Name:     "ssaspike",
			Surface:  Surface("function"),
			DeclName: "Use",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     1,
				EndLine:  1,
			},
		}},
	})
	if err != nil {
		t.Fatalf("loadModule: %v", err)
	}

	built, err := buildProgram(loaded)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	if built.Program == nil || built.RootPackage == nil {
		t.Fatal("SSA program or root package is nil")
	}
	if len(cha.CallGraph(built.Program).Nodes) == 0 {
		t.Fatal("CHA callgraph is empty")
	}
}

func TestLoadModuleRejectsMissingRootPragma(t *testing.T) {
	t.Parallel()

	_, err := loadModule(Request{Sources: []string{filepath.Join("..", "testdata", "ssaspike")}})
	if err == nil || !strings.Contains(err.Error(), "no parsed monolift root pragma") {
		t.Fatalf("loadModule error = %v, want missing root pragma error", err)
	}
}

func TestLoadModulePropagatesPackageErrors(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "loaderror", "broken.go")
	_, err := loadModule(Request{
		Sources: []string{filepath.Join("..", "testdata", "loaderror")},
		Pragmas: []Pragma{{
			Name:     "broken",
			Surface:  SurfaceFunction,
			DeclName: "Broken",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     1,
				EndLine:  1,
			},
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "packages.Load reported errors") {
		t.Fatalf("loadModule error = %v, want packages.Load reported errors", err)
	}
}

func TestAnalyzeBuildsDeterministicMetadataReport(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "ssaspike", "sample.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "ssaspike")},
		Pragmas: []Pragma{{
			Name:     "ssaspike",
			Surface:  Surface("function"),
			DeclName: "Use",
			DeclKind: "func",
			Options:  map[string]string{"mode": "remote"},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Report.Analysis.Algorithm != "ssa-cha+rta" {
		t.Fatalf("analysis.algorithm = %q, want ssa-cha+rta", result.Report.Analysis.Algorithm)
	}
	if !result.Report.Analysis.Deterministic {
		t.Fatal("analysis.deterministic = false, want true")
	}
	if !strings.HasSuffix(result.Report.BuildConfig.ModuleRoot, "ssaspike") {
		t.Fatalf("buildConfig.moduleRoot = %q, want suffix ssaspike", result.Report.BuildConfig.ModuleRoot)
	}

	data, err := json.Marshal(result.Report)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := reportv2.Validate(data); err != nil {
		t.Fatalf("reportv2.Validate: %v\n%s", err, data)
	}
}

func TestAnalyzeResolvesStructRootAndExposedMethods(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "rootresolve", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []Pragma{{
			Name:     "handler-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     3,
				EndLine:  3,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if result.Report.Root.Identity.ObjectName != "Handler" || result.Report.Root.Identity.Kind != "type" {
		t.Fatalf("root identity = %+v, want Handler/type", result.Report.Root.Identity)
	}
	if result.Report.Root.RegistryKey == nil || *result.Report.Root.RegistryKey != "http.handlers.reverse_proxy" {
		t.Fatalf("registryKey = %v, want http.handlers.reverse_proxy", result.Report.Root.RegistryKey)
	}
	if len(result.Report.Root.ExposedOperations) != 1 || result.Report.Root.ExposedOperations[0].ObjectName != "(*Handler).ServeHTTP" {
		t.Fatalf("exposed operations = %+v, want (*Handler).ServeHTTP", result.Report.Root.ExposedOperations)
	}
	if len(result.Report.Closure.WiringPaths) != 1 {
		t.Fatalf("wiringPaths = %+v, want single root->ServeHTTP path", result.Report.Closure.WiringPaths)
	}
	path := result.Report.Closure.WiringPaths[0]
	if path.Target.ObjectName != "(*Handler).ServeHTTP" {
		t.Fatalf("wiring path target = %+v, want (*Handler).ServeHTTP", path.Target)
	}
	if len(path.Steps) != 2 || path.Steps[0].Identity.ObjectName != "Handler" || path.Steps[1].Identity.ObjectName != "(*Handler).ServeHTTP" {
		t.Fatalf("wiring path steps = %+v, want Handler -> (*Handler).ServeHTTP", path.Steps)
	}
}

func TestAnalyzeRefusesReflectionDispatchWithoutRegistry(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "reflectiondispatch", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "reflectiondispatch")},
		Pragmas: []Pragma{{
			Name:     "reflection-root",
			Surface:  SurfaceFunction,
			DeclName: "Entry",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     5,
				EndLine:  5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := result.Report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}
	if result.Report.Pruning.Bounded {
		t.Fatal("pruning.bounded = true, want false after refusal")
	}
	gotCodes := uniqueDiagnosticCodes(t, result.Diagnostics)
	wantCodes := []string{"MLV2_NO_ERROR_CHANNEL", codeReflectionDispatch, "MLV2_SERIALIZATION_UNSUPPORTED", "MLV2_SHAPE_UNSUPPORTED"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic codes = %v, want %v", gotCodes, wantCodes)
	}
	if len(result.Report.Diagnostics) != 0 {
		t.Fatalf("report diagnostics = %+v, want none before caller translation", result.Report.Diagnostics)
	}
}

func TestAnalyzeRefusesUnsafeBoundary(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "unsafeedge", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "unsafeedge")},
		Pragmas: []Pragma{{
			Name:     "unsafe-root",
			Surface:  SurfaceFunction,
			DeclName: "Entry",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     5,
				EndLine:  5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := result.Report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}
	if result.Report.Pruning.Bounded {
		t.Fatal("pruning.bounded = true, want false after refusal")
	}
	gotCodes := uniqueDiagnosticCodes(t, result.Diagnostics)
	wantCodes := []string{codeClosureUnbounded, "MLV2_NO_ERROR_CHANNEL", "MLV2_SERIALIZATION_UNSUPPORTED"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic codes = %v, want %v", gotCodes, wantCodes)
	}
	if len(result.Report.Diagnostics) != 0 {
		t.Fatalf("report diagnostics = %+v, want none before caller translation", result.Report.Diagnostics)
	}
}

func TestAnalyzeRefusesDynamicPluginLoad(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "pluginedge", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "pluginedge")},
		Pragmas: []Pragma{{
			Name:     "plugin-root",
			Surface:  SurfaceFunction,
			DeclName: "Entry",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     5,
				EndLine:  5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := result.Report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}
	if result.Report.Pruning.Bounded {
		t.Fatal("pruning.bounded = true, want false after refusal")
	}

	gotCodes := uniqueDiagnosticCodes(t, result.Diagnostics)
	wantCodes := []string{"MLV2_CHANNEL_BOUNDARY", codeClosureUnbounded, codeDynamicPlugin, "MLV2_SERIALIZATION_UNSUPPORTED"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic codes = %v, want %v", gotCodes, wantCodes)
	}
}

func TestAnalyzeEmitsLiftabilityDiagnosticsOncePerIdentity(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "reflectiondispatch", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "reflectiondispatch")},
		Pragmas: []Pragma{{
			Name:     "reflection-root",
			Surface:  SurfaceFunction,
			DeclName: "Entry",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     5,
				EndLine:  5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	gotCounts := diagnosticIdentityCounts(t, result.Diagnostics)
	if len(gotCounts) != len(result.Diagnostics) {
		t.Fatalf("diagnostic identities = %v, diagnostics = %+v; want one emission per identity", gotCounts, result.Diagnostics)
	}
	if len(result.Diagnostics) != 5 {
		t.Fatalf("diagnostics = %+v, want 5 distinct findings after extract-seam dedup", result.Diagnostics)
	}
	for key, count := range gotCounts {
		if count != 1 {
			t.Fatalf("diagnostic %q count = %d, want 1 (all diagnostics: %+v)", key, count, result.Diagnostics)
		}
	}
}

func TestAnalyzeKeepsDistinctSameCodeDiagnosticsWhenSpansDiffer(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "multiunsafehelpers", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "multiunsafehelpers")},
		Pragmas: []Pragma{{
			Name:     "multiunsafehelpers-root",
			Surface:  SurfaceFunction,
			DeclName: "Entry",
			DeclKind: "func",
			Span: Span{
				Filename: rootFile,
				Line:     5,
				EndLine:  5,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	seenSpans := map[string]bool{}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != codeClosureUnbounded || diagnostic.Message != "unsafe.Pointer crosses the extraction boundary" {
			continue
		}
		seenSpans[diagnostic.Span.Filename+":"+strconv.Itoa(diagnostic.Span.Line)] = true
	}
	got := make([]string, 0, len(seenSpans))
	for span := range seenSpans {
		got = append(got, span)
	}
	slices.Sort(got)
	want := []string{
		"root.go:11",
		"root.go:15",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s detector spans = %v, want %v (all diagnostics: %+v)", codeClosureUnbounded, got, want, result.Diagnostics)
	}
}

func diagnosticCodes(t *testing.T, diagnostics []Diagnostic) []string {
	t.Helper()
	codes := make([]string, 0, len(diagnostics))
	for _, d := range diagnostics {
		if d.Severity != SeverityError {
			t.Fatalf("diagnostic severity = %q, want error (diagnostic %+v)", d.Severity, d)
		}
		codes = append(codes, d.Code)
	}
	return codes
}

func uniqueDiagnosticCodes(t *testing.T, diagnostics []Diagnostic) []string {
	t.Helper()
	raw := diagnosticCodes(t, diagnostics)
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, code := range raw {
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	return out
}

func diagnosticIdentityCounts(t *testing.T, diagnostics []Diagnostic) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity != SeverityError {
			t.Fatalf("diagnostic severity = %q, want error (diagnostic %+v)", diagnostic.Severity, diagnostic)
		}
		counts[diagnosticIdentityKey(diagnostic)]++
	}
	return counts
}

func diagnosticIdentityKey(d Diagnostic) string {
	return strings.Join([]string{
		d.Code,
		d.Span.Filename,
		strconv.Itoa(d.Span.Line),
		strconv.Itoa(d.Span.EndLine),
		d.Message,
	}, "|")
}

func TestFindModuleRootPrefersGoMod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subdir := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := findModuleRoot(file)
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	if root != dir {
		t.Fatalf("findModuleRoot = %q, want %q", root, dir)
	}
}

func TestFindModuleRootFallsBackToGoWork(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	subdir := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// No go.mod — only go.work at root
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte("go 1.24\n\nuse .\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := findModuleRoot(file)
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	if root != dir {
		t.Fatalf("findModuleRoot = %q, want %q", root, dir)
	}
}

func TestFindModuleRootGoModWinsOverGoWork(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	innerDir := filepath.Join(dir, "inner")
	subdir := filepath.Join(innerDir, "pkg")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	// go.work at top level
	if err := os.WriteFile(filepath.Join(dir, "go.work"), []byte("go 1.24\n\nuse ./inner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// go.mod at inner level
	if err := os.WriteFile(filepath.Join(innerDir, "go.mod"), []byte("module example/inner\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "main.go")
	if err := os.WriteFile(file, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	root, err := findModuleRoot(file)
	if err != nil {
		t.Fatalf("findModuleRoot: %v", err)
	}
	// Should find go.mod at inner level, not go.work at top level
	if root != innerDir {
		t.Fatalf("findModuleRoot = %q, want %q (go.mod should win over go.work)", root, innerDir)
	}
}

func TestAnalyzeDetectsPocketBaseRefusals(t *testing.T) {
	if os.Getenv("MONOLIFT_CORPUS_TESTS") != "1" {
		t.Skip("skipping corpus-scale test; set MONOLIFT_CORPUS_TESTS=1 to run")
	}
	t.Parallel()

	sources := []string{filepath.Join("..", "..", "..", "evaluation", "pocketbase")}
	rootFile := filepath.Join("..", "..", "..", "evaluation", "pocketbase", "core", "app.go")
	result, err := Analyze(Request{
		Sources: sources,
		Pragmas: []Pragma{{
			Name:     "pocketbase-app",
			Surface:  SurfaceInterface,
			DeclName: "App",
			DeclKind: "interface",
			Options:  map[string]string{"state": "external"},
			Span: Span{
				Filename: rootFile,
				Line:     29,
				EndLine:  29,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := result.Report.Pragma.Options["verdict"]; got != "refuse-blocking" {
		t.Fatalf("pragma verdict = %q, want refuse-blocking", got)
	}
	if result.Report.Pruning.Bounded {
		t.Fatal("pruning.bounded = true, want false")
	}
	gotCodes := []string{}
	for _, diagnostic := range result.Diagnostics {
		gotCodes = append(gotCodes, diagnostic.Code)
	}
	wantCodes := []string{"MLV2_CLOSURE_TOO_LARGE", "MLV2_EMBEDDED_DB_APP_ROOT"}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("diagnostic codes = %v, want %v", gotCodes, wantCodes)
	}
	gotRows := []string{}
	for _, item := range result.Report.State {
		gotRows = append(gotRows, item.Symbol.ObjectName+"="+item.Disposition)
	}
	wantRows := []string{
		"BaseApp.auxConcurrentDB=refused",
		"BaseApp.auxNonconcurrentDB=refused",
		"BaseApp.concurrentDB=refused",
		"BaseApp.nonconcurrentDB=refused",
	}
	if !reflect.DeepEqual(gotRows, wantRows) {
		t.Fatalf("state rows = %v, want %v", gotRows, wantRows)
	}
}

func TestAnalyzePreservesAcceptWithWarningsVerdict(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "transport", "testdata", "fixtures", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "transport", "testdata", "fixtures")},
		Pragmas: []Pragma{{
			Name:     "request-reply-warning",
			Surface:  SurfaceFunction,
			DeclName: "RequestReply",
			DeclKind: "func",
			Options: map[string]string{
				"verdict": "accept-with-warnings",
			},
			Span: Span{
				Filename: rootFile,
				Line:     22,
				EndLine:  22,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if got := result.Report.Pragma.Options["verdict"]; got != "accept-with-warnings" {
		t.Fatalf("pragma verdict = %q, want accept-with-warnings", got)
	}
}

func TestAnalyzeResolvesInterfaceExposedMethods(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "rootresolve", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []Pragma{{
			Name:     "app-root",
			Surface:  SurfaceInterface,
			DeclName: "App",
			DeclKind: "interface",
			Options: map[string]string{
				"methods": "Run",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  9,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(result.Report.Root.ExposedOperations) != 1 || result.Report.Root.ExposedOperations[0].ObjectName != "App.Run" {
		t.Fatalf("exposed operations = %+v, want App.Run", result.Report.Root.ExposedOperations)
	}
}

func TestAnalyzeEnumeratesFullExposedSurfaceWithoutMethodsFilter(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "rootresolve", "root.go")

	structResult, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []Pragma{{
			Name:     "handler-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Span: Span{
				Filename: rootFile,
				Line:     3,
				EndLine:  3,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze struct: %v", err)
	}
	if got, want := symbolObjectNames(structResult.Report.Root.ExposedOperations), []string{"(*Handler).Provision", "(*Handler).ServeHTTP"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("struct exposed operations = %v, want %v", got, want)
	}

	interfaceResult, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "rootresolve")},
		Pragmas: []Pragma{{
			Name:     "composite-root",
			Surface:  SurfaceInterface,
			DeclName: "Composite",
			DeclKind: "interface",
			Span: Span{
				Filename: rootFile,
				Line:     17,
				EndLine:  20,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze interface: %v", err)
	}
	if got, want := symbolObjectNames(interfaceResult.Report.Root.ExposedOperations), []string{"Composite.Ping", "Composite.Run"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("interface exposed operations = %v, want %v", got, want)
	}
}

func TestAnalyzeWalksClosureAcrossCallsClosuresAndGlobals(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"methods": "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	names := map[string]bool{}
	for _, symbol := range result.Report.Closure.IncludedSymbols {
		names[symbol.Identity.ObjectName] = true
	}
	required := []string{
		"Handler",
		"(*Handler).ServeHTTP",
		"makeGreeter",
		"greeterImpl.Greet",
		"helper",
		"PackageGlobal",
		"PackageConst",
	}
	for _, name := range required {
		if !names[name] {
			t.Fatalf("closure missing %s: %+v", name, result.Report.Closure.IncludedSymbols)
		}
	}
	hasClosure := false
	for name := range names {
		if strings.Contains(name, "ServeHTTP$") {
			hasClosure = true
			break
		}
	}
	if !hasClosure {
		t.Fatalf("closure missing captured anonymous function: %+v", result.Report.Closure.IncludedSymbols)
	}
}

func TestAnalyzeUsesCHADefaultForInterfaceDispatch(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"methods": "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	names := map[string]bool{}
	for _, symbol := range result.Report.Closure.IncludedSymbols {
		names[symbol.Identity.ObjectName] = true
	}
	if !names["greeterImpl.Greet"] || !names["otherGreeter.Greet"] {
		t.Fatalf("CHA default should include both interface implementers, got %+v", result.Report.Closure.IncludedSymbols)
	}
}

func TestAnalyzeRefinesRegistryDispatchWithRTA(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	names := map[string]bool{}
	for _, symbol := range result.Report.Closure.IncludedSymbols {
		names[symbol.Identity.ObjectName] = true
	}
	if !names["greeterImpl.Greet"] {
		t.Fatalf("RTA-refined closure missing greeterImpl.Greet: %+v", result.Report.Closure.IncludedSymbols)
	}
	if names["otherGreeter.Greet"] {
		t.Fatalf("RTA-refined closure should exclude unused otherGreeter.Greet: %+v", result.Report.Closure.IncludedSymbols)
	}
	if !slices.Equal(result.Report.Analysis.PrecisionTriggers, []string{
		"dispatch-growth:example.com/closurewalk:(*Handler).ServeHTTP:Greet",
		"registry-key:http.handlers.reverse_proxy",
		"rta-escape:(*Handler).ServeHTTP",
	}) {
		t.Fatalf("precision triggers = %v, want dispatch-growth + registry-key + rta-escape", result.Report.Analysis.PrecisionTriggers)
	}
}

func TestAnalyzeBuildsCallgraphAtMostOncePerProgramPerPass(t *testing.T) {
	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	resetCallgraphBuildCounters()

	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(result.Report.Closure.IncludedSymbols) == 0 {
		t.Fatal("closure includedSymbols is empty")
	}

	counts := snapshotCallgraphBuildCounters()
	if counts.cha != 1 || counts.rta != 1 {
		t.Fatalf("callgraph build counts = %+v, want exactly one CHA build and one RTA build", counts)
	}
}

func TestAnalyzeRecordsDispatchGrowthTriggerDeterministically(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"methods": "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     7,
				EndLine:  7,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if !slices.Equal(result.Report.Analysis.PrecisionTriggers, []string{
		"dispatch-growth:example.com/closurewalk:(*Handler).ServeHTTP:Greet",
	}) {
		t.Fatalf("precision triggers = %v, want stable dispatch-growth trigger", result.Report.Analysis.PrecisionTriggers)
	}
}

func TestAnalyzePrunesStdlibCallsIntoExternalDeps(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	result, err := Analyze(Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"methods": "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     9,
				EndLine:  9,
			},
		}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	paths := make([]string, 0, len(result.Report.ExternalDeps))
	for _, dep := range result.Report.ExternalDeps {
		paths = append(paths, dep.AccessPath)
	}
	if !slices.Contains(paths, "strings") {
		t.Fatalf("external deps = %v, want strings", paths)
	}
	for _, symbol := range result.Report.Closure.IncludedSymbols {
		if symbol.Identity.PackagePath == "strings" {
			t.Fatalf("stdlib symbol leaked into included closure: %+v", symbol)
		}
	}
}

func TestAnalyzeEmitsDeterministicOrderingAcrossRuns(t *testing.T) {
	t.Parallel()

	rootFile := filepath.Join("..", "testdata", "closurewalk", "root.go")
	request := Request{
		Sources: []string{filepath.Join("..", "testdata", "closurewalk")},
		Pragmas: []Pragma{{
			Name:     "closure-root",
			Surface:  SurfaceStruct,
			DeclName: "Handler",
			DeclKind: "struct",
			Options: map[string]string{
				"registry": "http.handlers.reverse_proxy",
				"methods":  "ServeHTTP",
			},
			Span: Span{
				Filename: rootFile,
				Line:     9,
				EndLine:  9,
			},
		}},
	}

	first, err := Analyze(request)
	if err != nil {
		t.Fatalf("Analyze(first): %v", err)
	}
	second, err := Analyze(request)
	if err != nil {
		t.Fatalf("Analyze(second): %v", err)
	}

	if !reflect.DeepEqual(first.Report.Closure.IncludedSymbols, second.Report.Closure.IncludedSymbols) {
		t.Fatalf("includedSymbols drifted between runs\nfirst=%+v\nsecond=%+v", first.Report.Closure.IncludedSymbols, second.Report.Closure.IncludedSymbols)
	}
	if !reflect.DeepEqual(first.Report.Closure.ExcludedSymbols, second.Report.Closure.ExcludedSymbols) {
		t.Fatalf("excludedSymbols drifted between runs\nfirst=%+v\nsecond=%+v", first.Report.Closure.ExcludedSymbols, second.Report.Closure.ExcludedSymbols)
	}
	if !reflect.DeepEqual(first.Report.Closure.WiringPaths, second.Report.Closure.WiringPaths) {
		t.Fatalf("wiringPaths drifted between runs\nfirst=%+v\nsecond=%+v", first.Report.Closure.WiringPaths, second.Report.Closure.WiringPaths)
	}
	if !reflect.DeepEqual(first.Report.Analysis.PrecisionTriggers, second.Report.Analysis.PrecisionTriggers) {
		t.Fatalf("precisionTriggers drifted between runs\nfirst=%+v\nsecond=%+v", first.Report.Analysis.PrecisionTriggers, second.Report.Analysis.PrecisionTriggers)
	}
	if !reflect.DeepEqual(first.Report.Diagnostics, second.Report.Diagnostics) {
		t.Fatalf("diagnostics drifted between runs\nfirst=%+v\nsecond=%+v", first.Report.Diagnostics, second.Report.Diagnostics)
	}
}

func symbolObjectNames(symbols []reportv2.SymbolIdentity) []string {
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		out = append(out, symbol.ObjectName)
	}
	return out
}
