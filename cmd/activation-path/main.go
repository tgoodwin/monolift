package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	activationeval "github.com/tgoodwin/monolift/pkg/activation/eval"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var (
		packagesFlag  string
		target        string
		format        string
		verbose       bool
		timeout       time.Duration
		evalMode      bool
		evalTraces    string
		evalManifest  string
		evalRoot      string
		evalProjects  string
		evalJSON      string
		evalMD        string
		deterministic     bool
		augmentations     string
		reverseImportScope bool
	)
	flags := flag.NewFlagSet("activation-path", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&packagesFlag, "packages", ".", "comma-separated Go package patterns to load")
	flags.StringVar(&target, "target", "", "target function source location as file:line")
	flags.StringVar(&format, "format", "text", "output format: text or json")
	flags.BoolVar(&verbose, "verbose", false, "include diagnostics and phase timings in text output")
	flags.DurationVar(&timeout, "timeout", 120*time.Second, "per-target analysis timeout")
	flags.BoolVar(&evalMode, "eval", false, "run the activation-path evaluation harness")
	flags.StringVar(&evalTraces, "eval-traces", "docs/research/activation-paths/traces", "directory of structured JSON traces")
	flags.StringVar(&evalManifest, "eval-manifest", "evaluation/MANIFEST.yaml", "evaluation manifest path")
	flags.StringVar(&evalRoot, "eval-root", "evaluation", "evaluation targets root directory")
	flags.StringVar(&evalProjects, "eval-projects", "", "comma-separated projects to evaluate")
	flags.StringVar(&evalJSON, "eval-json", "", "write evaluation JSON to this path")
	flags.StringVar(&evalMD, "eval-md", "", "write evaluation Markdown report to this path")
	flags.BoolVar(&deterministic, "deterministic", false, "redact nondeterministic feasibility timing/memory fields")
	flags.StringVar(&augmentations, "augmentations", string(activation.ModeAll), "augmentation mode: rta, structfield, predicates, goroutine, all")
	flags.BoolVar(&reverseImportScope, "reverse-import-scope", false, "pre-filter packages to transitive importers of the target before type-checking")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	augmentMode, err := activation.ParseAugmentMode(augmentations)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if evalMode {
		return runEval(timeout, evalTraces, evalManifest, evalRoot, evalProjects, evalJSON, evalMD, deterministic, augmentMode)
	}
	patterns := splitPatterns(packagesFlag)
	if target == "" {
		fmt.Fprintln(os.Stderr, "--target is required")
		return 2
	}
	if format != "text" && format != "json" {
		fmt.Fprintf(os.Stderr, "unsupported --format %q\n", format)
		return 2
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	analyzer := activation.NewAnalyzer(activation.Config{
		Dir:           ".",
		Packages:      patterns,
		Target:        target,
		Format:        format,
		Verbose:       verbose,
		Timeout:       timeout,
		Augment:       augmentMode,
		ScopePackages: reverseImportScope,
	})
	result, err := analyzer.Analyze(ctx)
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if result == nil {
			result = &activation.Result{Category: activation.MissPackageLoadFailure}
		}
		if encodeErr := enc.Encode(result); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			return 1
		}
	} else {
		writeText(os.Stdout, result, verbose)
	}
	if err != nil {
		return 1
	}
	if result != nil && !result.Found {
		return 1
	}
	return 0
}

func runEval(timeout time.Duration, tracesDir, manifestPath, evaluationRoot, projects, jsonPath, mdPath string, deterministic bool, augmentMode activation.AugmentMode) int {
	projectList := activationeval.ParseProjectList(projects)
	projectCount := len(projectList)
	if projectCount == 0 {
		projectCount = 6
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout*time.Duration(projectCount))
	defer cancel()
	result, err := activationeval.Run(ctx, activationeval.Options{
		TracesDir:      tracesDir,
		ManifestPath:   manifestPath,
		EvaluationRoot: evaluationRoot,
		Projects:       projectList,
		Timeout:        timeout,
		Deterministic:  deterministic,
		Augment:        augmentMode,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if jsonPath != "" {
		if err := activationeval.WriteJSON(jsonPath, result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	} else {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	if mdPath != "" {
		if err := activationeval.WriteMarkdown(mdPath, result); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	}
	return 0
}

func splitPatterns(raw string) []string {
	var patterns []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			patterns = append(patterns, part)
		}
	}
	if len(patterns) == 0 {
		return []string{"."}
	}
	return patterns
}

func writeText(out io.Writer, result *activation.Result, verbose bool) {
	if result == nil {
		fmt.Fprintln(out, "miss: analysis failed before producing a result")
		return
	}
	if result.Found {
		fmt.Fprintf(out, "found: %d steps\n", len(result.Path.Steps))
		for i, step := range result.Path.Steps {
			if step.Edge != nil {
				fmt.Fprintf(out, "  --%s--> %s\n", step.Edge.Kind, formatNode(step.Node))
			} else {
				fmt.Fprintf(out, "[%d] %s\n", i, formatNode(step.Node))
			}
		}
	} else {
		fmt.Fprintf(out, "miss: %s\n", result.Category)
		if result.Target != nil {
			fmt.Fprintf(out, "target: %s\n", formatNode(result.Target))
		}
	}
	if verbose {
		for _, diag := range result.Diagnostics {
			fmt.Fprintf(out, "%s[%s]: %s\n", diag.Severity, diag.Phase, diag.Message)
		}
		for _, timing := range result.Timings {
			fmt.Fprintf(out, "time[%s]: %s\n", timing.Phase, timing.Duration)
		}
	}
}

func formatNode(node *activation.Node) string {
	if node == nil {
		return "<nil>"
	}
	if node.Position.File == "" {
		return node.Key.String()
	}
	return fmt.Sprintf("%s (%s:%d)", node.Key.String(), node.Position.File, node.Position.Line)
}
