// Cut-placement analysis ranks single-node network boundaries along an
// existing activation path. Call AnalyzeCut with a Result whose Path contains
// SSA-backed Nodes, plus the Graph used to build the path when available. The
// analyzer applies the SPRINT-0039 decision tree: reject boundary-data shapes
// that cannot cross a network boundary, classify proxy-required candidates,
// prefer zero-callback cuts, then compare state reconstruction, surface area,
// error semantics, and edge alignment lexicographically. It deliberately does
// not collapse those dimensions into a numeric score.
//
// First-version limits: analysis is single-node only, path-local, and depends
// on in-memory SSA function data. It does not model composite cuts, liftability
// facts, transport selection, or graph-global merging of shared handlers.
package activation

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// CutResult contains the ranked cut-placement candidates for one activation
// path.
type CutResult struct {
	Recommended *CutCandidate  `json:"recommended,omitempty"`
	Candidates  []CutCandidate `json:"candidates"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

// CutCandidate is one possible single-node network boundary on an activation
// path.
type CutCandidate struct {
	Step         int               `json:"step"`
	NodeKey      FunctionKey       `json:"node_key"`
	NodeName     string            `json:"node_name"`
	IncomingEdge EdgeKind          `json:"incoming_edge"`
	Feasibility  CutFeasibility    `json:"feasibility"`
	BoundaryData BoundaryDataClass `json:"boundary_data"`
	Callbacks    CallbackClass     `json:"callbacks"`
	State        StateClass        `json:"state"`
	Surface      SurfaceClass      `json:"surface"`
	ErrorSem     ErrorSemClass     `json:"error_sem"`
	EdgeAlign    EdgeAlignClass    `json:"edge_align"`
	AdapterClass AdapterClass      `json:"adapter_class,omitempty"`
	AdapterReason string           `json:"adapter_reason,omitempty"`
	Reason       string            `json:"reason"`
}

// AnalyzeCut ranks single-node cut placements for an existing activation path.
func AnalyzeCut(result *Result, graph *Graph) (*CutResult, error) {
	cut := &CutResult{}
	hydratePathFunctions(result, graph)
	if err := validateCutPath(result, cut); err != nil {
		return cut, err
	}
	cut.Candidates = buildCutCandidates(result.Path, cut)
	rankCutCandidates(cut)
	return cut, nil
}

// DemoteCandidate marks a candidate as infeasible after codegen admission
// refuses it. The reason should include the admission refusal code and details.
func (cut *CutResult) DemoteCandidate(step int, nodeKey FunctionKey, reason string) {
	if cut == nil {
		return
	}
	demotionReason := "admission-refused"
	if strings.TrimSpace(reason) != "" {
		demotionReason += ": " + reason
	}
	for i := range cut.Candidates {
		if cut.Candidates[i].Step != step || cut.Candidates[i].NodeKey != nodeKey {
			continue
		}
		cut.Candidates[i].Feasibility = Infeasible
		cut.Candidates[i].Reason = demotionReason
		cut.Recommended = nil
		rankCutCandidates(cut)
		return
	}
}

func hydratePathFunctions(result *Result, graph *Graph) {
	if result == nil || result.Path == nil || graph == nil {
		return
	}
	byID := map[int]*Node{}
	byKey := map[string]*Node{}
	for _, node := range graph.Nodes {
		if node == nil {
			continue
		}
		byID[node.ID] = node
		if !node.Key.IsZero() {
			byKey[node.Key.String()] = node
		}
	}
	for _, step := range result.Path.Steps {
		if step.Node == nil || step.Node.Func != nil {
			continue
		}
		if graphNode := byID[step.Node.ID]; graphNode != nil && graphNode.Func != nil {
			step.Node.Func = graphNode.Func
			continue
		}
		if !step.Node.Key.IsZero() {
			if graphNode := byKey[step.Node.Key.String()]; graphNode != nil && graphNode.Func != nil {
				step.Node.Func = graphNode.Func
			}
		}
	}
}

func buildCutCandidates(path *Path, cut *CutResult) []CutCandidate {
	if path == nil || len(path.Steps) <= 1 {
		return nil
	}

	projectModule := inferProjectModule(path)
	candidates := make([]CutCandidate, 0, len(path.Steps)-1)
	for stepIndex := 1; stepIndex < len(path.Steps); stepIndex++ {
		step := path.Steps[stepIndex]
		node := step.Node
		incoming := incomingEdgeKind(stepIndex, step, cut)
		boundary, boundaryReasons := classifyBoundaryData(node.Func)
		callbacks := classifyCallbacks(node.Func, nodesAboveCut(path, stepIndex))
		feasibility := Feasible
		if boundary == BoundaryInfeasible || boundary == ProxyRequired {
			feasibility = Infeasible
		}
		if feasibility != Infeasible && !isProjectPackage(node.Package, projectModule) {
			feasibility = Infeasible
			boundaryReasons = append(boundaryReasons, "function is outside the project module ("+projectModule+")")
		}

		adapterClass := defaultAdapterClass(boundary)

		candidate := CutCandidate{
			Step:         stepIndex,
			NodeKey:      node.Key,
			NodeName:     node.Name,
			IncomingEdge: incoming,
			Feasibility:  feasibility,
			BoundaryData: boundary,
			Callbacks:    callbacks,
			State:        classifyState(node.Func),
			Surface:      classifySurface(stepIndex, len(path.Steps)),
			ErrorSem:     classifyErrorSemantics(node.Func),
			EdgeAlign:    classifyEdgeAlignment(incoming),
			AdapterClass: adapterClass,
		}
		if boundary == BoundaryInfeasible {
			candidate.Reason = "rejected by boundary-data hard gate: " + strings.Join(boundaryReasons, "; ")
			if cut != nil {
				cut.Diagnostics = append(cut.Diagnostics, Diagnostic{
					Severity: "warning",
					Phase:    "cut-boundary",
					Message:  fmt.Sprintf("rejected cut at step %d node %s: %s", stepIndex, cutNodeLabel(node), strings.Join(boundaryReasons, "; ")),
				})
			}
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func incomingEdgeKind(stepIndex int, step PathStep, cut *CutResult) EdgeKind {
	if step.Edge != nil {
		return step.Edge.Kind
	}
	if cut != nil {
		cut.Diagnostics = append(cut.Diagnostics, Diagnostic{
			Severity: "warning",
			Phase:    "cut-enumeration",
			Message:  fmt.Sprintf("path step %d node %s has no incoming edge; using Unsupported", stepIndex, cutNodeLabel(step.Node)),
		})
	}
	return Unsupported
}

func nodesAboveCut(path *Path, step int) []*Node {
	if path == nil || step <= 0 {
		return nil
	}
	nodes := make([]*Node, 0, step)
	for i := 0; i < step && i < len(path.Steps); i++ {
		nodes = append(nodes, path.Steps[i].Node)
	}
	return nodes
}

func rankCutCandidates(cut *CutResult) {
	if cut == nil || len(cut.Candidates) == 0 {
		return
	}
	sort.SliceStable(cut.Candidates, func(i, j int) bool {
		return betterCutCandidate(cut.Candidates[i], cut.Candidates[j])
	})
	for i := range cut.Candidates {
		if cut.Candidates[i].Feasibility != Infeasible {
			cut.Recommended = &cut.Candidates[i]
			break
		}
	}
	annotateCutReasons(cut)
}

func betterCutCandidate(a, b CutCandidate) bool {
	if a.Feasibility == Infeasible || b.Feasibility == Infeasible {
		if a.Feasibility != b.Feasibility {
			return b.Feasibility == Infeasible
		}
		return cutTieBreak(a, b)
	}
	// Surface area ranks first among soft dimensions. The corpus shows mean
	// recommended depth 0.924 — deep cuts dominate. Without this priority,
	// shallow bootstrap functions (stateless, zero callbacks, VeryLarge surface)
	// beat deep service functions on callbacks/state despite extracting the
	// entire application.
	if rankA, rankB := surfaceRank(a.Surface), surfaceRank(b.Surface); rankA != rankB {
		return rankA < rankB
	}
	if rankA, rankB := callbackRank(a.Callbacks), callbackRank(b.Callbacks); rankA != rankB {
		return rankA < rankB
	}
	if rankA, rankB := stateRankForSort(a.State), stateRankForSort(b.State); rankA != rankB {
		return rankA < rankB
	}
	if rankA, rankB := errorSemRank(a.ErrorSem), errorSemRank(b.ErrorSem); rankA != rankB {
		return rankA < rankB
	}
	if rankA, rankB := edgeAlignRank(a.EdgeAlign), edgeAlignRank(b.EdgeAlign); rankA != rankB {
		return rankA < rankB
	}
	if a.Feasibility != b.Feasibility {
		return a.Feasibility == Feasible
	}
	return cutTieBreak(a, b)
}

func cutTieBreak(a, b CutCandidate) bool {
	if a.Step != b.Step {
		return a.Step > b.Step
	}
	return a.NodeKey.String() < b.NodeKey.String()
}

func annotateCutReasons(cut *CutResult) {
	if cut.Recommended == nil {
		for i := range cut.Candidates {
			if cut.Candidates[i].Reason == "" {
				cut.Candidates[i].Reason = "no recommended cut; candidate did not pass hard gates"
			}
		}
		if len(cut.Candidates) == 0 {
			cut.Diagnostics = append(cut.Diagnostics, Diagnostic{
				Severity: "info",
				Phase:    "cut-ranking",
				Message:  "activation path has no non-entrypoint steps, so no cut is available",
			})
		}
		return
	}

	best := *cut.Recommended
	for i := range cut.Candidates {
		if cut.Candidates[i].Feasibility == Infeasible {
			continue
		}
		if cut.Candidates[i].Step == best.Step && cut.Candidates[i].NodeKey == best.NodeKey {
			if len(cut.Candidates) == 1 {
				cut.Candidates[i].Reason = "selected as the only candidate after the boundary-data hard gate"
			} else {
				cut.Candidates[i].Reason = "selected after the boundary-data hard gate; " + decisiveReasonAgainstNext(cut.Candidates)
			}
			cut.Recommended = &cut.Candidates[i]
			continue
		}
		cut.Candidates[i].Reason = "ranked below recommended step " + fmt.Sprint(best.Step) + ": " + decisiveDifference(best, cut.Candidates[i])
	}
}

func decisiveReasonAgainstNext(candidates []CutCandidate) string {
	if len(candidates) < 2 {
		return "no competing feasible candidate remained"
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Feasibility != Infeasible {
			return decisiveDifference(best, candidate)
		}
	}
	return "no competing feasible candidate remained"
}

func decisiveDifference(best, other CutCandidate) string {
	if other.Feasibility == Infeasible {
		return "competitor failed the boundary-data hard gate"
	}
	if best.Feasibility != other.Feasibility {
		return "feasible candidate was preferred over infeasible"
	}
	if surfaceRank(best.Surface) != surfaceRank(other.Surface) {
		return fmt.Sprintf("surface %s beat %s", best.Surface, other.Surface)
	}
	if callbackRank(best.Callbacks) != callbackRank(other.Callbacks) {
		return fmt.Sprintf("callbacks %s beat %s", best.Callbacks, other.Callbacks)
	}
	if stateRankForSort(best.State) != stateRankForSort(other.State) {
		return fmt.Sprintf("state %s beat %s", best.State, other.State)
	}
	if errorSemRank(best.ErrorSem) != errorSemRank(other.ErrorSem) {
		return fmt.Sprintf("error semantics %s beat %s", best.ErrorSem, other.ErrorSem)
	}
	if edgeAlignRank(best.EdgeAlign) != edgeAlignRank(other.EdgeAlign) {
		return fmt.Sprintf("edge alignment %s beat %s", best.EdgeAlign, other.EdgeAlign)
	}
	if best.Step != other.Step {
		return "deeper path step won the deterministic tiebreaker"
	}
	return "node key won the deterministic tiebreaker"
}

func callbackRank(class CallbackClass) int {
	switch class {
	case ZeroConfirmed, ZeroEstimated:
		return 0
	case Low:
		return 1
	case Moderate:
		return 2
	case Many:
		return 3
	default:
		return 4
	}
}

func stateRankForSort(class StateClass) int {
	switch class {
	case Stateless:
		return 0
	case ConfigOnly:
		return 1
	case ClientReconstructible:
		return 2
	case SharedState:
		return 3
	default:
		return 4
	}
}

func surfaceRank(class SurfaceClass) int {
	switch class {
	case Minimal:
		return 0
	case Small:
		return 1
	case Medium:
		return 2
	case Large:
		return 3
	case VeryLarge:
		return 4
	default:
		return 5
	}
}

func errorSemRank(class ErrorSemClass) int {
	switch class {
	case ErrorOK:
		return 0
	case NeedsWrapper:
		return 1
	case ErrorInfeasible:
		return 2
	default:
		return 3
	}
}

func edgeAlignRank(class EdgeAlignClass) int {
	switch class {
	case Strong:
		return 0
	case Weak:
		return 1
	case Anti:
		return 2
	default:
		return 3
	}
}

func validateCutPath(result *Result, cut *CutResult) error {
	if result == nil {
		return cutValidationError(cut, "cut analysis requires a non-nil Result")
	}
	if result.Path == nil {
		return cutValidationError(cut, "cut analysis requires Result.Path")
	}
	if len(result.Path.Steps) == 0 {
		return cutValidationError(cut, "cut analysis requires at least one path step")
	}
	for i, step := range result.Path.Steps {
		if step.Node == nil {
			return cutValidationError(cut, "cut analysis requires every path step to have a node; step %d is nil", i)
		}
		if step.Node.Func == nil {
			return cutValidationError(cut, "cut analysis requires SSA function data; step %d node %s has nil Func", i, cutNodeLabel(step.Node))
		}
	}
	return nil
}

func cutValidationError(cut *CutResult, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	if cut != nil {
		cut.Diagnostics = append(cut.Diagnostics, Diagnostic{
			Severity: "error",
			Phase:    "cut-validation",
			Message:  message,
		})
	}
	return errors.New(message)
}

func cutNodeLabel(node *Node) string {
	if node == nil {
		return "<nil>"
	}
	if !node.Key.IsZero() {
		return node.Key.String()
	}
	if node.Name != "" {
		return node.Name
	}
	return fmt.Sprintf("#%d", node.ID)
}

func inferProjectModule(path *Path) string {
	if path == nil || len(path.Steps) == 0 {
		return ""
	}
	target := path.Steps[len(path.Steps)-1].Node
	if target == nil {
		return ""
	}
	pkg := target.Package
	if pkg == "" {
		return ""
	}
	parts := strings.Split(pkg, "/")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], "/")
	}
	return pkg
}

func isProjectPackage(pkg, projectModule string) bool {
	if projectModule == "" || pkg == "" {
		return true
	}
	if pkg == "main" || pkg == "command-line-arguments" {
		return true
	}
	return strings.HasPrefix(pkg, projectModule)
}
