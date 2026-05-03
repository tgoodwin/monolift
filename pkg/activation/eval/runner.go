package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	"golang.org/x/tools/go/ssa"
)

// Options configures an evaluation run.
type Options struct {
	TracesDir      string
	ManifestPath   string
	EvaluationRoot string
	Projects       []string
	Timeout        time.Duration
	Deterministic  bool
	Augment        activation.AugmentMode
}

// Run evaluates the selected corpus and returns aggregate results.
func Run(ctx context.Context, opts Options) (EvaluationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}
	cases, err := EnumerateCases(opts.TracesDir, opts.ManifestPath, opts.EvaluationRoot)
	if err != nil {
		return EvaluationResult{}, err
	}
	cases = filterProjects(cases, opts.Projects)
	grouped := groupCases(cases)
	order := projectOrder(grouped, opts.Projects)

	var traces []TraceResult
	var feasibility []Feasibility
	for _, project := range order {
		projectCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
		projectTraces, projectFeasibility := runProject(projectCtx, grouped[project], opts)
		cancel()
		traces = append(traces, projectTraces...)
		feasibility = append(feasibility, projectFeasibility)
	}
	result := AggregateResults(traces)
	result.Feasibility = feasibility
	return result, nil
}

func runProject(ctx context.Context, cases []Case, opts Options) ([]TraceResult, Feasibility) {
	if len(cases) == 0 {
		return nil, Feasibility{}
	}
	target := cases[0].Target
	start := time.Now()
	feasibility := Feasibility{
		Project:        target.Name,
		PackagePattern: target.PackagePattern,
		WorkDir:        target.WorkDir,
		SHA:            target.SHA,
	}
	env, cleanup, err := evaluationEnv(target)
	if err != nil {
		feasibility.Error = err.Error()
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, activation.MissPackageLoadFailure, nil), feasibility
	}
	defer cleanup()
	cfg := activation.Config{
		Dir:      target.WorkDir,
		Packages: []string{target.PackagePattern},
		Timeout:  opts.Timeout,
		Context:  ctx,
		Env:      env,
		Augment:  opts.Augment,
	}
	program, err := cfg.LoadProgram()
	if err != nil {
		feasibility.Error = err.Error()
		feasibility.TimedOut = ctx.Err() != nil
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, categoryForProjectError(err, ctx), nil), feasibility
	}
	program.BuildSSA()
	if ctx.Err() != nil {
		feasibility.TimedOut = true
		feasibility.Error = ctx.Err().Error()
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, activation.MissTimeout, nil), feasibility
	}
	entrypoints, err := cfg.FindEntrypoints(program)
	if err != nil {
		feasibility.Error = err.Error()
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, activation.MissTargetUnreachable, nil), feasibility
	}
	graph, err := activation.BuildRTAGraph(program, entrypoints)
	if err != nil {
		feasibility.Error = err.Error()
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, activation.MissTargetUnreachable, nil), feasibility
	}
	if err := activation.Augment(graph, program, opts.Augment); err != nil {
		feasibility.Error = err.Error()
		finalizeFeasibility(&feasibility, start, opts)
		return failedProjectResults(cases, activation.MissTargetUnreachable, nil), feasibility
	}
	feasibility.Completed = true
	finalizeFeasibility(&feasibility, start, opts)

	results := make([]TraceResult, 0, len(cases))
	for _, c := range cases {
		results = append(results, runTrace(c, cfg, program, graph, entrypoints))
	}
	return results, feasibility
}

func runTrace(c Case, cfg activation.Config, program *activation.Program, graph *activation.Graph, entrypoints []*ssa.Function) TraceResult {
	expected, _ := TraceFunctionKeys(c.Trace, c.Target)
	expectedSteps, _ := TraceExpectedSteps(c.Trace, c.Target)
	traceResult := TraceResult{
		ID:           c.Trace.ID,
		Project:      c.Trace.Project,
		Target:       c.Trace.RegionRoot,
		ExpectedKeys: expected,
	}
	file, line, err := TargetLocation(c.Trace, c.Target)
	if err != nil {
		traceResult.Category = activation.MissTargetNotFound
		traceResult.FirstBlocker = FirstUnsupportedEdge(c.Trace)
		traceResult.Scores.Tier1Reachable, traceResult.Scores.Tier1Score = ScoreTier1(false)
		attachPartial(&traceResult, &activation.PartialPath{Gap: activation.Gap{
			AfterStep:    0,
			ExpectedEdge: "",
			Reason:       activation.GapTargetNotLoaded,
		}}, len(expectedSteps))
		return traceResult
	}
	targetFn, err := cfg.ResolveTarget(program, file, line)
	if err != nil {
		traceResult.Category, traceResult.FirstBlocker = ClassifyMiss(false, activation.MissTargetNotFound, c.Trace)
		traceResult.Scores.Tier1Reachable, traceResult.Scores.Tier1Score = ScoreTier1(false)
		partial := activation.FindPartialPath(graph, expectedSteps)
		if partial != nil {
			partial.Gap.Reason = activation.GapTargetNotLoaded
		}
		attachPartial(&traceResult, partial, len(expectedSteps))
		return traceResult
	}
	path, found := activation.ShortestPath(graph, entrypoints, targetFn)
	traceResult.Reachable = found
	traceResult.ActualKeys = pathKeys(path)
	traceResult.Scores.Tier1Reachable, traceResult.Scores.Tier1Score = ScoreTier1(found)
	traceResult.Scores.Tier2Exact, traceResult.Scores.Tier2Fuzzy = ScoreTier2(expected, traceResult.ActualKeys)
	traceResult.Scores.Tier3FileLine = ScoreTier3(c.Trace, c.Target, path)
	if found {
		traceResult.Category = activation.MissNone
	} else {
		traceResult.Category, traceResult.FirstBlocker = ClassifyMiss(false, activation.MissTargetUnreachable, c.Trace)
		attachPartial(&traceResult, activation.FindPartialPath(graph, expectedSteps), len(expectedSteps))
	}
	return traceResult
}

func attachPartial(result *TraceResult, partial *activation.PartialPath, total int) {
	if result == nil || partial == nil {
		return
	}
	if partial.Prefix != nil {
		result.PartialSteps = len(partial.Prefix.Steps)
	}
	result.TotalExpectedSteps = total
	result.GapReason = partial.Gap.Reason
}

// TargetLocation returns an absolute source file path and line for a trace's
// region root.
func TargetLocation(trace Trace, target Target) (string, int, error) {
	file, line, ok := splitTraceLocation(trace.RegionRoot)
	if !ok {
		return "", 0, fmt.Errorf("invalid region root %q", trace.RegionRoot)
	}
	file = filepath.ToSlash(file)
	projectPrefix := "evaluation/" + target.Name + "/"
	switch {
	case filepath.IsAbs(file):
		return filepath.Clean(file), line, nil
	case strings.HasPrefix(file, projectPrefix):
		return filepath.Join(target.ProjectDir, strings.TrimPrefix(file, projectPrefix)), line, nil
	default:
		return filepath.Join(target.ProjectDir, file), line, nil
	}
}

func pathKeys(path *activation.Path) []activation.FunctionKey {
	if path == nil {
		return nil
	}
	keys := make([]activation.FunctionKey, 0, len(path.Steps))
	for _, step := range path.Steps {
		if step.Node != nil {
			keys = append(keys, step.Node.Key)
		}
	}
	return keys
}

func failedProjectResults(cases []Case, category activation.MissCategory, blocker *BlockingEdge) []TraceResult {
	results := make([]TraceResult, 0, len(cases))
	for _, c := range cases {
		expected, _ := TraceFunctionKeys(c.Trace, c.Target)
		traceCategory, traceBlocker := ClassifyMiss(false, category, c.Trace)
		if blocker != nil {
			traceBlocker = blocker
		}
		reachable, score := ScoreTier1(false)
		results = append(results, TraceResult{
			ID:           c.Trace.ID,
			Project:      c.Trace.Project,
			Target:       c.Trace.RegionRoot,
			Reachable:    false,
			Category:     traceCategory,
			Scores:       Scores{Tier1Reachable: reachable, Tier1Score: score},
			FirstBlocker: traceBlocker,
			ExpectedKeys: expected,
		})
	}
	return results
}

func categoryForProjectError(err error, ctx context.Context) activation.MissCategory {
	if ctx.Err() != nil {
		return activation.MissTimeout
	}
	if err != nil {
		return activation.MissPackageLoadFailure
	}
	return activation.MissTargetUnreachable
}

func finalizeFeasibility(feasibility *Feasibility, start time.Time, opts Options) {
	if feasibility == nil {
		return
	}
	if opts.Deterministic {
		return
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	feasibility.WallTimeMS = time.Since(start).Milliseconds()
	feasibility.HeapAllocBytes = mem.HeapAlloc
}

func evaluationEnv(target Target) ([]string, func(), error) {
	if target.Name != "mattermost" {
		return nil, func() {}, nil
	}
	file, err := os.CreateTemp("", "monolift-mattermost-*.work")
	if err != nil {
		return nil, nil, err
	}
	serverDir, err := filepath.Abs(target.WorkDir)
	if err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, nil, err
	}
	publicDir, err := filepath.Abs(filepath.Join(target.WorkDir, "public"))
	if err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, nil, err
	}
	content := fmt.Sprintf("go 1.25.8\n\nuse (\n\t%s\n\t%s\n)\n",
		filepath.ToSlash(serverDir),
		filepath.ToSlash(publicDir),
	)
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return nil, nil, err
	}
	cleanup := func() { _ = os.Remove(file.Name()) }
	return []string{"GOWORK=" + file.Name()}, cleanup, nil
}

func filterProjects(cases []Case, projects []string) []Case {
	if len(projects) == 0 {
		return cases
	}
	allowed := map[string]bool{}
	for _, project := range projects {
		if project = strings.TrimSpace(project); project != "" {
			allowed[project] = true
		}
	}
	filtered := make([]Case, 0, len(cases))
	for _, c := range cases {
		if allowed[c.Trace.Project] {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func groupCases(cases []Case) map[string][]Case {
	grouped := map[string][]Case{}
	for _, c := range cases {
		grouped[c.Trace.Project] = append(grouped[c.Trace.Project], c)
	}
	for project := range grouped {
		sort.Slice(grouped[project], func(i, j int) bool {
			if grouped[project][i].Trace.CandidateNumber != grouped[project][j].Trace.CandidateNumber {
				return grouped[project][i].Trace.CandidateNumber < grouped[project][j].Trace.CandidateNumber
			}
			return grouped[project][i].Trace.ID < grouped[project][j].Trace.ID
		})
	}
	return grouped
}

func projectOrder(grouped map[string][]Case, requested []string) []string {
	if len(requested) > 0 {
		var order []string
		for _, project := range requested {
			project = strings.TrimSpace(project)
			if project != "" && len(grouped[project]) > 0 {
				order = append(order, project)
			}
		}
		return order
	}
	preferred := []string{"miniflux", "listmonk", "pocketbase", "caddy", "gitea", "mattermost"}
	seen := map[string]bool{}
	var order []string
	for _, project := range preferred {
		if len(grouped[project]) > 0 {
			order = append(order, project)
			seen[project] = true
		}
	}
	var rest []string
	for project := range grouped {
		if !seen[project] {
			rest = append(rest, project)
		}
	}
	sort.Strings(rest)
	return append(order, rest...)
}

func ParseProjectList(raw string) []string {
	var projects []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			projects = append(projects, part)
		}
	}
	return projects
}

func WriteJSON(path string, result EvaluationResult) error {
	data, err := jsonMarshalIndent(result)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func jsonMarshalIndent(result EvaluationResult) ([]byte, error) {
	return json.MarshalIndent(result, "", "  ")
}

func WriteMarkdown(path string, result EvaluationResult) error {
	var b strings.Builder
	fmt.Fprintln(&b, "# SPRINT-0035 RTA Baseline")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Traces: %d\n", result.Corpus.Traces)
	fmt.Fprintf(&b, "- Reachable: %d (%.1f%%)\n", result.Corpus.Reachable, 100*result.Corpus.ReachabilityRate)
	fmt.Fprintf(&b, "- Mean Tier 2 exact: %.3f\n", result.Corpus.MeanTier2Exact)
	fmt.Fprintf(&b, "- Mean Tier 2 fuzzy: %.3f\n", result.Corpus.MeanTier2Fuzzy)
	fmt.Fprintf(&b, "- Mean Tier 3 file:line: %.3f\n", result.Corpus.MeanTier3)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Per Project")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Project | Traces | Reachable | Reachability | Mean T2 Exact | Mean T2 Fuzzy | Mean T3 |")
	fmt.Fprintln(&b, "|---|---:|---:|---:|---:|---:|---:|")
	for _, project := range result.Projects {
		fmt.Fprintf(&b, "| %s | %d | %d | %.1f%% | %.3f | %.3f | %.3f |\n",
			project.Project, project.Traces, project.Reachable, 100*project.ReachabilityRate,
			project.MeanTier2Exact, project.MeanTier2Fuzzy, project.MeanTier3)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Feasibility")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Project | Pattern | Completed | Timed out | Wall ms | Heap bytes | Error |")
	fmt.Fprintln(&b, "|---|---|---:|---:|---:|---:|---|")
	for _, f := range result.Feasibility {
		fmt.Fprintf(&b, "| %s | `%s` | %t | %t | %d | %d | %s |\n",
			f.Project, f.PackagePattern, f.Completed, f.TimedOut, f.WallTimeMS, f.HeapAllocBytes, escapeTable(f.Error))
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Unsupported First Blockers")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Edge kind | Count |")
	fmt.Fprintln(&b, "|---|---:|")
	for _, item := range result.UnsupportedBreakdown {
		fmt.Fprintf(&b, "| %s | %d |\n", item.Kind, item.Count)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Gap Analysis")
	fmt.Fprintln(&b)
	if len(result.UnsupportedBreakdown) == 0 {
		fmt.Fprintln(&b, "No unsupported first blockers were recorded.")
	} else {
		for _, item := range result.UnsupportedBreakdown {
			fmt.Fprintf(&b, "- `%s` blocks %d trace(s). Follow-up augmentation should start with the concrete patterns listed in the miss details below.\n", item.Kind, item.Count)
		}
	}
	for _, project := range result.Projects {
		for _, category := range project.Categories {
			if category.Category == activation.MissTargetNotFound {
				fmt.Fprintf(&b, "- `%s` has %d target-not-found trace(s); verify package patterns, build tags, and nested modules before treating those as graph-edge misses.\n", project.Project, category.Count)
			}
			if category.Category == activation.MissPackageLoadFailure {
				fmt.Fprintf(&b, "- `%s` has %d package-load failure(s); resolve loader/toolchain issues before comparing graph quality.\n", project.Project, category.Count)
			}
			if category.Category == activation.MissTimeout {
				fmt.Fprintf(&b, "- `%s` has %d timeout(s); record the phase and memory profile before increasing algorithm complexity.\n", project.Project, category.Count)
			}
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Trace Miss Details")
	fmt.Fprintln(&b)
	for _, trace := range result.Traces {
		if trace.Reachable {
			continue
		}
		fmt.Fprintf(&b, "### %s\n\n", trace.ID)
		fmt.Fprintf(&b, "- Category: `%s`\n", trace.Category)
		if trace.GapReason != "" {
			fmt.Fprintf(&b, "- Partial path: resolved %d/%d steps, gap: `%s`\n", trace.PartialSteps, trace.TotalExpectedSteps, trace.GapReason)
		}
		if trace.FirstBlocker != nil {
			fmt.Fprintf(&b, "- First blocker: step %d `%s` (%s)\n", trace.FirstBlocker.Step, trace.FirstBlocker.RawType, trace.FirstBlocker.Kind)
			fmt.Fprintf(&b, "- Pattern: from `%s` to `%s`, func `%s`\n", trace.FirstBlocker.From, trace.FirstBlocker.To, trace.FirstBlocker.Func)
			if trace.FirstBlocker.FromRaw != "" || trace.FirstBlocker.ToRaw != "" {
				fmt.Fprintf(&b, "- Raw: %s -> %s\n", trace.FirstBlocker.FromRaw, trace.FirstBlocker.ToRaw)
			}
		}
		fmt.Fprintln(&b)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func escapeTable(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
