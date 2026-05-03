package activation

import (
	"context"
	"errors"
	"fmt"
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
	_, err = timePhase(result, "augment", func() (struct{}, error) {
		return struct{}{}, Augment(graph, program, cfg.Augment)
	})
	if err := checkContext(ctx, result); err != nil {
		return result, err
	}
	if err != nil {
		result.Category = MissTargetUnreachable
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Phase: "augment", Message: err.Error()})
		return result, err
	}
	result.Stats = GraphStats{Nodes: len(graph.Nodes), Edges: len(graph.Edges)}
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
	value, err := fn()
	result.Timings = append(result.Timings, PhaseTiming{Phase: phase, Duration: time.Since(start)})
	return value, err
}
