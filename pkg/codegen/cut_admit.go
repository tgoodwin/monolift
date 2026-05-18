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
)

const defaultCandidatePlanTimeout = 15 * time.Second
const candidateAdmissionBudget = 60 * time.Second

var (
	candidatePlanTimeout = defaultCandidatePlanTimeout
	buildCandidatePlan   = BuildPlan
)

var retryableAdmissionRefusals = map[string]struct{}{
	"receiver_requires_reconstruction": {},
	"non_serializable_receiver":        {},
	"unsupported_result_shape":         {},
	"missing_reconstructor":            {},
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
		verdict, _, err := tryAdmitCandidate(report, candidate)
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

		// Boundary-adapter recovery branch (SPRINT-0051 §0.4).
		// When enabled and the refusal is shape-compatible, the adapter
		// pass attempts to normalize the boundary before demotion.
		// Phase 5 wires tryAdapterPass here; Phase 1 marks eligibility
		// and falls through to demotion.
		if adapterEnabled && isAdapterEligibleRefusal(refusal) {
			// Mark the candidate as adapter-eligible. Phase 5 will wire
			// tryAdapterPass here; on success it attaches an AdapterPlan
			// and returns an accepted verdict. For now (Phase 1), we mark
			// the candidate and fall through to demotion so the branch is
			// observable and the flag gate is not a no-op.
			candidate.AdapterClass = activation.AdapterUnknown
			candidate.AdapterReason = "adapter-eligible refusal (" + refusal.Code + "); adapter pass not yet wired (Phase 5)"
			// Update the candidate in the cut's candidate list so the
			// marking is visible to downstream consumers.
			for i := range cut.Candidates {
				if cut.Candidates[i].Step == candidate.Step && cut.Candidates[i].NodeKey == candidate.NodeKey {
					cut.Candidates[i].AdapterClass = candidate.AdapterClass
					cut.Candidates[i].AdapterReason = candidate.AdapterReason
					break
				}
			}
		}

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
// shape-compatible codes that can trigger boundary-adapter recovery per
// SPRINT-0051 §0.4. Receiver reconstruction failures, shared-state receivers,
// and infrastructure-handle reconstructor failures are explicitly excluded.
func isAdapterEligibleRefusal(refusal AdmissionRefusal) bool {
	_, ok := adapterEligibleRefusals[refusal.Code]
	return ok
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
