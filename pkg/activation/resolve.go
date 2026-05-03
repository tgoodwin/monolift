package activation

import (
	"fmt"
	"go/ast"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// TargetCandidate is returned when target resolution cannot find an exact
// containing function.
type TargetCandidate struct {
	Key      FunctionKey `json:"key"`
	Name     string      `json:"name"`
	Position Position    `json:"position"`
	Distance int         `json:"distance"`
}

// TargetNotFoundError reports a failed file:line target lookup with nearest
// same-file candidates.
type TargetNotFoundError struct {
	File       string            `json:"file"`
	Line       int               `json:"line"`
	Candidates []TargetCandidate `json:"candidates"`
}

func (e *TargetNotFoundError) Error() string {
	if e == nil {
		return "target not found"
	}
	return fmt.Sprintf("target not found at %s:%d", e.File, e.Line)
}

// ResolveTarget finds the SSA function whose source range contains file:line.
func (c Config) ResolveTarget(program *Program, file string, line int) (*ssa.Function, error) {
	if program == nil {
		return nil, fmt.Errorf("program is nil")
	}
	program.BuildSSA()
	if program.SSAProgram == nil {
		return nil, fmt.Errorf("SSA program was not built")
	}
	var matches []*ssa.Function
	var candidates []TargetCandidate
	for _, fn := range sortedFunctions(program.SSAProgram) {
		syntax := fn.Syntax()
		if syntax == nil {
			continue
		}
		start := program.Fset.Position(syntax.Pos())
		end := program.Fset.Position(syntax.End())
		if !sameSourceFile(start.Filename, file, c.Dir) {
			continue
		}
		distance := lineDistance(line, start.Line, end.Line)
		candidates = append(candidates, TargetCandidate{
			Key:      FunctionKeyForSSA(fn),
			Name:     fn.String(),
			Position: Position{File: start.Filename, Line: start.Line, Column: start.Column},
			Distance: distance,
		})
		if containsLine(syntax, program, line) {
			matches = append(matches, fn)
		}
	}
	if len(matches) > 0 {
		sort.Slice(matches, func(i, j int) bool {
			ri := sourceRangeSize(program, matches[i].Syntax())
			rj := sourceRangeSize(program, matches[j].Syntax())
			if ri != rj {
				return ri < rj
			}
			return FunctionKeyForSSA(matches[i]).String() < FunctionKeyForSSA(matches[j]).String()
		})
		return matches[0], nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Distance != candidates[j].Distance {
			return candidates[i].Distance < candidates[j].Distance
		}
		return candidates[i].Key.String() < candidates[j].Key.String()
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return nil, &TargetNotFoundError{File: file, Line: line, Candidates: candidates}
}

func containsLine(node ast.Node, program *Program, line int) bool {
	start := program.Fset.Position(node.Pos())
	end := program.Fset.Position(node.End())
	return start.Line <= line && line <= end.Line
}

func sourceRangeSize(program *Program, node ast.Node) int {
	if node == nil {
		return 1 << 30
	}
	start := program.Fset.Position(node.Pos())
	end := program.Fset.Position(node.End())
	return end.Line - start.Line
}

func lineDistance(line, start, end int) int {
	if line < start {
		return start - line
	}
	if line > end {
		return line - end
	}
	return 0
}
