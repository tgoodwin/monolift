package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tgoodwin/monolift/pkg/activation"
)

// Trace is one structured SPRINT-0034 activation-path trace.
type Trace struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	RegionRoot      string         `json:"region_root"`
	PathLength      int            `json:"path_length"`
	FullyResolvable string         `json:"fully_resolvable"`
	HardestEdge     string         `json:"hardest_edge"`
	Steps           []TraceStep    `json:"steps"`
	EdgeSummary     map[string]int `json:"edge_summary"`
	Project         string         `json:"project"`
	SourceFile      string         `json:"source_file"`
	File            string         `json:"-"`
	CandidateNumber int            `json:"-"`
}

// TraceStep is one edge in a structured trace.
type TraceStep struct {
	Step     int     `json:"step"`
	From     *string `json:"from"`
	To       string  `json:"to"`
	Func     *string `json:"func"`
	EdgeType string  `json:"edge_type"`
	FromRaw  string  `json:"from_raw"`
	ToRaw    string  `json:"to_raw"`
}

// CanonicalEdgeKind maps the trace step's raw synthesis edge type to the
// activation package taxonomy.
func (s TraceStep) CanonicalEdgeKind() activation.EdgeKindMapping {
	return activation.CanonicalizeTraceEdgeKind(s.EdgeType)
}

// LoadTrace reads one JSON trace file from disk.
func LoadTrace(path string) (Trace, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Trace{}, err
	}
	var trace Trace
	if err := json.Unmarshal(data, &trace); err != nil {
		return Trace{}, fmt.Errorf("parse %s: %w", path, err)
	}
	trace.File = path
	trace.CandidateNumber = candidateNumber(filepath.Base(path))
	if trace.ID == "" {
		return Trace{}, fmt.Errorf("parse %s: missing id", path)
	}
	if trace.Project == "" {
		parts := strings.Split(trace.ID, "/")
		if len(parts) > 0 {
			trace.Project = parts[0]
		}
	}
	return trace, nil
}

// LoadTraces reads all JSON traces in dir, sorted by project and candidate
// number for deterministic evaluation.
func LoadTraces(dir string) ([]Trace, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var traces []Trace
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		trace, err := LoadTrace(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		traces = append(traces, trace)
	}
	sort.Slice(traces, func(i, j int) bool {
		if traces[i].Project != traces[j].Project {
			return traces[i].Project < traces[j].Project
		}
		if traces[i].CandidateNumber != traces[j].CandidateNumber {
			return traces[i].CandidateNumber < traces[j].CandidateNumber
		}
		return traces[i].ID < traces[j].ID
	})
	return traces, nil
}

func candidateNumber(base string) int {
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	idx := strings.LastIndex(stem, "-M-")
	if idx < 0 {
		return 0
	}
	n, _ := strconv.Atoi(stem[idx+3:])
	return n
}
