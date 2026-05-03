package activation

import (
	"sort"
	"strings"
)

// Gap describes the first unresolved edge while walking an expected trace.
type Gap struct {
	AfterStep    int    `json:"after_step"`
	ExpectedEdge string `json:"expected_edge"`
	Reason       string `json:"reason"`
}

// PartialPath contains the resolved prefix and the first labeled gap.
type PartialPath struct {
	Prefix *Path `json:"prefix,omitempty"`
	Gap    Gap   `json:"gap"`
}

const (
	GapStructFieldNotResolved      = "struct-field-not-resolved"
	GapFrameworkPredicateMissing   = "framework-predicate-not-registered"
	GapClosureCaptureDeferred      = "closure-capture-deferred"
	GapChannelFlowDeferred         = "channel-flow-deferred"
	GapHTTPRegistrationDeferred    = "http-registration-deferred"
	GapReflectionDeferred          = "reflection-deferred"
	GapStringKeyedRegistryDeferred = "string-keyed-registry-deferred"
	GapCrossProcessDeferred        = "cross-process-deferred"
	GapTargetNotLoaded             = "target-not-loaded"
	GapUnknownUnreachable          = "unknown-unreachable"
)

// ExpectedStep is the activation package's trace-neutral representation of an
// expected path step.
type ExpectedStep struct {
	Step      int
	Key       FunctionKey
	EdgeKind  EdgeKind
	RawEdge   string
	GapReason string
}

// FindPartialPath walks expected trace steps against graph edges and returns
// the longest resolved prefix plus the first labeled gap.
func FindPartialPath(graph *Graph, expected []ExpectedStep) *PartialPath {
	if graph == nil || len(expected) == 0 {
		return &PartialPath{Gap: Gap{AfterStep: 0, Reason: GapUnknownUnreachable}}
	}
	startIndex := -1
	var current *Node
	for i, step := range expected {
		if step.Key.IsZero() {
			continue
		}
		current = findNodeByFuzzyKey(graph, step.Key)
		startIndex = i
		break
	}
	if current == nil {
		return &PartialPath{Gap: Gap{AfterStep: 0, ExpectedEdge: firstRawEdge(expected), Reason: GapTargetNotLoaded}}
	}

	prefix := &Path{Steps: []PathStep{{Node: current}}}
	lastResolved := expected[startIndex].Step
	for _, step := range expected[startIndex+1:] {
		if step.Key.IsZero() {
			return &PartialPath{
				Prefix: prefix,
				Gap: Gap{
					AfterStep:    lastResolved,
					ExpectedEdge: step.RawEdge,
					Reason:       gapReason(step),
				},
			}
		}
		edge := findExpectedEdge(graph, current, step.Key)
		if edge == nil {
			return &PartialPath{
				Prefix: prefix,
				Gap: Gap{
					AfterStep:    lastResolved,
					ExpectedEdge: step.RawEdge,
					Reason:       gapReason(step),
				},
			}
		}
		current = graph.Nodes[edge.To]
		prefix.Steps = append(prefix.Steps, PathStep{Node: current, Edge: edge})
		lastResolved = step.Step
	}
	return &PartialPath{
		Prefix: prefix,
		Gap: Gap{
			AfterStep:    lastResolved,
			ExpectedEdge: "",
			Reason:       GapUnknownUnreachable,
		},
	}
}

func findExpectedEdge(graph *Graph, current *Node, key FunctionKey) *Edge {
	if graph == nil || current == nil || key.IsZero() {
		return nil
	}
	edges := append([]*Edge(nil), graph.Out[current.ID]...)
	sort.SliceStable(edges, func(i, j int) bool {
		return edgeLess(graph, edges[i], edges[j])
	})
	for _, edge := range edges {
		if edge.To < 0 || edge.To >= len(graph.Nodes) {
			continue
		}
		if functionKeyMatches(graph.Nodes[edge.To].Key, key) {
			return edge
		}
	}
	return nil
}

func findNodeByFuzzyKey(graph *Graph, key FunctionKey) *Node {
	var matches []*Node
	for _, node := range graph.Nodes {
		if functionKeyMatches(node.Key, key) {
			matches = append(matches, node)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool {
		return nodeLess(matches[i], matches[j])
	})
	if len(matches) == 0 {
		return nil
	}
	return matches[0]
}

func functionKeyMatches(actual, expected FunctionKey) bool {
	if actual.String() == expected.String() {
		return true
	}
	actual = actual.Fuzzy()
	expected = expected.Fuzzy()
	if actual.String() == expected.String() {
		return true
	}
	return actual.PackagePath == expected.PackagePath &&
		actual.FuncName == expected.FuncName &&
		(actual.Receiver == "" || expected.Receiver == "")
}

func firstRawEdge(expected []ExpectedStep) string {
	for _, step := range expected {
		if step.RawEdge != "" {
			return step.RawEdge
		}
	}
	return ""
}

func gapReason(step ExpectedStep) string {
	if step.GapReason != "" {
		return step.GapReason
	}
	raw := strings.ToLower(cleanTraceEdgeKind(step.RawEdge))
	switch step.EdgeKind {
	case StructFieldFuncValue, StructLiteralFieldAssignment:
		if strings.Contains(raw, "framework") {
			return GapFrameworkPredicateMissing
		}
		return GapStructFieldNotResolved
	case ClosureCapture, CallbackRegistration:
		return GapClosureCaptureDeferred
	case ChannelFlow:
		return GapChannelFlowDeferred
	case HTTPHandlerRegistration:
		return GapHTTPRegistrationDeferred
	case Unsupported:
		switch {
		case strings.Contains(raw, "init-populated-registry") || strings.Contains(raw, "string-keyed-registry"):
			return GapStringKeyedRegistryDeferred
		case strings.Contains(raw, "reflect") || strings.Contains(raw, "funcmap") || strings.Contains(raw, "keyed-map") || strings.Contains(raw, "map-indexed"):
			return GapReflectionDeferred
		case strings.Contains(raw, "plugin") || strings.Contains(raw, "rpc") || strings.Contains(raw, "cross-process"):
			return GapCrossProcessDeferred
		}
	}
	return GapUnknownUnreachable
}
