package entrypath

import (
	"sort"
	"strings"
	"time"

	"golang.org/x/tools/go/ssa"
)

const (
	OraclePhaseLoadedSSA                 = "loaded_ssa"
	OraclePhaseReverseBFSTouchpoint      = "reverse_bfs_touchpoint"
	OraclePhaseReverseFrontierOwner      = "reverse_frontier_owner"
	OraclePhaseAdjacentOwner             = "adjacent_owner"
	OraclePhaseBoundaryCandidate         = "boundary_candidate"
	OraclePhaseBoundaryPredicateEvidence = "boundary_predicate_evidence"
	OraclePhaseSeedSet                   = "seed_set"
	OraclePhaseFunctionRefIndex          = "function_ref_index"
	OraclePhaseFinalClassification       = "final_classification"

	OracleRelationshipWrapperLink      = "wrapper_link"
	OracleRelationshipRegistration     = "registration"
	OracleRelationshipBoundaryEvidence = "boundary_evidence"
	OracleRelationshipFunctionRef      = "function_ref"
)

var oraclePhases = []string{
	OraclePhaseLoadedSSA,
	OraclePhaseReverseBFSTouchpoint,
	OraclePhaseReverseFrontierOwner,
	OraclePhaseAdjacentOwner,
	OraclePhaseBoundaryCandidate,
	OraclePhaseBoundaryPredicateEvidence,
	OraclePhaseSeedSet,
	OraclePhaseFunctionRefIndex,
	OraclePhaseFinalClassification,
}

type OracleSpec struct {
	Nodes         []OracleNodeSpec         `json:"nodes,omitempty"`
	Relationships []OracleRelationshipSpec `json:"relationships,omitempty"`
	BridgeStarts  []string                 `json:"bridgeStarts,omitempty"`
}

type OracleNodeSpec struct {
	ID            string `json:"id"`
	PackagePath   string `json:"packagePath,omitempty"`
	ObjectName    string `json:"objectName,omitempty"`
	LabelContains string `json:"labelContains,omitempty"`
}

type OracleRelationshipSpec struct {
	ID                 string `json:"id"`
	Kind               string `json:"kind,omitempty"`
	From               string `json:"from,omitempty"`
	To                 string `json:"to,omitempty"`
	EdgeKind           string `json:"edgeKind,omitempty"`
	StaticTypeContains string `json:"staticTypeContains,omitempty"`
	SinkKind           string `json:"sinkKind,omitempty"`
}

type OracleTrace struct {
	Nodes         []OracleNodeTrace         `json:"nodes,omitempty"`
	Relationships []OracleRelationshipTrace `json:"relationships,omitempty"`
}

type OracleNodeTrace struct {
	ID                string                `json:"id"`
	Spec              OracleNodeSpec        `json:"spec"`
	Resolved          TraceNode             `json:"resolved,omitempty"`
	Phases            []OraclePhasePresence `json:"phases"`
	FirstMissingPhase string                `json:"firstMissingPhase,omitempty"`
	LikelyMechanism   string                `json:"likelyMechanism,omitempty"`
}

type OracleRelationshipTrace struct {
	ID                string                 `json:"id"`
	Spec              OracleRelationshipSpec `json:"spec"`
	Phases            []OraclePhasePresence  `json:"phases"`
	FirstMissingPhase string                 `json:"firstMissingPhase,omitempty"`
	LikelyMechanism   string                 `json:"likelyMechanism,omitempty"`
}

type OraclePhasePresence struct {
	Phase    string   `json:"phase"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence,omitempty"`
}

type oracleTraceBuilder struct {
	prog              *ssa.Program
	spec              OracleSpec
	nodesByID         map[string]*ssa.Function
	nodeTraceByID     map[string]TraceNode
	nodeIDsByFunction map[*ssa.Function][]string
	nodePhaseEvidence map[string]map[string][]string
	relPhaseEvidence  map[string]map[string][]string
	ranPhases         map[string]bool
}

func newOracleTraceBuilder(prog *ssa.Program, spec OracleSpec) *oracleTraceBuilder {
	if len(spec.Nodes) == 0 && len(spec.Relationships) == 0 && len(spec.BridgeStarts) == 0 {
		return nil
	}
	builder := &oracleTraceBuilder{
		prog:              prog,
		spec:              spec,
		nodesByID:         map[string]*ssa.Function{},
		nodeTraceByID:     map[string]TraceNode{},
		nodeIDsByFunction: map[*ssa.Function][]string{},
		nodePhaseEvidence: map[string]map[string][]string{},
		relPhaseEvidence:  map[string]map[string][]string{},
		ranPhases:         map[string]bool{},
	}
	builder.resolveNodes()
	return builder
}

func (builder *oracleTraceBuilder) resolveNodes() {
	builder.ranPhases[OraclePhaseLoadedSSA] = true
	functions := sortedProgramFunctions(builder.prog)
	for _, node := range builder.spec.Nodes {
		if node.ID == "" {
			continue
		}
		for _, fn := range functions {
			if oracleNodeMatches(builder.prog, node, fn) {
				builder.nodesByID[node.ID] = fn
				builder.nodeTraceByID[node.ID] = traceNodeForFunction(builder.prog, fn)
				builder.nodeIDsByFunction[fn] = append(builder.nodeIDsByFunction[fn], node.ID)
				builder.markNode(node.ID, OraclePhaseLoadedSSA, functionString(fn))
				break
			}
		}
	}
	for _, rel := range builder.spec.Relationships {
		if rel.ID == "" {
			continue
		}
		if builder.relationshipEndpointsResolved(rel) {
			builder.markRelationship(rel.ID, OraclePhaseLoadedSSA, "relationship endpoints resolved")
		}
	}
}

func oracleNodeMatches(prog *ssa.Program, spec OracleNodeSpec, fn *ssa.Function) bool {
	if fn == nil {
		return false
	}
	trace := traceNodeForFunction(prog, fn)
	if spec.PackagePath != "" && trace.Identity.PackagePath != spec.PackagePath {
		return false
	}
	if spec.ObjectName != "" && trace.Identity.ObjectName != spec.ObjectName {
		return false
	}
	if spec.LabelContains != "" && !strings.Contains(trace.Label, spec.LabelContains) && !strings.Contains(trace.ID, spec.LabelContains) {
		return false
	}
	return spec.PackagePath != "" || spec.ObjectName != "" || spec.LabelContains != ""
}

func (builder *oracleTraceBuilder) markTouchpoints(touchpoints []RegionTouchpoint) {
	if builder == nil {
		return
	}
	builder.ranPhases[OraclePhaseReverseBFSTouchpoint] = true
	for _, touchpoint := range touchpoints {
		builder.markTraceNode(touchpoint.Touchpoint, OraclePhaseReverseBFSTouchpoint, touchpoint.EdgeKind)
	}
}

func (builder *oracleTraceBuilder) markReverseFrontierOwners(owners []*ssa.Function) {
	builder.markFunctionPhase(OraclePhaseReverseFrontierOwner, owners, "reverse frontier owner")
}

func (builder *oracleTraceBuilder) markAdjacentOwners(owners []*ssa.Function) {
	builder.markFunctionPhase(OraclePhaseAdjacentOwner, owners, "adjacent owner")
}

func (builder *oracleTraceBuilder) markBoundaryCandidateOwners(owners []*ssa.Function) {
	builder.markFunctionPhase(OraclePhaseBoundaryCandidate, owners, "boundary candidate")
}

func (builder *oracleTraceBuilder) markSeedSet(seeds *FunctionIndexSeedSet) {
	if builder == nil || seeds == nil {
		return
	}
	builder.ranPhases[OraclePhaseSeedSet] = true
	for _, owner := range seeds.Owners() {
		builder.markFunction(owner, OraclePhaseSeedSet, strings.Join(seeds.Reasons(owner), ","))
	}
}

func (builder *oracleTraceBuilder) markBoundaryEvidence(evidence []BoundaryPredicateEvidence) {
	if builder == nil {
		return
	}
	builder.ranPhases[OraclePhaseBoundaryPredicateEvidence] = true
	for _, item := range evidence {
		builder.markFunction(item.Owner, OraclePhaseBoundaryPredicateEvidence, boundaryEvidenceString(item))
		for _, rel := range builder.spec.Relationships {
			if rel.Kind != OracleRelationshipBoundaryEvidence || rel.ID == "" {
				continue
			}
			owner := builder.relationshipOwner(rel)
			if owner != nil && owner == item.Owner && relationshipStaticTypeMatches(rel, item.StaticType) {
				builder.markRelationship(rel.ID, OraclePhaseBoundaryPredicateEvidence, boundaryEvidenceString(item))
			}
		}
	}
}

func (builder *oracleTraceBuilder) markFunctionRefIndex(index FunctionRefIndex) {
	if builder == nil {
		return
	}
	builder.ranPhases[OraclePhaseFunctionRefIndex] = true
	for id, fn := range builder.nodesByID {
		var evidence []string
		if index.ScannedOwners[fn] {
			evidence = append(evidence, "scanned owner")
		}
		if _, ok := index.Sources[fn]; ok {
			evidence = append(evidence, "source")
		}
		if refs := index.Uses[fn]; len(refs) > 0 {
			evidence = append(evidence, "uses:"+itoa(len(refs)))
		}
		for _, item := range evidence {
			builder.markNode(id, OraclePhaseFunctionRefIndex, item)
		}
	}
	for _, rel := range builder.spec.Relationships {
		if rel.ID == "" {
			continue
		}
		if evidence, ok := builder.relationshipInFunctionIndex(rel, index); ok {
			builder.markRelationship(rel.ID, OraclePhaseFunctionRefIndex, evidence)
		}
	}
}

func (builder *oracleTraceBuilder) markFinalClassifications(flow functionFlowResult) {
	if builder == nil {
		return
	}
	builder.ranPhases[OraclePhaseFinalClassification] = true
	for _, surface := range flow.ExternalSurfaces {
		builder.markTraceNode(surface.Node, OraclePhaseFinalClassification, "external surface")
	}
	for _, site := range flow.RegistrationSites {
		builder.markTraceNode(site.Node, OraclePhaseFinalClassification, "registration owner")
		builder.markTraceNode(site.Handler, OraclePhaseFinalClassification, "registration handler")
	}
	for _, chain := range flow.WrapperChains {
		builder.markTraceNode(chain.ExternalSurface, OraclePhaseFinalClassification, "chain source")
		builder.markTraceNode(chain.RegistrationSite, OraclePhaseFinalClassification, "chain registration")
		for _, link := range chain.Links {
			builder.markTraceNode(link.From, OraclePhaseFinalClassification, "chain link from")
			builder.markTraceNode(link.To, OraclePhaseFinalClassification, "chain link to")
		}
	}
	for _, rel := range builder.spec.Relationships {
		if rel.ID == "" {
			continue
		}
		if evidence, ok := builder.relationshipInFinalClassifications(rel, flow); ok {
			builder.markRelationship(rel.ID, OraclePhaseFinalClassification, evidence)
		}
	}
}

func (builder *oracleTraceBuilder) trace() *OracleTrace {
	if builder == nil {
		return nil
	}
	trace := &OracleTrace{}
	for _, spec := range builder.spec.Nodes {
		if spec.ID == "" {
			continue
		}
		phases := builder.phasesFor(builder.nodePhaseEvidence[spec.ID])
		missing := firstMissingNodePhase(phases)
		trace.Nodes = append(trace.Nodes, OracleNodeTrace{
			ID:                spec.ID,
			Spec:              spec,
			Resolved:          builder.nodeTraceByID[spec.ID],
			Phases:            phases,
			FirstMissingPhase: missing,
			LikelyMechanism:   likelyMechanism(missing),
		})
	}
	for _, spec := range builder.spec.Relationships {
		if spec.ID == "" {
			continue
		}
		phases := builder.phasesFor(builder.relPhaseEvidence[spec.ID])
		missing := firstMissingRelationshipPhase(spec, phases)
		trace.Relationships = append(trace.Relationships, OracleRelationshipTrace{
			ID:                spec.ID,
			Spec:              spec,
			Phases:            phases,
			FirstMissingPhase: missing,
			LikelyMechanism:   likelyMechanism(missing),
		})
	}
	return trace
}

func (builder *oracleTraceBuilder) markFunctionPhase(phase string, owners []*ssa.Function, evidence string) {
	if builder == nil {
		return
	}
	builder.ranPhases[phase] = true
	for _, owner := range owners {
		builder.markFunction(owner, phase, evidence)
	}
}

func (builder *oracleTraceBuilder) markTraceNode(node TraceNode, phase string, evidence string) {
	if builder == nil || node.ID == "" {
		return
	}
	for id, resolved := range builder.nodeTraceByID {
		if resolved.ID == node.ID {
			builder.markNode(id, phase, evidence)
		}
	}
}

func (builder *oracleTraceBuilder) markFunction(fn *ssa.Function, phase string, evidence string) {
	if builder == nil || fn == nil {
		return
	}
	for _, id := range builder.nodeIDsByFunction[fn] {
		builder.markNode(id, phase, evidence)
	}
}

func (builder *oracleTraceBuilder) markNode(id, phase, evidence string) {
	if builder.nodePhaseEvidence[id] == nil {
		builder.nodePhaseEvidence[id] = map[string][]string{}
	}
	builder.nodePhaseEvidence[id][phase] = appendUniqueString(builder.nodePhaseEvidence[id][phase], evidence)
}

func (builder *oracleTraceBuilder) markRelationship(id, phase, evidence string) {
	if builder.relPhaseEvidence[id] == nil {
		builder.relPhaseEvidence[id] = map[string][]string{}
	}
	builder.relPhaseEvidence[id][phase] = appendUniqueString(builder.relPhaseEvidence[id][phase], evidence)
}

func (builder *oracleTraceBuilder) phasesFor(evidenceByPhase map[string][]string) []OraclePhasePresence {
	phases := make([]OraclePhasePresence, 0, len(oraclePhases))
	for _, phase := range oraclePhases {
		status := "not_run"
		if builder.ranPhases[phase] {
			status = "absent"
		}
		evidence := append([]string(nil), evidenceByPhase[phase]...)
		if len(evidence) > 0 {
			status = "present"
			sort.Strings(evidence)
		}
		phases = append(phases, OraclePhasePresence{
			Phase:    phase,
			Status:   status,
			Evidence: evidence,
		})
	}
	return phases
}

func (builder *oracleTraceBuilder) relationshipEndpointsResolved(rel OracleRelationshipSpec) bool {
	if rel.From != "" && builder.nodesByID[rel.From] == nil {
		return false
	}
	if rel.To != "" && builder.nodesByID[rel.To] == nil {
		return false
	}
	return rel.From != "" || rel.To != ""
}

func (builder *oracleTraceBuilder) relationshipOwner(rel OracleRelationshipSpec) *ssa.Function {
	if rel.To != "" {
		return builder.nodesByID[rel.To]
	}
	if rel.From != "" {
		return builder.nodesByID[rel.From]
	}
	return nil
}

func (builder *oracleTraceBuilder) relationshipInFunctionIndex(rel OracleRelationshipSpec, index FunctionRefIndex) (string, bool) {
	from := builder.nodesByID[rel.From]
	to := builder.nodesByID[rel.To]
	if from == nil {
		return "", false
	}
	for _, ref := range index.Uses[from] {
		if rel.EdgeKind != "" && !oracleRefEdgeMatches(ref, rel.EdgeKind) {
			continue
		}
		if to != nil && ref.Owner != to && oracleStaticCallee(ref) != to {
			continue
		}
		return ref.Kind + " in " + functionString(ref.Owner), true
	}
	return "", false
}

func oracleRefEdgeMatches(ref FunctionRef, edgeKind string) bool {
	switch edgeKind {
	case EdgeFunctionValueArg:
		return ref.Kind == "call_arg" || ref.Kind == "go_arg"
	case EdgeFunctionValueReturned:
		return ref.Kind == "return"
	case EdgeClosureCapture:
		return ref.Kind == "capture"
	case EdgeDirectInvoke:
		return ref.Kind == "direct_invoke"
	case EdgeGoroutineLaunch:
		return ref.Kind == "go_arg" || ref.Kind == "go_launch"
	default:
		return edgeKind == "" || ref.Kind == edgeKind
	}
}

func oracleStaticCallee(ref FunctionRef) *ssa.Function {
	call, ok := ref.Instruction.(ssa.CallInstruction)
	if !ok || call.Common() == nil {
		return nil
	}
	return call.Common().StaticCallee()
}

func (builder *oracleTraceBuilder) relationshipInFinalClassifications(rel OracleRelationshipSpec, flow functionFlowResult) (string, bool) {
	switch rel.Kind {
	case OracleRelationshipBoundaryEvidence:
		return "", false
	case OracleRelationshipRegistration:
		return builder.registrationRelationship(rel, flow.RegistrationSites)
	case OracleRelationshipWrapperLink, OracleRelationshipFunctionRef, "":
		if evidence, ok := builder.wrapperRelationship(rel, flow.WrapperChains); ok {
			return evidence, true
		}
		return builder.registrationRelationship(rel, flow.RegistrationSites)
	default:
		return "", false
	}
}

func (builder *oracleTraceBuilder) wrapperRelationship(rel OracleRelationshipSpec, chains []WrapperChain) (string, bool) {
	from := builder.nodeTraceByID[rel.From]
	to := builder.nodeTraceByID[rel.To]
	if from.ID == "" || to.ID == "" {
		return "", false
	}
	for _, chain := range chains {
		for _, link := range chain.Links {
			if link.From.ID != from.ID || link.To.ID != to.ID {
				continue
			}
			if rel.EdgeKind != "" && rel.EdgeKind != link.EdgeKind {
				continue
			}
			return link.EdgeKind, true
		}
	}
	return "", false
}

func (builder *oracleTraceBuilder) registrationRelationship(rel OracleRelationshipSpec, sites []RegistrationSite) (string, bool) {
	from := builder.nodeTraceByID[rel.From]
	to := builder.nodeTraceByID[rel.To]
	for _, site := range sites {
		if from.ID != "" && site.Handler.ID != from.ID {
			continue
		}
		if to.ID != "" && site.Node.ID != to.ID {
			continue
		}
		if rel.EdgeKind != "" && rel.EdgeKind != site.EdgeKind {
			continue
		}
		if rel.SinkKind != "" && rel.SinkKind != site.SinkKind {
			continue
		}
		if !relationshipStaticTypeMatches(rel, site.StaticParameterType) {
			continue
		}
		return site.EdgeKind + " " + site.StaticParameterType, true
	}
	return "", false
}

func relationshipStaticTypeMatches(rel OracleRelationshipSpec, staticType string) bool {
	return rel.StaticTypeContains == "" || strings.Contains(staticType, rel.StaticTypeContains)
}

func firstMissingPhase(phases []OraclePhasePresence) string {
	if phaseStatus(phases, OraclePhaseFinalClassification) == "present" {
		return ""
	}
	for _, phase := range phases {
		if phase.Status == "absent" {
			return phase.Phase
		}
	}
	return ""
}

func firstMissingNodePhase(phases []OraclePhasePresence) string {
	if phaseStatus(phases, OraclePhaseFinalClassification) == "present" {
		return ""
	}
	if phaseStatus(phases, OraclePhaseLoadedSSA) == "absent" {
		return OraclePhaseLoadedSSA
	}
	if phaseStatus(phases, OraclePhaseFunctionRefIndex) == "absent" {
		return OraclePhaseFunctionRefIndex
	}
	if phaseStatus(phases, OraclePhaseSeedSet) == "absent" {
		return OraclePhaseSeedSet
	}
	if phaseStatus(phases, OraclePhaseBoundaryPredicateEvidence) == "absent" {
		return OraclePhaseBoundaryPredicateEvidence
	}
	if phaseStatus(phases, OraclePhaseReverseBFSTouchpoint) == "absent" {
		return OraclePhaseReverseBFSTouchpoint
	}
	if phaseStatus(phases, OraclePhaseFinalClassification) == "absent" {
		return OraclePhaseFinalClassification
	}
	return firstMissingPhase(phases)
}

func firstMissingRelationshipPhase(spec OracleRelationshipSpec, phases []OraclePhasePresence) string {
	if phaseStatus(phases, OraclePhaseFinalClassification) == "present" {
		return ""
	}
	if spec.Kind == OracleRelationshipBoundaryEvidence {
		switch phaseStatus(phases, OraclePhaseBoundaryPredicateEvidence) {
		case "present", "not_run":
			return ""
		case "absent":
			return OraclePhaseBoundaryPredicateEvidence
		}
	}
	if phaseStatus(phases, OraclePhaseLoadedSSA) == "absent" {
		return OraclePhaseLoadedSSA
	}
	if phaseStatus(phases, OraclePhaseFunctionRefIndex) == "absent" {
		return OraclePhaseFunctionRefIndex
	}
	if phaseStatus(phases, OraclePhaseFinalClassification) == "absent" {
		return OraclePhaseFinalClassification
	}
	return firstMissingPhase(phases)
}

func phaseStatus(phases []OraclePhasePresence, want string) string {
	for _, phase := range phases {
		if phase.Phase == want {
			return phase.Status
		}
	}
	return "not_run"
}

func likelyMechanism(phase string) string {
	switch phase {
	case "":
		return ""
	case OraclePhaseLoadedSSA:
		return "loaded SSA coverage"
	case OraclePhaseReverseBFSTouchpoint:
		return "graph edge coverage"
	case OraclePhaseReverseFrontierOwner, OraclePhaseAdjacentOwner, OraclePhaseBoundaryCandidate, OraclePhaseSeedSet:
		return "owner selection or ordering"
	case OraclePhaseBoundaryPredicateEvidence:
		return "boundary predicate rejection"
	case OraclePhaseFunctionRefIndex:
		return "function-value flow or index coverage"
	case OraclePhaseFinalClassification:
		return "final classification"
	default:
		return "unknown"
	}
}

func boundaryEvidenceString(item BoundaryPredicateEvidence) string {
	parts := []string{item.Predicate}
	if item.Reason != "" {
		parts = append(parts, item.Reason)
	}
	if item.StaticType != "" {
		parts = append(parts, item.StaticType)
	}
	return strings.Join(parts, " ")
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

type OracleBridgeOptions struct {
	MaxPackageFunctions int
	MaxOwners           int
	MaxDuration         time.Duration
}

func oracleBridgeOptionsFromProbe(options ProbeOptions) OracleBridgeOptions {
	bridge := OracleBridgeOptions{
		MaxPackageFunctions: options.OracleBridgeMaxPackageFunctions,
		MaxOwners:           options.OracleBridgeMaxOwners,
		MaxDuration:         options.OracleBridgeMaxDuration,
	}
	if bridge.MaxPackageFunctions == 0 {
		bridge.MaxPackageFunctions = 2000
	}
	if bridge.MaxOwners == 0 {
		bridge.MaxOwners = 500
	}
	if bridge.MaxDuration == 0 {
		bridge.MaxDuration = 30 * time.Second
	}
	return bridge
}

func oracleBridgeSeedSet(prog *ssa.Program, spec OracleSpec, options OracleBridgeOptions) *FunctionIndexSeedSet {
	seeds := NewFunctionIndexSeedSet()
	builder := newOracleTraceBuilder(prog, spec)
	if builder == nil {
		seeds.Diagnostics = append(seeds.Diagnostics, Diagnostic{
			Kind:   "oracle_bridge_spec_missing",
			Reason: "oracle bridge requires an oracle spec",
		})
		return seeds
	}
	started := time.Now()
	starts := oracleBridgeStarts(spec, builder.nodesByID)
	for _, start := range starts {
		if start == nil {
			continue
		}
		seeds.Add(start, FunctionIndexSeedReasonOracleBridge)
		for _, owner := range oraclePackageBridgeOwners(prog, start, started, options, seeds) {
			if options.MaxOwners > 0 && seeds.Len() >= options.MaxOwners {
				appendSeedDiagnosticOnce(seeds, "oracle_bridge_owner_budget_exceeded", "oracle bridge owner budget exceeded")
				return seeds
			}
			seeds.Add(owner, FunctionIndexSeedReasonOracleBridge)
			for _, callee := range oracleBridgeCalleesForStart(owner, start) {
				if options.MaxOwners > 0 && seeds.Len() >= options.MaxOwners {
					appendSeedDiagnosticOnce(seeds, "oracle_bridge_owner_budget_exceeded", "oracle bridge owner budget exceeded")
					return seeds
				}
				seeds.Add(callee, FunctionIndexSeedReasonOracleBridge)
			}
		}
	}
	if targetedExpansionDurationExceeded(started, options.MaxDuration) {
		appendSeedDiagnosticOnce(seeds, "oracle_bridge_duration_exceeded", "oracle bridge duration exceeded")
	}
	return seeds
}

func oracleBridgeStarts(spec OracleSpec, nodes map[string]*ssa.Function) []*ssa.Function {
	var starts []*ssa.Function
	if len(spec.BridgeStarts) == 0 {
		for _, node := range spec.Nodes {
			if fn := nodes[node.ID]; fn != nil {
				starts = append(starts, fn)
			}
		}
		return sortedUniqueFunctions(starts)
	}
	for _, id := range spec.BridgeStarts {
		if fn := nodes[id]; fn != nil {
			starts = append(starts, fn)
		}
	}
	return sortedUniqueFunctions(starts)
}

func oraclePackageBridgeOwners(prog *ssa.Program, start *ssa.Function, started time.Time, options OracleBridgeOptions, seeds *FunctionIndexSeedSet) []*ssa.Function {
	if prog == nil || start == nil {
		return nil
	}
	startPkg := functionPackagePath(start)
	var owners []*ssa.Function
	scanned := 0
	for _, fn := range sortedProgramFunctions(prog) {
		if targetedExpansionDurationExceeded(started, options.MaxDuration) {
			appendSeedDiagnosticOnce(seeds, "oracle_bridge_duration_exceeded", "oracle bridge duration exceeded")
			break
		}
		if functionPackagePath(fn) != startPkg {
			continue
		}
		scanned++
		if options.MaxPackageFunctions > 0 && scanned > options.MaxPackageFunctions {
			appendSeedDiagnosticOnce(seeds, "oracle_bridge_package_budget_exceeded", "oracle bridge package scan budget exceeded")
			break
		}
		evidence := (netHTTPBoundaryPredicate{}).MatchOwner(fn)
		for _, item := range evidence {
			seeds.AddBoundaryEvidence(item)
		}
		if ownerReferencesValue(fn, start) || len(evidence) > 0 {
			owners = append(owners, fn)
		}
	}
	return sortedUniqueFunctions(owners)
}

func ownerReferencesValue(owner *ssa.Function, value ssa.Value) bool {
	if owner == nil || value == nil {
		return false
	}
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			for _, ref := range refsForInstruction(owner, instr) {
				if ref.Operand == value {
					return true
				}
			}
		}
	}
	return false
}

func oracleBridgeCalleesForStart(owner, start *ssa.Function) []*ssa.Function {
	if owner == nil || start == nil {
		return nil
	}
	seen := map[*ssa.Function]bool{}
	for _, block := range owner.Blocks {
		for _, instr := range block.Instrs {
			call, ok := instr.(ssa.CallInstruction)
			if !ok || call.Common() == nil || call.Common().StaticCallee() == nil {
				continue
			}
			for _, arg := range call.Common().Args {
				if arg == start {
					seen[call.Common().StaticCallee()] = true
				}
			}
		}
	}
	return sortedFunctionMapKeys(seen)
}

func oracleTraceForSpec(prog *ssa.Program, spec OracleSpec, touchpoints []RegionTouchpoint, reverseOwners, adjacentOwners, boundaryCandidateOwners []*ssa.Function, seeds *FunctionIndexSeedSet, refIndex FunctionRefIndex, flow functionFlowResult) *OracleTrace {
	builder := newOracleTraceBuilder(prog, spec)
	if builder == nil {
		return nil
	}
	builder.markTouchpoints(touchpoints)
	if reverseOwners != nil {
		builder.markReverseFrontierOwners(reverseOwners)
	}
	if adjacentOwners != nil {
		builder.markAdjacentOwners(adjacentOwners)
	}
	if boundaryCandidateOwners != nil {
		builder.markBoundaryCandidateOwners(boundaryCandidateOwners)
	}
	if seeds != nil {
		builder.markBoundaryEvidence(seeds.BoundaryEvidence)
		builder.markSeedSet(seeds)
	}
	builder.markFunctionRefIndex(refIndex)
	builder.markFinalClassifications(flow)
	return builder.trace()
}
