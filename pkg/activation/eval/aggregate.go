package eval

import (
	"sort"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// EvaluationResult is the complete deterministic evaluator output.
type EvaluationResult struct {
	Corpus               CorpusSummary    `json:"corpus"`
	Projects             []ProjectSummary `json:"projects"`
	Feasibility          []Feasibility    `json:"feasibility,omitempty"`
	UnsupportedBreakdown []EdgeKindCount  `json:"unsupported_breakdown,omitempty"`
	Traces               []TraceResult    `json:"traces"`
}

// Feasibility records per-codebase runtime feasibility diagnostics.
type Feasibility struct {
	Project           string `json:"project"`
	PackagePattern    string `json:"package_pattern"`
	WorkDir           string `json:"work_dir"`
	SHA               string `json:"sha"`
	Completed         bool   `json:"completed"`
	TimedOut          bool   `json:"timed_out"`
	Error             string `json:"error,omitempty"`
	AugmentIterations int    `json:"augment_iterations,omitempty"`
	AugmentLimitHit   bool   `json:"augment_limit_hit,omitempty"`
	WallTimeMS        int64  `json:"wall_time_ms,omitempty"`
	HeapAllocBytes    uint64 `json:"heap_alloc_bytes,omitempty"`
}

// CorpusSummary aggregates all trace results.
type CorpusSummary struct {
	Traces           int     `json:"traces"`
	Reachable        int     `json:"reachable"`
	ReachabilityRate float64 `json:"reachability_rate"`
	MeanTier2Exact   float64 `json:"mean_tier2_exact"`
	MeanTier2Fuzzy   float64 `json:"mean_tier2_fuzzy"`
	MeanTier3        float64 `json:"mean_tier3"`
}

// ProjectSummary aggregates results for one codebase.
type ProjectSummary struct {
	Project          string          `json:"project"`
	Traces           int             `json:"traces"`
	Reachable        int             `json:"reachable"`
	ReachabilityRate float64         `json:"reachability_rate"`
	MeanTier2Exact   float64         `json:"mean_tier2_exact"`
	MeanTier2Fuzzy   float64         `json:"mean_tier2_fuzzy"`
	MeanTier3        float64         `json:"mean_tier3"`
	Categories       []CategoryCount `json:"categories,omitempty"`
}

// CategoryCount counts miss categories.
type CategoryCount struct {
	Category activation.MissCategory `json:"category"`
	Count    int                     `json:"count"`
}

// EdgeKindCount counts first blocking edge kinds.
type EdgeKindCount struct {
	Kind  activation.EdgeKind `json:"kind"`
	Count int                 `json:"count"`
}

// AggregateResults builds corpus-wide and per-project summaries.
func AggregateResults(traces []TraceResult) EvaluationResult {
	sorted := append([]TraceResult(nil), traces...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Project != sorted[j].Project {
			return sorted[i].Project < sorted[j].Project
		}
		return sorted[i].ID < sorted[j].ID
	})

	var result EvaluationResult
	result.Traces = sorted
	result.Corpus = summarize(sorted)

	byProject := map[string][]TraceResult{}
	unsupported := map[activation.EdgeKind]int{}
	for _, trace := range sorted {
		byProject[trace.Project] = append(byProject[trace.Project], trace)
		if trace.FirstBlocker != nil {
			unsupported[trace.FirstBlocker.Kind]++
		}
	}
	projects := make([]string, 0, len(byProject))
	for project := range byProject {
		projects = append(projects, project)
	}
	sort.Strings(projects)
	for _, project := range projects {
		summary := ProjectSummary{Project: project}
		copySummary(&summary, summarize(byProject[project]))
		summary.Categories = categoryCounts(byProject[project])
		result.Projects = append(result.Projects, summary)
	}

	kinds := make([]activation.EdgeKind, 0, len(unsupported))
	for kind := range unsupported {
		kinds = append(kinds, kind)
	}
	sort.Slice(kinds, func(i, j int) bool {
		if unsupported[kinds[i]] != unsupported[kinds[j]] {
			return unsupported[kinds[i]] > unsupported[kinds[j]]
		}
		return kinds[i] < kinds[j]
	})
	for _, kind := range kinds {
		result.UnsupportedBreakdown = append(result.UnsupportedBreakdown, EdgeKindCount{Kind: kind, Count: unsupported[kind]})
	}
	return result
}

func summarize(traces []TraceResult) CorpusSummary {
	var summary CorpusSummary
	summary.Traces = len(traces)
	for _, trace := range traces {
		if trace.Reachable {
			summary.Reachable++
		}
		summary.MeanTier2Exact += trace.Scores.Tier2Exact
		summary.MeanTier2Fuzzy += trace.Scores.Tier2Fuzzy
		summary.MeanTier3 += trace.Scores.Tier3FileLine
	}
	if summary.Traces > 0 {
		summary.ReachabilityRate = float64(summary.Reachable) / float64(summary.Traces)
		summary.MeanTier2Exact /= float64(summary.Traces)
		summary.MeanTier2Fuzzy /= float64(summary.Traces)
		summary.MeanTier3 /= float64(summary.Traces)
	}
	return summary
}

func copySummary(dst *ProjectSummary, src CorpusSummary) {
	dst.Traces = src.Traces
	dst.Reachable = src.Reachable
	dst.ReachabilityRate = src.ReachabilityRate
	dst.MeanTier2Exact = src.MeanTier2Exact
	dst.MeanTier2Fuzzy = src.MeanTier2Fuzzy
	dst.MeanTier3 = src.MeanTier3
}

func categoryCounts(traces []TraceResult) []CategoryCount {
	counts := map[activation.MissCategory]int{}
	for _, trace := range traces {
		counts[trace.Category]++
	}
	categories := make([]activation.MissCategory, 0, len(counts))
	for category := range counts {
		categories = append(categories, category)
	}
	sort.Slice(categories, func(i, j int) bool {
		if counts[categories[i]] != counts[categories[j]] {
			return counts[categories[i]] > counts[categories[j]]
		}
		return categories[i] < categories[j]
	})
	out := make([]CategoryCount, 0, len(categories))
	for _, category := range categories {
		out = append(out, CategoryCount{Category: category, Count: counts[category]})
	}
	return out
}
