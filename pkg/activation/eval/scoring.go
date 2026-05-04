package eval

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// Scores contains the tiered evaluation metrics for one trace.
type Scores struct {
	Tier1Reachable bool    `json:"tier1_reachable"`
	Tier1Score     float64 `json:"tier1_score"`
	Tier2Exact     float64 `json:"tier2_exact"`
	Tier2Fuzzy     float64 `json:"tier2_fuzzy"`
	Tier3FileLine  float64 `json:"tier3_file_line"`
}

// TraceResult is the deterministic evaluation output for one ground-truth
// trace.
type TraceResult struct {
	ID                 string                   `json:"id"`
	Project            string                   `json:"project"`
	Target             string                   `json:"target"`
	Reachable          bool                     `json:"reachable"`
	Category           activation.MissCategory  `json:"category"`
	Scores             Scores                   `json:"scores"`
	FirstBlocker       *BlockingEdge            `json:"first_blocker,omitempty"`
	ExpectedKeys       []activation.FunctionKey `json:"expected_keys,omitempty"`
	ActualKeys         []activation.FunctionKey `json:"actual_keys,omitempty"`
	PartialSteps       int                      `json:"partial_steps,omitempty"`
	TotalExpectedSteps int                      `json:"total_expected_steps,omitempty"`
	GapReason          string                   `json:"gap_reason,omitempty"`
}

// BlockingEdge records the first expected edge the RTA baseline cannot
// represent for an unreachable trace.
type BlockingEdge struct {
	Step       int                 `json:"step"`
	Kind       activation.EdgeKind `json:"kind"`
	RawType    string              `json:"raw_type"`
	Func       string              `json:"func,omitempty"`
	From       string              `json:"from,omitempty"`
	To         string              `json:"to,omitempty"`
	FromRaw    string              `json:"from_raw,omitempty"`
	ToRaw      string              `json:"to_raw,omitempty"`
	Diagnostic string              `json:"diagnostic,omitempty"`
}

// ScoreTier1 returns the binary reachability score.
func ScoreTier1(reachable bool) (bool, float64) {
	if reachable {
		return true, 1
	}
	return false, 0
}

// FirstUnsupportedEdge identifies the first expected edge that cannot be
// represented by the RTA baseline edge taxonomy.
func FirstUnsupportedEdge(trace Trace) *BlockingEdge {
	for _, step := range trace.Steps {
		if step.Step == 0 || step.EdgeType == "entrypoint" {
			continue
		}
		mapping := step.CanonicalEdgeKind()
		if rtaRepresents(mapping.Kind) {
			continue
		}
		blocker := &BlockingEdge{
			Step:       step.Step,
			Kind:       mapping.Kind,
			RawType:    step.EdgeType,
			FromRaw:    step.FromRaw,
			ToRaw:      step.ToRaw,
			Diagnostic: mapping.Diagnostic,
		}
		if step.Func != nil {
			blocker.Func = *step.Func
		}
		if step.From != nil {
			blocker.From = *step.From
		}
		blocker.To = step.To
		return blocker
	}
	return nil
}

// ClassifyMiss returns the evaluator miss category and first blocker for one
// trace result.
func ClassifyMiss(reachable bool, analyzerCategory activation.MissCategory, trace Trace) (activation.MissCategory, *BlockingEdge) {
	if reachable {
		return activation.MissNone, nil
	}
	blocker := FirstUnsupportedEdge(trace)
	if analyzerCategory == activation.MissTargetNotFound {
		return analyzerCategory, blocker
	}
	switch analyzerCategory {
	case activation.MissPackageLoadFailure, activation.MissTimeout, activation.MissTargetNotFound:
		return analyzerCategory, nil
	}
	if blocker != nil {
		return activation.MissUnsupportedEdgeKind, blocker
	}
	if analyzerCategory != activation.MissNone {
		return analyzerCategory, nil
	}
	return activation.MissTargetUnreachable, nil
}

func rtaRepresents(kind activation.EdgeKind) bool {
	switch kind {
	case activation.DirectCall,
		activation.ConcreteMethodCall,
		activation.InterfaceDispatch,
		activation.StructFieldFuncValue,
		activation.StructLiteralFieldAssignment,
		activation.PackageVarFuncValue,
		activation.CallbackRegistration,
		activation.MapFuncValue,
		activation.GoroutineLaunch:
		return true
	default:
		return false
	}
}

// ScoreTier2 returns exact and fuzzy Jaccard similarity over intermediate
// functions. The entrypoint and target are excluded when present.
func ScoreTier2(expected, actual []activation.FunctionKey) (exact, fuzzy float64) {
	return jaccard(intermediate(expected), intermediate(actual), false),
		jaccard(intermediate(expected), intermediate(actual), true)
}

func intermediate(keys []activation.FunctionKey) []activation.FunctionKey {
	cleaned := make([]activation.FunctionKey, 0, len(keys))
	for _, key := range keys {
		if !key.IsZero() {
			cleaned = append(cleaned, key)
		}
	}
	if len(cleaned) <= 2 {
		return nil
	}
	return cleaned[1 : len(cleaned)-1]
}

func jaccard(expected, actual []activation.FunctionKey, fuzzy bool) float64 {
	expectedSet := keySet(expected, fuzzy)
	actualSet := keySet(actual, fuzzy)
	if len(expectedSet) == 0 && len(actualSet) == 0 {
		return 1
	}
	var intersection int
	for key := range expectedSet {
		if actualSet[key] {
			intersection++
		}
	}
	union := len(expectedSet)
	for key := range actualSet {
		if !expectedSet[key] {
			union++
		}
	}
	if union == 0 {
		return 1
	}
	return float64(intersection) / float64(union)
}

func keySet(keys []activation.FunctionKey, fuzzy bool) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		if fuzzy {
			key = key.Fuzzy()
		}
		if !key.IsZero() {
			set[key.String()] = true
		}
	}
	return set
}

// ScoreTier3 reports the fraction of trace function steps whose file:line
// appears in the analyzer path.
func ScoreTier3(trace Trace, target Target, path *activation.Path) float64 {
	var expected []sourcePoint
	for _, step := range trace.Steps {
		if step.Func == nil {
			continue
		}
		point, ok := traceStepPoint(step, target)
		if ok {
			expected = append(expected, point)
		}
	}
	if len(expected) == 0 {
		return 0
	}
	actual := map[sourcePoint]bool{}
	if path != nil {
		for _, step := range path.Steps {
			if step.Node == nil || step.Node.Position.File == "" || step.Node.Position.Line == 0 {
				continue
			}
			actual[actualPoint(step.Node.Position, target)] = true
		}
	}
	var matches int
	for _, point := range expected {
		if actual[point] {
			matches++
		}
	}
	return float64(matches) / float64(len(expected))
}

type sourcePoint struct {
	File string
	Line int
}

func traceStepPoint(step TraceStep, target Target) (sourcePoint, bool) {
	file, line, ok := splitTraceLocation(step.To)
	if !ok {
		return sourcePoint{}, false
	}
	path, err := traceFilePath(file+":"+strconv.Itoa(line), target)
	if err != nil {
		return sourcePoint{}, false
	}
	return sourcePoint{File: filepath.ToSlash(path), Line: line}, true
}

func actualPoint(position activation.Position, target Target) sourcePoint {
	file := filepath.ToSlash(filepath.Clean(position.File))
	projectPrefix := filepath.ToSlash(target.ProjectDir) + "/"
	workPrefix := filepath.ToSlash(target.WorkDir) + "/"
	switch {
	case strings.HasPrefix(file, workPrefix):
		file = strings.TrimPrefix(file, workPrefix)
	case strings.HasPrefix(file, projectPrefix):
		file = strings.TrimPrefix(file, projectPrefix)
	}
	if target.Name == "mattermost" {
		file = strings.TrimPrefix(file, "server/")
	}
	return sourcePoint{File: file, Line: position.Line}
}

func splitTraceLocation(raw string) (string, int, bool) {
	cleaned := cleanTraceLocation(raw)
	idx := strings.LastIndex(cleaned, ":")
	if idx <= 0 || idx == len(cleaned)-1 {
		return "", 0, false
	}
	line, err := strconv.Atoi(cleaned[idx+1:])
	if err != nil {
		return "", 0, false
	}
	return cleaned[:idx], line, true
}
