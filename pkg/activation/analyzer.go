package activation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/go/ssa"
)

// NewAnalyzer constructs an Analyzer with conservative defaults.
func NewAnalyzer(config Config) *Analyzer {
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second
	}
	return &Analyzer{Config: config}
}

// Analyze runs the configured activation-path analysis.
func (a *Analyzer) Analyze(ctx context.Context) (*Result, error) {
	if a == nil {
		return nil, fmt.Errorf("analyzer is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := a.Config
	cfg.Context = ctx
	result := &Result{}
	file, line, err := ParseTarget(cfg.Target)
	if err != nil {
		result.Category = MissTargetNotFound
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "target", Message: err.Error()})
		return result, err
	}

	if cfg.ScopePackages && cfg.Target != "" {
		_, err := timePhase(result, "scope", func() (struct{}, error) {
			targetFile, _, parseErr := ParseTarget(cfg.Target)
			if parseErr != nil {
				return struct{}{}, nil
			}
			scoped, scopeErr := ReverseImportScope(cfg.Dir, targetFile, cfg.Env)
			if scopeErr != nil {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: "warning", Phase: "scope",
					Message: "reverse-import scoping failed, falling back to original patterns: " + scopeErr.Error(),
				})
				return struct{}{}, nil
			}
			result.Diagnostics = append(result.Diagnostics, Diagnostic{
				Severity: "info", Phase: "scope",
				Message: fmt.Sprintf("scoped from %v to %d packages", cfg.Packages, len(scoped)),
			})
			cfg.Packages = scoped
			return struct{}{}, nil
		})
		if err != nil {
			return result, err
		}
	}

	program, err := timePhase(result, "load", func() (*Program, error) {
		return cfg.LoadProgram()
	})
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			result.Category = MissTimeout
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "timeout", Message: ctx.Err().Error()})
			return result, ctx.Err()
		}
		result.Category = MissPackageLoadFailure
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "load", Message: err.Error()})
		return result, err
	}
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}

	_, err = timePhase(result, "ssa", func() (struct{}, error) {
		program.BuildSSA()
		return struct{}{}, nil
	})
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissPackageLoadFailure
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "ssa", Message: err.Error()})
		return result, err
	}

	target, err := timePhase(result, "resolve-target", func() (*ssaFunction, error) {
		fn, err := cfg.ResolveTarget(program, file, line)
		return (*ssaFunction)(fn), err
	})
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissTargetNotFound
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "resolve-target", Message: err.Error()})
		var nf *TargetNotFoundError
		if errors.As(err, &nf) {
			for _, candidate := range nf.Candidates {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{
					Severity: "info",
					Phase:    "resolve-target",
					Message:  "nearest candidate: " + candidate.Key.String(),
					Position: candidate.Position,
				})
			}
		}
		return result, err
	}
	result.Target = nodeForFunction(0, program, target.function())

	entryFns, err := timePhase(result, "entrypoints", func() ([]*ssaFunction, error) {
		fns, err := cfg.FindEntrypoints(program)
		wrapped := make([]*ssaFunction, 0, len(fns))
		for _, fn := range fns {
			wrapped = append(wrapped, (*ssaFunction)(fn))
		}
		return wrapped, err
	})
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissTargetUnreachable
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "entrypoints", Message: err.Error()})
		return result, err
	}
	for i, fn := range entryFns {
		result.Entrypoints = append(result.Entrypoints, nodeForFunction(i, program, fn.function()))
	}
	entrypointFuncs := make([]*ssa.Function, 0, len(entryFns))
	for _, fn := range entryFns {
		entrypointFuncs = append(entrypointFuncs, fn.function())
	}
	graph, err := timePhase(result, "rta", func() (*Graph, error) {
		return BuildRTAGraph(program, entrypointFuncs)
	})
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissTargetUnreachable
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "rta", Message: err.Error()})
		return result, err
	}
	preAugmentNodes := len(graph.Nodes)
	preAugmentEdges := len(graph.Edges)
	_, rtaFound := ShortestPath(graph, entrypointFuncs, target.function())
	if rtaFound && cfg.SkipAugmentWhenRTAReachable {
		result.SkippedAugment = true
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Severity: "info",
			Phase:    "augment",
			Message:  "target is reachable in RTA graph; skipped augmentation",
		})
		_, err = timePhase(result, "augment", func() (struct{}, error) {
			return struct{}{}, nil
		})
		setPhaseMetadata(result, "augment", map[string]any{"skipped": true})
	} else {
		_, err = timePhase(result, "augment", func() (struct{}, error) {
			return struct{}{}, Augment(graph, program, cfg.Augment, &result.SubTimings)
		})
	}
	result.Stats = graphStats(program, graph, preAugmentNodes, preAugmentEdges)
	augmentMetadata := graphStatsMetadata(result.Stats)
	if result.SkippedAugment {
		augmentMetadata["skipped"] = true
	}
	setPhaseMetadata(result, "augment", augmentMetadata)
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissTargetUnreachable
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "augment", Message: err.Error()})
		return result, err
	}
	result.Diagnostics = append(result.Diagnostics, graph.AugmentDiagnostics...)
	if graphTarget := graph.nodeByFunction(target.function()); graphTarget != nil {
		result.Target = graphTarget
	}
	result.Entrypoints = result.Entrypoints[:0]
	for _, fn := range entrypointFuncs {
		if node := graph.nodeByFunction(fn); node != nil {
			result.Entrypoints = append(result.Entrypoints, node)
		}
	}
	var bfsFound bool
	path, err := timePhase(result, "bfs", func() (*Path, error) {
		path, found := ShortestPath(graph, entrypointFuncs, target.function())
		bfsFound = found
		if !bfsFound {
			return nil, nil
		}
		return path, nil
	})
	if err != nil {
		result.Category = MissTargetUnreachable
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "bfs", Message: err.Error()})
		return result, err
	}
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if bfsFound {
		result.Found = true
		result.Category = MissNone
		result.Path = path
		return result, nil
	}
	result.Category = MissTargetUnreachable
	result.Diagnostics = append(result.Diagnostics, Diagnostic{
		Severity: "info",
		Phase:    "bfs",
		Message:  "target is not reachable in the RTA call graph",
	})
	return result, nil
}

func checkContext(ctx context.Context, result *Result) error {
	select {
	case <-ctx.Done():
		result.Category = MissTimeout
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "timeout", Message: ctx.Err().Error()})
		return ctx.Err()
	default:
		return nil
	}
}

// ParseTarget parses file:line target strings.
func ParseTarget(raw string) (string, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, fmt.Errorf("target is required")
	}
	idx := strings.LastIndex(raw, ":")
	if idx <= 0 || idx == len(raw)-1 {
		return "", 0, fmt.Errorf("target must be file:line, got %q", raw)
	}
	line, err := strconv.Atoi(raw[idx+1:])
	if err != nil || line <= 0 {
		return "", 0, fmt.Errorf("target line must be a positive integer, got %q", raw[idx+1:])
	}
	return raw[:idx], line, nil
}

type ssaFunction ssa.Function

func (f *ssaFunction) function() *ssa.Function {
	return (*ssa.Function)(f)
}

func nodeForFunction(id int, program *Program, fn *ssa.Function) *Node {
	return &Node{
		ID:       id,
		Key:      FunctionKeyForSSA(fn),
		Name:     fn.String(),
		Package:  FunctionKeyForSSA(fn).PackagePath,
		Position: positionFor(program, fn.Pos()),
		Func:     fn,
	}
}

func timePhase[T any](result *Result, phase string, fn func() (T, error)) (T, error) {
	start := time.Now()
	logActivationProgress("activation phase", phase, "start", 0)
	value, err := fn()
	logActivationProgress("activation phase", phase, progressStatus(err), time.Since(start))
	result.Timings = append(result.Timings, PhaseTiming{Phase: phase, Duration: time.Since(start)})
	return value, err
}

func timeSubPhase[T any](timings *[]PhaseTiming, phase string, fn func() (T, error)) (T, error) {
	start := time.Now()
	logActivationProgress("activation subphase", phase, "start", 0)
	value, err := fn()
	logActivationProgress("activation subphase", phase, progressStatus(err), time.Since(start))
	if timings != nil {
		*timings = append(*timings, PhaseTiming{Phase: phase, Duration: time.Since(start)})
	}
	return value, err
}

func logActivationProgress(message, phase, event string, duration time.Duration) {
	args := []any{"component", "activation", "phase", phase, "event", event}
	if duration > 0 {
		args = append(args, "duration", duration.Round(time.Millisecond))
	}
	slog.Debug(message, args...)
}

func progressStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "done"
}

func setPhaseMetadata(result *Result, phase string, metadata map[string]any) {
	if result == nil || len(metadata) == 0 {
		return
	}
	for i := len(result.Timings) - 1; i >= 0; i-- {
		if result.Timings[i].Phase == phase {
			result.Timings[i].Metadata = metadata
			return
		}
	}
}

func graphStats(program *Program, graph *Graph, preAugmentNodes, preAugmentEdges int) GraphStats {
	stats := GraphStats{}
	if program != nil {
		stats.SSAFunctions, stats.ScannedInstructions = countSSAFunctionsAndInstructions(program)
	}
	if graph != nil {
		stats.Nodes = len(graph.Nodes)
		stats.Edges = len(graph.Edges)
		stats.GraphFunctions = len(graph.Nodes)
		stats.AddedNodes = len(graph.Nodes) - preAugmentNodes
		stats.AddedEdges = len(graph.Edges) - preAugmentEdges
		stats.AugmentIterations = graph.AugmentIterations
		stats.AugmentLimitHit = graph.AugmentLimitHit
	}
	return stats
}

func graphStatsMetadata(stats GraphStats) map[string]any {
	return map[string]any{
		"ssa_functions":        stats.SSAFunctions,
		"graph_functions":      stats.GraphFunctions,
		"scanned_instructions": stats.ScannedInstructions,
		"added_nodes":          stats.AddedNodes,
		"added_edges":          stats.AddedEdges,
		"augment_iterations":   stats.AugmentIterations,
		"augment_limit_hit":    stats.AugmentLimitHit,
	}
}

func countSSAFunctionsAndInstructions(program *Program) (int, int) {
	if program == nil || program.SSAProgram == nil {
		return 0, 0
	}
	funcs := program.Functions()
	instructions := 0
	for _, fn := range funcs {
		for _, block := range fn.Blocks {
			instructions += len(block.Instrs)
		}
	}
	return len(funcs), instructions
}
