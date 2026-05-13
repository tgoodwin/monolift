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
		packagesFlag       string
		target             string
		format             string
		verbose            bool
		profile            bool
		profileOutput      string
		timeout            time.Duration
		evalMode           bool
		evalTraces         string
		evalManifest       string
		evalRoot           string
		evalProjects       string
		evalJSON           string
		evalMD             string
		deterministic      bool
		augmentations      string
		reverseImportScope bool
		skipAugmentIfRTA   bool
	)
	flags := flag.NewFlagSet("activation-path", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&packagesFlag, "packages", ".", "comma-separated Go package patterns to load")
	flags.StringVar(&target, "target", "", "target function source location as file:line")
	flags.StringVar(&format, "format", "text", "output format: text or json")
	flags.BoolVar(&verbose, "verbose", false, "include diagnostics and phase timings in text output")
	flags.BoolVar(&profile, "profile", false, "emit structured JSON timing profile")
	flags.StringVar(&profileOutput, "profile-output", "", "write --profile JSON to this path instead of stdout")
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
	flags.BoolVar(&skipAugmentIfRTA, "skip-augment-if-rta-reachable", false, "skip augmentation when the target is already reachable in the RTA graph")
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
		Dir:                         ".",
		Packages:                    patterns,
		Target:                      target,
		Format:                      format,
		Verbose:                     verbose,
		Timeout:                     timeout,
		Augment:                     augmentMode,
		ScopePackages:               reverseImportScope,
		SkipAugmentWhenRTAReachable: skipAugmentIfRTA,
	})
	result, err := analyzer.Analyze(ctx)
	if profile {
		if writeErr := writeProfile(profileOutput, result); writeErr != nil {
			fmt.Fprintln(os.Stderr, writeErr)
			return 1
		}
	}
	if profile && profileOutput == "" {
		// Keep stdout machine-readable when --profile owns it.
	} else if format == "json" {
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

type profileReport struct {
	Found              bool                     `json:"found"`
	SkippedAugment     bool                     `json:"skipped_augment,omitempty"`
	Category           activation.MissCategory  `json:"category,omitempty"`
	Target             *activation.Node         `json:"target,omitempty"`
	PathLength         int                      `json:"path_length,omitempty"`
	RecommendedCutStep int                      `json:"recommended_cut_step,omitempty"`
	RecommendedCutKey  string                   `json:"recommended_cut_key,omitempty"`
	Diagnostics        []activation.Diagnostic  `json:"diagnostics,omitempty"`
	PhaseTimings       []activation.PhaseTiming `json:"phase_timings,omitempty"`
	AugmentSubTimings  []activation.PhaseTiming `json:"augment_sub_timings,omitempty"`
	Stats              activation.GraphStats    `json:"stats"`
}

func writeProfile(path string, result *activation.Result) error {
	report := buildProfileReport(result)
	var out io.Writer = os.Stdout
	var file *os.File
	var err error
	if path != "" {
		file, err = os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		out = file
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func buildProfileReport(result *activation.Result) profileReport {
	if result == nil {
		return profileReport{Category: activation.MissPackageLoadFailure}
	}
	report := profileReport{
		Found:             result.Found,
		SkippedAugment:    result.SkippedAugment,
		Category:          result.Category,
		Target:            result.Target,
		Diagnostics:       result.Diagnostics,
		PhaseTimings:      result.Timings,
		AugmentSubTimings: result.SubTimings,
		Stats:             result.Stats,
	}
	if result.Path != nil {
		report.PathLength = len(result.Path.Steps)
	}
	if cut, err := activation.AnalyzeCut(result, nil); err == nil && cut != nil && cut.Recommended != nil {
		report.RecommendedCutStep = cut.Recommended.Step
		report.RecommendedCutKey = cut.Recommended.NodeKey.String()
	}
	return report
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
