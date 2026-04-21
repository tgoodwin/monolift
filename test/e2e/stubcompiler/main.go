package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tgoodwin/monolift/pkg/compiler"
	compilerdiagnostics "github.com/tgoodwin/monolift/pkg/compiler/diagnostics"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

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
	var sources sourceFlags
	flag.Var(&sources, "source", "source directory")
	flag.Parse()

	if *target == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: stubcompiler --target=<name> --output=<dir> [--source=<dir>...]")
		os.Exit(2)
	}

	fixtureDir := filepath.Join("test", "e2e", "stubcompiler", "fixtures", *target)
	if _, err := os.Stat(fixtureDir); err != nil {
		fixtureDir = filepath.Join(repoRoot(), "test", "e2e", "stubcompiler", "fixtures", *target)
	}
	if !usesRealCompiler(*target) {
		if _, err := os.Stat(fixtureDir); err == nil {
			if err := copyTree(fixtureDir, *output); err != nil {
				fmt.Fprintf(os.Stderr, "stubcompiler target %s: %v\n", *target, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stdout, "stubcompiler emitted %s to %s\n", *target, *output)
			return
		}
	}

	if len(sources) == 0 {
		fmt.Fprintf(os.Stderr, "stubcompiler target %s: no fixture and no --source paths\n", *target)
		os.Exit(1)
	}
	if err := emitPragmaReport(*target, *output, resolveSources(sources)); err != nil {
		fmt.Fprintf(os.Stderr, "stubcompiler target %s: %v\n", *target, err)
		os.Exit(1)
	}
	if usesRealCompiler(*target) {
		if err := copyLiftedArtifacts(fixtureDir, *output); err != nil {
			fmt.Fprintf(os.Stderr, "stubcompiler target %s: %v\n", *target, err)
			os.Exit(1)
		}
	}
	fmt.Fprintf(os.Stdout, "stubcompiler parsed %s to %s\n", *target, *output)
}

func usesRealCompiler(target string) bool {
	switch target {
	case "caddy", "pocketbase", "shape-transport-handler-mismatch", "state-decl-conflict-stateless-global-store":
		return true
	default:
		return false
	}
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

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		return copyFile(path, out)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func copyLiftedArtifacts(fixtureDir, output string) error {
	liftedDir := filepath.Join(fixtureDir, "lifted")
	if _, err := os.Stat(liftedDir); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyTree(liftedDir, filepath.Join(output, "lifted"))
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
	if usesRealCompiler(target) {
		pragmas, _, err := compiler.Parse(sources)
		if err != nil {
			return err
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

	pragmas, diagnostics, err := compiler.Parse(sources)
	if err != nil {
		return err
	}
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
		panic(err)
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
