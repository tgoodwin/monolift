// Candidate admission is the codegen-side feedback loop used by
// admission-aware cut ranking. AdmitCut is cheap, but BuildPlan can load large
// corpus packages, so each candidate gets a bounded planning timeout and a
// small process-local cache keyed by the function identity. The cache avoids
// reloading the same package/method pair when reranking revisits a candidate.
//
// This is refusal-driven reranking, not deepest-admissible exploration. An
// accepted recommendation stays selected even when a deeper candidate might
// also admit; parent-over-leaf research must use a separate opt-in mode.
package codegen

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
	"golang.org/x/tools/go/ssa"
)

const defaultCandidatePlanTimeout = 15 * time.Second
const candidateAdmissionBudget = 60 * time.Second

var (
	candidatePlanTimeout = defaultCandidatePlanTimeout
	buildCandidatePlan   = BuildPlan
	tryAdapterRecovery   = tryAdapterRecoveryFromPlan
)

var retryableAdmissionRefusals = map[string]struct{}{
	"receiver_requires_reconstruction": {},
	"non_serializable_receiver":        {},
	"unsupported_boundary_data":        {},
	"unsupported_result_shape":         {},
	"unsupported_param_shape":          {},
	"callable_boundary_values":         {},
	"missing_reconstructor":            {},
	"adapter_unknown":                  {},
}

// adapterEligibleRefusals are the shape-compatible refusal codes that can
// trigger the boundary-adapter recovery pass (SPRINT-0051 §0.4). On candidate
// refusal with one of these codes and MONOLIFT_BOUNDARY_ADAPTER enabled,
// tryAdapterPass fires before demotion. Phase 5 wires the actual adapter
// planning; Phase 1 reads the flag and marks eligible candidates.
var adapterEligibleRefusals = map[string]struct{}{
	"unsupported_boundary_data": {},
	"unsupported_result_shape":  {},
	"unsupported_param_shape":   {},
	"callable_boundary_values":  {},
	"missing_reconstructor":     {},
}

func admissionAwareRankEnabled() bool {
	return strings.TrimSpace(os.Getenv("MONOLIFT_ADMISSION_AWARE_RANK")) != "0"
}

// boundaryAdapterEnabled returns true when the boundary-adapter recovery
// pass is allowed to fire. Controlled by MONOLIFT_BOUNDARY_ADAPTER:
// absent or "1" enables the pass (default-on locally); "0" disables it.
// Flag-off parity with the SPRINT-0050 baseline is an acceptance criterion.
func boundaryAdapterEnabled() bool {
	v := strings.TrimSpace(os.Getenv("MONOLIFT_BOUNDARY_ADAPTER"))
	return v != "0"
}

func admitCutCandidates(report reportv2.Report, cut *activation.CutResult) (AdmissionVerdict, []CandidateDemotion, error) {
	if cut == nil {
		return refused(AdmissionVerdict{}, "missing_cut", "cut analysis did not produce a cut result", ""), nil, nil
	}
	if cut.Recommended == nil {
		return AdmitCut(report, *cut), nil, nil
	}
	deadline := time.Now().Add(candidateAdmissionBudget)
	maxAttempts := len(cut.Candidates)
	if maxAttempts == 0 {
		maxAttempts = 1
	}

	// Read the boundary-adapter feature flag at loop entry. When disabled,
	// the adapter recovery branch is skipped entirely (flag-off parity with
	// the SPRINT-0050 admission baseline). Phase 5 wires tryAdapterPass
	// behind this gate; Phase 1 establishes the read point.
	adapterEnabled := boundaryAdapterEnabled()

	var last AdmissionVerdict
	var demotionChain []CandidateDemotion
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if time.Now().After(deadline) {
			if len(last.Refusals) == 0 {
				last = AdmissionVerdict{Cut: cut.Recommended}
			}
			return refused(last, "admission_retry_budget_exceeded", fmt.Sprintf("admission-aware ranking exceeded %s wall-clock budget", candidateAdmissionBudget), ""), demotionChain, nil
		}
		if cut.Recommended == nil {
			if len(last.Refusals) > 0 {
				return last, demotionChain, nil
			}
			return AdmitCut(report, *cut), demotionChain, nil
		}
		candidate := *cut.Recommended
		if adapterEnabled && adapterParentForbiddenForCandidate(candidate, cut) {
			refusal := AdmissionRefusal{
				Code:    RefusalAdapterParentForbidden,
				Message: "candidate is an ancestor of a deeper candidate whose AdapterClass is not DirectBoundary; the adapter-shaped leaf must be tried instead of this parent",
			}
			demotionChain = append(demotionChain, CandidateDemotion{
				Step:        candidate.Step,
				NodeKey:     candidate.NodeKey,
				NodeName:    candidate.NodeName,
				RefusalCode: refusal.Code,
				Message:     refusal.Message,
			})
			cut.DemoteCandidate(candidate.Step, candidate.NodeKey, demotionReason(refusal))
			continue
		}
		verdict, plan, err := tryAdmitCandidate(report, candidate)
		if err != nil {
			return verdict, demotionChain, err
		}
		last = verdict
		if verdict.Accepted {
			return verdict, demotionChain, nil
		}
		refusal, ok := retryableRefusal(verdict)
		if !ok {
			return verdict, demotionChain, nil
		}

		if adapterEnabled && isAdapterEligibleRefusal(refusal) {
			if plan == nil {
				built, timedOut, buildErr := buildPlanWithTimeout(report, activation.CutResult{
					Recommended: &candidate,
					Candidates:  []activation.CutCandidate{candidate},
				}, candidatePlanTimeout)
				if buildErr == nil && !timedOut {
					plan = built
				}
			}
			if !adapterRecoveryAllowed(candidate, plan) {
				goto demoteCandidate
			}
			adapterPlan, adapterRefusals := tryAdapterRecovery(report, candidate, plan)
			if adapterPlan != nil && plan != nil {
				candidate.AdapterClass = activation.AdapterPossible
				candidate.AdapterReason = "adapter recovery accepted after direct refusal: " + refusal.Code
				plan.AdapterPlan = adapterPlan
				adapterVerdict := AdmitPlan(normalizedAdapterPlan(plan), AdmissionVerdict{
					Accepted: true,
					Reasons: []string{
						"adapter recovery accepted after direct refusal: " + refusal.Code,
						adapterRecoveryDiagnostic(refusal, adapterPlan),
					},
					Cut: &candidate,
				})
				if adapterVerdict.Accepted {
					// Cache the recovered adapter plan so the build-plan phase
					// reuses it instead of re-running tryAdapterRecovery.
					storeCandidateAdapterPlan(candidateAdmitKey(candidate), adapterPlan)
					markCandidateAdapter(cut, candidate)
					return adapterVerdict, demotionChain, nil
				}
				adapterRefusals = adapterVerdict.Refusals
			}
			candidate.AdapterClass, candidate.AdapterReason = adapterRefusalClass(adapterRefusals)
			markCandidateAdapter(cut, candidate)
		}

	demoteCandidate:
		demotionChain = append(demotionChain, CandidateDemotion{
			Step:        candidate.Step,
			NodeKey:     candidate.NodeKey,
			NodeName:    candidate.NodeName,
			RefusalCode: refusal.Code,
			Message:     refusal.Message,
		})
		cut.DemoteCandidate(candidate.Step, candidate.NodeKey, demotionReason(refusal))
	}
	return last, demotionChain, nil
}

// adapterParentForbiddenForCandidate refuses any candidate that is a strict
// ancestor (lower Step on the same activation path) of a deeper candidate
// whose AdapterClass is not DirectBoundary. The deeper candidate is either
// currently a boundary-adapter consideration (AdapterUnknown,
// AdapterPossible) or was already attempted by the adapter pass and
// classified as LiveProxyRequired or AdapterImpossible. Either way, the
// path has an adapter-shaped leaf and the broader parent must not be
// admitted in its place — the structural property is that the leaf, not
// the parent, is the intended cut even when the leaf refuses today.
//
// The predicate names no function and no type. Adapter eligibility is
// expressed solely via the activation.AdapterClass label, which is
// populated by defaultAdapterClass at cut-build time and refined by the
// adapter pass during admission. New patterns extend the set of
// AdapterUnknown/Possible-labeled candidates automatically.
func adapterParentForbiddenForCandidate(candidate activation.CutCandidate, cut *activation.CutResult) bool {
	if cut == nil {
		return false
	}
	for _, other := range cut.Candidates {
		if other.Step <= candidate.Step {
			continue
		}
		switch other.AdapterClass {
		case "", activation.DirectBoundary:
			continue
		}
		return true
	}
	return false
}

func adapterRecoveryAllowed(candidate activation.CutCandidate, plan *Plan) bool {
	if candidate.NodeKey.Receiver != "" || (plan != nil && plan.CutPoint.Receiver != "") {
		return false
	}
	switch candidate.State {
	case activation.SharedState:
		return false
	}
	if (candidate.Callbacks == activation.Moderate || candidate.Callbacks == activation.Many) && planHasFunctionBoundary(plan) {
		return false
	}
	switch candidate.Surface {
	case "", activation.Minimal, activation.Small:
		return true
	default:
		return false
	}
}

func planHasFunctionBoundary(plan *Plan) bool {
	if plan == nil {
		return true
	}
	for _, param := range plan.BoundaryParams {
		if strings.HasPrefix(strings.TrimSpace(param.GoType), "func(") || strings.HasPrefix(strings.TrimSpace(param.GoType), "func (") {
			return true
		}
	}
	for _, result := range plan.Results {
		if strings.HasPrefix(strings.TrimSpace(result.GoType), "func(") || strings.HasPrefix(strings.TrimSpace(result.GoType), "func (") {
			return true
		}
	}
	return false
}

func markCandidateAdapter(cut *activation.CutResult, candidate activation.CutCandidate) {
	for i := range cut.Candidates {
		if cut.Candidates[i].Step == candidate.Step && cut.Candidates[i].NodeKey == candidate.NodeKey {
			cut.Candidates[i].AdapterClass = candidate.AdapterClass
			cut.Candidates[i].AdapterReason = candidate.AdapterReason
			cut.Recommended = &cut.Candidates[i]
			return
		}
	}
}

func adapterRefusalClass(refusals []AdmissionRefusal) (activation.AdapterClass, string) {
	if len(refusals) == 0 {
		return activation.AdapterUnknown, "adapter recovery failed without a detailed refusal"
	}
	refusal := refusals[0]
	class := activation.AdapterUnknown
	switch refusal.Code {
	case RefusalLiveProxyRequired:
		class = activation.LiveProxyRequired
	case RefusalAdapterImpossible:
		class = activation.AdapterImpossible
	}
	reason := refusal.Code
	if refusal.Message != "" {
		reason += ": " + refusal.Message
	}
	return class, reason
}

func adapterRecoveryDiagnostic(direct AdmissionRefusal, plan *AdapterPlan) string {
	proofs := make([]string, 0, len(plan.Proofs))
	for _, proof := range plan.Proofs {
		status := "refused"
		if proof.Satisfied {
			status = "satisfied"
		}
		proofs = append(proofs, proof.Obligation+"="+status)
	}
	return "AdapterRecovery direct_refusal=" + direct.Code + " adapter_class=" + string(activation.AdapterPossible) + " normalized_boundary=" + plan.RemoteSignature + " proofs=[" + strings.Join(proofs, ",") + "]"
}

func tryAdmitCandidate(report reportv2.Report, candidate activation.CutCandidate) (AdmissionVerdict, *Plan, error) {
	cut := activation.CutResult{
		Recommended: &candidate,
		Candidates:  []activation.CutCandidate{candidate},
	}
	verdict := AdmitCut(report, cut)
	if !verdict.Accepted {
		return verdict, nil, nil
	}
	if preflightVerdict, refused := preflightReceiverAdmission(verdict, candidate); refused {
		return preflightVerdict, nil, nil
	}
	key := candidateAdmitKey(candidate)
	if cached, ok := lookupCandidateAdmitResult(key); ok {
		return cached.verdict, cached.plan, cached.err
	}
	plan, timedOut, err := buildPlanWithTimeout(report, cut, candidatePlanTimeout)
	if timedOut {
		verdict = refused(verdict, "plan_build_timeout", fmt.Sprintf("building codegen plan exceeded %s", candidatePlanTimeout), candidate.NodeKey.String())
		storeCandidateAdmitResult(key, candidateAdmitResult{verdict: verdict})
		return verdict, nil, nil
	}
	if err != nil {
		if buildVerdict, ok := buildPlanAdmissionError(verdict, err); ok {
			storeCandidateAdmitResult(key, candidateAdmitResult{verdict: buildVerdict})
			return buildVerdict, nil, nil
		}
		storeCandidateAdmitResult(key, candidateAdmitResult{verdict: verdict, err: err})
		return verdict, nil, err
	}
	verdict = AdmitPlan(plan, verdict)
	storeCandidateAdmitResult(key, candidateAdmitResult{verdict: verdict, plan: plan})
	return verdict, plan, nil
}

func preflightReceiverAdmission(base AdmissionVerdict, candidate activation.CutCandidate) (AdmissionVerdict, bool) {
	receiver := strings.TrimPrefix(candidate.NodeKey.Receiver, "*")
	if receiver == "" {
		return base, false
	}
	if _, ok := receiverFactoryRegistry[candidate.NodeKey.PackagePath+"."+receiver]; ok {
		return base, false
	}
	if hasKnownReceiverReconstructor(candidate.NodeKey.PackagePath, receiver) {
		return base, false
	}
	switch candidate.State {
	case activation.Stateless, activation.ConfigOnly:
		return base, false
	}
	return refused(
		base,
		"receiver_requires_reconstruction",
		fmt.Sprintf("receiver %s has state class %s", candidate.NodeKey.Receiver, candidate.State),
		candidate.NodeKey.Receiver,
	), true
}

func buildPlanAdmissionError(base AdmissionVerdict, err error) (AdmissionVerdict, bool) {
	if err == nil {
		return base, false
	}
	message := err.Error()
	for code := range retryableAdmissionRefusals {
		if strings.HasPrefix(message, code+":") {
			detail := strings.TrimSpace(strings.TrimPrefix(message, code+":"))
			return refused(base, code, detail, ""), true
		}
	}
	return base, false
}

func retryableRefusal(verdict AdmissionVerdict) (AdmissionRefusal, bool) {
	for _, refusal := range verdict.Refusals {
		if _, ok := retryableAdmissionRefusals[refusal.Code]; ok {
			return refusal, true
		}
	}
	return AdmissionRefusal{}, false
}

// isAdapterEligibleRefusal returns true when the refusal code is one of the
// shape-compatible codes that can trigger boundary-adapter recovery. Broader
// exclusions such as receivers, shared state, broad surfaces, and function
// boundaries are enforced by adapterRecoveryAllowed after plan construction.
//
// missing_reconstructor is special-cased: only a refusal about a boundary
// *parameter* value type is eligible. Receiver and infrastructure-handle
// reconstructor refusals (database, transaction, connection, filesystem,
// file) cannot be adapter-normalized and must not enter the recovery branch.
func isAdapterEligibleRefusal(refusal AdmissionRefusal) bool {
	if _, ok := adapterEligibleRefusals[refusal.Code]; !ok {
		return false
	}
	if refusal.Code == "missing_reconstructor" {
		return isParameterTypeReconstructorRefusal(refusal)
	}
	return true
}

// isParameterTypeReconstructorRefusal reports whether a missing_reconstructor
// refusal concerns a boundary parameter value type an adapter pattern could
// normalize to a wire type — as opposed to an infrastructure handle (database,
// transaction, connection, filesystem, file) or receiver that genuinely needs
// host-side reconstruction and that no adapter pattern can serialize. The
// check reads refusal.Type only and names no target; an empty Type fails
// closed (not eligible).
func isParameterTypeReconstructorRefusal(refusal AdmissionRefusal) bool {
	if refusal.Code != "missing_reconstructor" || refusal.Type == "" {
		return false
	}
	return !isInfrastructureHandleType(refusal.Type)
}

// isInfrastructureHandleType reports whether a Go type names a non-serializable
// infrastructure resource handle that requires host-side reconstruction rather
// than wire normalization. Generic type-string classification (no target
// names), paralleling isSerializableReceiverType's database checks.
func isInfrastructureHandleType(goType string) bool {
	lower := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(goType), "*"))
	switch {
	case strings.Contains(lower, "sql.db"),
		strings.Contains(lower, "sql.tx"),
		strings.Contains(lower, "sql.conn"),
		strings.Contains(lower, "sql.stmt"):
		return true
	case strings.Contains(lower, "filesystem.system"),
		strings.Contains(lower, "afero."),
		strings.Contains(lower, "os.file"),
		strings.Contains(lower, "os.root"):
		return true
	}
	return false
}

func tryAdapterRecoveryFromPlan(report reportv2.Report, candidate activation.CutCandidate, plan *Plan) (*AdapterPlan, []AdmissionRefusal) {
	if plan == nil {
		built, _, err := buildPlanWithTimeout(report, activation.CutResult{
			Recommended: &candidate,
			Candidates:  []activation.CutCandidate{candidate},
		}, candidatePlanTimeout)
		if err != nil {
			return nil, []AdmissionRefusal{{Code: RefusalAdapterUnknown, Message: err.Error()}}
		}
		plan = built
	}
	fn, index, err := loadAdapterSSAWithCallSites(plan)
	if err != nil {
		return nil, []AdmissionRefusal{{Code: RefusalAdapterUnknown, Message: err.Error()}}
	}
	return TryAdapterPass(AdapterContext{
		Fn:                    fn,
		MaxInlinePayloadBytes: defaultInlinePayloadBytes,
		FunctionExported:      isExportedIdentifier(plan.CutPoint.FuncName),
		CallSiteIndex:         index,
	})
}

// adapterSSAKey identifies a cached adapter SSA load. The module root is part
// of the key so synthetic per-test modules (each in its own temp dir) never
// collide on a shared package/func name.
type adapterSSAKey struct {
	moduleRoot  string
	packagePath string
	funcName    string
	receiver    string
}

type adapterSSAEntry struct {
	fn    *ssa.Function
	index *CallSiteIndex
	err   error
}

var adapterSSACache = struct {
	sync.Mutex
	entries map[adapterSSAKey]adapterSSAEntry
}{entries: map[adapterSSAKey]adapterSSAEntry{}}

func resetAdapterSSACache() {
	adapterSSACache.Lock()
	defer adapterSSACache.Unlock()
	adapterSSACache.entries = map[adapterSSAKey]adapterSSAEntry{}
}

// loadAdapterSSAWithCallSites loads the helper SSA function over the
// activation-path scope (the cut package plus its reverse importers) and
// builds the call-site index in the same pass. The result is cached per
// candidate so the build-plan phase does not reload and rescan.
func loadAdapterSSAWithCallSites(plan *Plan) (*ssa.Function, *CallSiteIndex, error) {
	if plan == nil {
		return nil, nil, fmt.Errorf("adapter recovery requires a built plan")
	}
	key := adapterSSAKey{
		moduleRoot:  plan.SourceModuleRoot,
		packagePath: plan.CutPoint.PackagePath,
		funcName:    plan.CutPoint.FuncName,
		receiver:    plan.CutPoint.Receiver,
	}
	adapterSSACache.Lock()
	if entry, ok := adapterSSACache.entries[key]; ok {
		adapterSSACache.Unlock()
		return entry.fn, entry.index, entry.err
	}
	adapterSSACache.Unlock()

	fn, index, err := loadAdapterSSAUncached(plan)
	adapterSSACache.Lock()
	adapterSSACache.entries[key] = adapterSSAEntry{fn: fn, index: index, err: err}
	adapterSSACache.Unlock()
	return fn, index, err
}

func loadAdapterSSAUncached(plan *Plan) (*ssa.Function, *CallSiteIndex, error) {
	// Scope to the reverse-import set so the call-site scan observes callers
	// in importing packages, not just the cut package. Fall back to the cut
	// package alone if scoping fails.
	packages := []string{plan.CutPoint.PackagePath}
	if plan.CutPoint.File != "" {
		if scoped, err := activation.ReverseImportScope(plan.SourceModuleRoot, plan.CutPoint.File, nil); err == nil && len(scoped) > 0 {
			packages = scoped
		}
	}
	cfg := activation.Config{
		Dir:      plan.SourceModuleRoot,
		Packages: packages,
	}
	program, err := cfg.LoadProgram()
	if err != nil {
		return nil, nil, fmt.Errorf("load SSA for adapter recovery: %w", err)
	}
	program.BuildSSA()
	for _, pkg := range program.SSAProgram.AllPackages() {
		if pkg == nil || pkg.Pkg == nil || pkg.Pkg.Path() != plan.CutPoint.PackagePath {
			continue
		}
		for _, member := range pkg.Members {
			fn, ok := member.(*ssa.Function)
			if ok && fn.Name() == plan.CutPoint.FuncName {
				return fn, buildCallSiteIndex(fn), nil
			}
		}
	}
	return nil, nil, fmt.Errorf("adapter recovery could not find SSA function %s in %s", plan.CutPoint.FuncName, plan.CutPoint.PackagePath)
}

func isExportedIdentifier(name string) bool {
	if name == "" {
		return false
	}
	return strings.ToUpper(name[:1]) == name[:1]
}

func demotionReason(refusal AdmissionRefusal) string {
	reason := refusal.Code
	if refusal.Message != "" {
		reason += ": " + refusal.Message
	}
	if refusal.Type != "" {
		reason += " (" + refusal.Type + ")"
	}
	return reason
}

type candidateAdmitCacheKey struct {
	packagePath  string
	funcName     string
	receiverType string
}

type candidateAdmitResult struct {
	verdict AdmissionVerdict
	plan    *Plan
	err     error
	// adapterPlan is the recovery result, set after admitCutCandidates accepts
	// an adapter-recovered candidate. The build-plan phase reuses it so
	// tryAdapterRecovery runs exactly once per recovered candidate.
	adapterPlan *AdapterPlan
}

var candidateAdmitCache = struct {
	sync.Mutex
	results map[candidateAdmitCacheKey]candidateAdmitResult
}{
	results: map[candidateAdmitCacheKey]candidateAdmitResult{},
}

func candidateAdmitKey(candidate activation.CutCandidate) candidateAdmitCacheKey {
	return candidateAdmitCacheKey{
		packagePath:  candidate.NodeKey.PackagePath,
		funcName:     candidate.NodeKey.FuncName,
		receiverType: candidate.NodeKey.Receiver,
	}
}

func lookupCandidateAdmitResult(key candidateAdmitCacheKey) (candidateAdmitResult, bool) {
	candidateAdmitCache.Lock()
	defer candidateAdmitCache.Unlock()
	result, ok := candidateAdmitCache.results[key]
	return result, ok
}

func storeCandidateAdmitResult(key candidateAdmitCacheKey, result candidateAdmitResult) {
	candidateAdmitCache.Lock()
	defer candidateAdmitCache.Unlock()
	candidateAdmitCache.results[key] = result
}

// storeCandidateAdapterPlan attaches a recovered adapter plan to the cached
// admit result for key, preserving the existing verdict/plan. The entry is
// created if absent.
func storeCandidateAdapterPlan(key candidateAdmitCacheKey, adapterPlan *AdapterPlan) {
	candidateAdmitCache.Lock()
	defer candidateAdmitCache.Unlock()
	entry := candidateAdmitCache.results[key]
	entry.adapterPlan = adapterPlan
	candidateAdmitCache.results[key] = entry
}

// cachedAdapterPlanFor returns the adapter plan recovered for candidate during
// admission, or nil when none is cached. The lookup enforces an invariant: the
// cached plan's SourceFunction must match the candidate's function name (the
// FunctionKey identity), guarding against a stale or mis-keyed entry. On
// mismatch it returns nil so the caller recomputes rather than trusting a wrong
// plan.
func cachedAdapterPlanFor(candidate activation.CutCandidate) *AdapterPlan {
	cached, ok := lookupCandidateAdmitResult(candidateAdmitKey(candidate))
	if !ok || cached.adapterPlan == nil {
		return nil
	}
	if cached.adapterPlan.SourceFunction != candidate.NodeKey.FuncName {
		return nil
	}
	return cached.adapterPlan
}

type candidatePlanResult struct {
	plan *Plan
	err  error
}

func buildPlanWithTimeout(report reportv2.Report, cut activation.CutResult, timeout time.Duration) (*Plan, bool, error) {
	results := make(chan candidatePlanResult, 1)
	builder := buildCandidatePlan
	go func() {
		plan, err := builder(report, cut)
		results <- candidatePlanResult{plan: plan, err: err}
	}()
	if timeout <= 0 {
		result := <-results
		return result.plan, false, result.err
	}
	select {
	case result := <-results:
		return result.plan, false, result.err
	case <-time.After(timeout):
		return nil, true, nil
	}
}
