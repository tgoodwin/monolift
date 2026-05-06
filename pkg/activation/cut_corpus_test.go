//go:build corpus

package activation_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	activationeval "github.com/tgoodwin/monolift/pkg/activation/eval"
	"golang.org/x/tools/go/ssa"
)

type expectedCut struct {
	TraceID      string
	Step         int
	Function     string
	BoundaryData activation.BoundaryDataClass
	State        activation.StateClass
	Callbacks    activation.CallbackClass
	Feasibility  activation.CutFeasibility
	Gap          bool
}

type corpusCutSummary struct {
	Total                  int
	Exact                  int
	AcceptableDivergences  int
	Disagreements          int
	Skipped                int
	InfeasibleForFeasible  int
	StepDeltas             []int
	ProjectDisagreements   []string
	ProjectAcceptableNotes []string
}

func TestCutPlacementCorpus(t *testing.T) {
	if testing.Short() {
		t.Skip("cut-placement corpus evaluation is skipped in short mode")
	}

	root := repoRoot(t)
	expected := loadRecommendedCuts(t, filepath.Join(root, "docs/research/activation-paths/analyses/recommended-cuts.md"))
	cases, err := activationeval.EnumerateCases(
		filepath.Join(root, "docs/research/activation-paths/traces"),
		filepath.Join(root, "evaluation/MANIFEST.yaml"),
		filepath.Join(root, "evaluation"),
	)
	if err != nil {
		t.Fatal(err)
	}
	byProject := groupCutCases(cases)

	total := corpusCutSummary{}
	for _, project := range []string{"caddy", "miniflux", "listmonk", "pocketbase", "gitea", "mattermost"} {
		projectCases := byProject[project]
		if len(projectCases) == 0 {
			continue
		}
		t.Run(project, func(t *testing.T) {
			projectSummary := runCutProject(t, projectCases, expected)
			total.Total += projectSummary.Total
			total.Exact += projectSummary.Exact
			total.AcceptableDivergences += projectSummary.AcceptableDivergences
			total.Disagreements += projectSummary.Disagreements
			total.Skipped += projectSummary.Skipped
			total.InfeasibleForFeasible += projectSummary.InfeasibleForFeasible
			total.ProjectDisagreements = append(total.ProjectDisagreements, projectSummary.ProjectDisagreements...)
			total.ProjectAcceptableNotes = append(total.ProjectAcceptableNotes, projectSummary.ProjectAcceptableNotes...)
			total.StepDeltas = append(total.StepDeltas, projectSummary.StepDeltas...)
			t.Logf("cut corpus %s: total=%d exact=%d acceptable=%d disagreements=%d skipped=%d deltas=%v",
				project, projectSummary.Total, projectSummary.Exact, projectSummary.AcceptableDivergences, projectSummary.Disagreements, projectSummary.Skipped, projectSummary.StepDeltas)
			for _, note := range projectSummary.ProjectAcceptableNotes {
				t.Log(note)
			}
			for _, note := range projectSummary.ProjectDisagreements {
				t.Error(note)
			}
		})
	}

	t.Logf("cut corpus total: total=%d exact=%d acceptable=%d disagreements=%d skipped=%d",
		total.Total, total.Exact, total.AcceptableDivergences, total.Disagreements, total.Skipped)
	t.Logf("step deltas: %v", total.StepDeltas)
	if len(total.StepDeltas) > 0 {
		sumAbs, maxAbs := 0, 0
		for _, d := range total.StepDeltas {
			ad := d
			if ad < 0 {
				ad = -ad
			}
			sumAbs += ad
			if ad > maxAbs {
				maxAbs = ad
			}
		}
		t.Logf("step distance: mean=%.1f max=%d", float64(sumAbs)/float64(len(total.StepDeltas)), maxAbs)
	}
	if total.Exact < 60 {
		t.Errorf("exact matches = %d, want >= 60", total.Exact)
	}
	if total.InfeasibleForFeasible != 0 {
		t.Errorf("infeasible recommendations for feasible ground truth = %d, want 0", total.InfeasibleForFeasible)
	}
}

func runCutProject(t *testing.T, cases []activationeval.Case, expected map[string]expectedCut) corpusCutSummary {
	t.Helper()
	target := cases[0].Target
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	env, cleanup := cutEvaluationEnv(t, target)
	defer cleanup()

	cfg := activation.Config{
		Dir:      target.WorkDir,
		Packages: []string{target.PackagePattern},
		Context:  ctx,
		Timeout:  10 * time.Minute,
		Env:      env,
		Augment:  activation.ModeAll,
	}
	program, err := cfg.LoadProgram()
	if err != nil {
		t.Fatal(err)
	}
	program.BuildSSA()
	entrypoints, err := cfg.FindEntrypoints(program)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := activation.BuildRTAGraph(program, entrypoints)
	if err != nil {
		t.Fatal(err)
	}
	if err := activation.Augment(graph, program, activation.ModeAll); err != nil {
		t.Fatal(err)
	}

	summary := corpusCutSummary{}
	for _, c := range cases {
		want, ok := expected[c.Trace.ID]
		if !ok {
			t.Fatalf("missing ground truth for %s", c.Trace.ID)
		}
		if c.Trace.ID == "mattermost/M-4" || want.Gap {
			summary.Skipped++
			continue
		}
		summary.Total++
		got, err := analyzeCorpusTraceCut(c, cfg, program, graph, entrypoints)
		if err != nil {
			summary.Disagreements++
			summary.ProjectDisagreements = append(summary.ProjectDisagreements, fmt.Sprintf("%s: %v", c.Trace.ID, err))
			continue
		}
		classification := classifyCutMatch(got, want)
		if got != nil && got.Recommended != nil {
			summary.StepDeltas = append(summary.StepDeltas, got.Recommended.Step-want.Step)
		} else {
			summary.StepDeltas = append(summary.StepDeltas, -want.Step)
		}
		switch classification {
		case "exact":
			summary.Exact++
		case "acceptable":
			summary.AcceptableDivergences++
			summary.ProjectAcceptableNotes = append(summary.ProjectAcceptableNotes, cutMismatchNote(c.Trace.ID, got, want, "acceptable"))
		default:
			summary.Disagreements++
			if got == nil || got.Recommended == nil || got.Recommended.Feasibility == activation.Infeasible {
				if want.Feasibility == activation.Feasible || want.Feasibility == activation.FeasibleWithProxy {
					summary.InfeasibleForFeasible++
				}
			}
			summary.ProjectDisagreements = append(summary.ProjectDisagreements, cutMismatchNote(c.Trace.ID, got, want, "disagreement"))
		}
	}
	return summary
}

func analyzeCorpusTraceCut(c activationeval.Case, cfg activation.Config, program *activation.Program, graph *activation.Graph, entrypoints []*ssa.Function) (*activation.CutResult, error) {
	file, line, err := activationeval.TargetLocation(c.Trace, c.Target)
	if err != nil {
		return nil, err
	}
	targetFn, err := cfg.ResolveTarget(program, file, line)
	if err != nil {
		return nil, err
	}
	path, found := activation.ShortestPath(graph, entrypoints, targetFn)
	if !found {
		return nil, fmt.Errorf("activation path not found")
	}
	return activation.AnalyzeCut(&activation.Result{Path: path}, graph)
}

func classifyCutMatch(got *activation.CutResult, want expectedCut) string {
	if got == nil || got.Recommended == nil {
		if want.Feasibility == activation.Infeasible {
			return "acceptable"
		}
		return "disagreement"
	}
	if got.Recommended.Step == want.Step {
		return "exact"
	}
	if feasibilityRank(got.Recommended.Feasibility) <= feasibilityRank(want.Feasibility) {
		return "acceptable"
	}
	return "disagreement"
}

func cutMismatchNote(traceID string, got *activation.CutResult, want expectedCut, kind string) string {
	if got == nil || got.Recommended == nil {
		return fmt.Sprintf("%s: %s, expected step %d %s/%s, got no recommendation (delta=-%d)", traceID, kind, want.Step, want.Function, want.Feasibility, want.Step)
	}
	delta := got.Recommended.Step - want.Step
	return fmt.Sprintf("%s: %s (delta=%+d), expected step %d %s/%s, got step %d %s/%s (%s)",
		traceID, kind, delta, want.Step, want.Function, want.Feasibility,
		got.Recommended.Step, got.Recommended.NodeName, got.Recommended.Feasibility, got.Recommended.Reason)
}

func feasibilityRank(class activation.CutFeasibility) int {
	switch class {
	case activation.Feasible:
		return 0
	case activation.FeasibleWithProxy:
		return 1
	case activation.Infeasible:
		return 2
	default:
		return 3
	}
}

func loadRecommendedCuts(t *testing.T, path string) map[string]expectedCut {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]expectedCut{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := splitMarkdownRow(line)
		if len(cells) != 9 {
			t.Fatalf("unexpected recommended-cuts row with %d cells: %s", len(cells), line)
		}
		traceID := trimMarkdownCell(cells[0])
		stepText := trimMarkdownCell(cells[2])
		cut := expectedCut{
			TraceID:      traceID,
			Function:     trimMarkdownCell(cells[3]),
			BoundaryData: parseBoundaryDataClass(t, cells[5]),
			State:        parseStateClass(t, cells[6]),
			Callbacks:    parseCallbackClass(t, cells[7]),
			Feasibility:  parseFeasibilityClass(t, cells[8]),
		}
		if stepText == "gap" {
			cut.Gap = true
		} else {
			step, err := strconv.Atoi(stepText)
			if err != nil {
				t.Fatalf("parse step %q in %s: %v", stepText, traceID, err)
			}
			cut.Step = step
		}
		result[traceID] = cut
	}
	return result
}

func splitMarkdownRow(line string) []string {
	raw := strings.Split(strings.Trim(line, "|"), "|")
	cells := make([]string, 0, len(raw))
	for _, cell := range raw {
		cells = append(cells, strings.TrimSpace(cell))
	}
	return cells
}

func trimMarkdownCell(cell string) string {
	cell = strings.TrimSpace(cell)
	cell = strings.Trim(cell, "`")
	cell = strings.ReplaceAll(cell, "`", "")
	return strings.TrimSpace(cell)
}

func parseBoundaryDataClass(t *testing.T, cell string) activation.BoundaryDataClass {
	t.Helper()
	switch strings.ToLower(trimMarkdownCell(cell)) {
	case "trivial":
		return activation.Trivial
	case "serializable":
		return activation.Serializable
	case "reconstructible":
		return activation.Reconstructible
	case "proxy-required":
		return activation.ProxyRequired
	case "-", "infeasible":
		return activation.BoundaryInfeasible
	default:
		t.Fatalf("unknown boundary-data class %q", cell)
		return ""
	}
}

func parseStateClass(t *testing.T, cell string) activation.StateClass {
	t.Helper()
	switch strings.ToLower(trimMarkdownCell(cell)) {
	case "stateless":
		return activation.Stateless
	case "config-only":
		return activation.ConfigOnly
	case "client-reconstructible":
		return activation.ClientReconstructible
	case "shared-state":
		return activation.SharedState
	case "-":
		return activation.SharedState
	default:
		t.Fatalf("unknown state class %q", cell)
		return ""
	}
}

func parseCallbackClass(t *testing.T, cell string) activation.CallbackClass {
	t.Helper()
	switch strings.ToLower(trimMarkdownCell(cell)) {
	case "0 (confirmed)":
		return activation.ZeroConfirmed
	case "0 (estimated)":
		return activation.ZeroEstimated
	case "low":
		return activation.Low
	case "moderate":
		return activation.Moderate
	case "many":
		return activation.Many
	case "-":
		return activation.Many
	default:
		t.Fatalf("unknown callback class %q", cell)
		return ""
	}
}

func parseFeasibilityClass(t *testing.T, cell string) activation.CutFeasibility {
	t.Helper()
	switch strings.ToLower(trimMarkdownCell(cell)) {
	case "feasible":
		return activation.Feasible
	case "feasible-with-proxy":
		return activation.Feasible // ADR-0028: FeasibleWithProxy retired, treat as Feasible
	case "infeasible", "-":
		return activation.Infeasible
	default:
		t.Fatalf("unknown feasibility class %q", cell)
		return ""
	}
}

func groupCutCases(cases []activationeval.Case) map[string][]activationeval.Case {
	grouped := map[string][]activationeval.Case{}
	for _, c := range cases {
		grouped[c.Trace.Project] = append(grouped[c.Trace.Project], c)
	}
	for project := range grouped {
		sort.Slice(grouped[project], func(i, j int) bool {
			return grouped[project][i].Trace.CandidateNumber < grouped[project][j].Trace.CandidateNumber
		})
	}
	return grouped
}

func cutEvaluationEnv(t *testing.T, target activationeval.Target) ([]string, func()) {
	t.Helper()
	if target.Name != "mattermost" {
		return nil, func() {}
	}
	file, err := os.CreateTemp("", "monolift-cut-mattermost-*.work")
	if err != nil {
		t.Fatal(err)
	}
	serverDir, err := filepath.Abs(target.WorkDir)
	if err != nil {
		t.Fatal(err)
	}
	publicDir, err := filepath.Abs(filepath.Join(target.WorkDir, "public"))
	if err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("go 1.25.8\n\nuse (\n\t%s\n\t%s\n)\n",
		filepath.ToSlash(serverDir),
		filepath.ToSlash(publicDir),
	)
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return []string{"GOWORK=" + file.Name()}, func() { _ = os.Remove(file.Name()) }
}
