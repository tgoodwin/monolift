package codegen

import (
	"testing"
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/compiler/reportv2"
)

func TestTryAdmitCandidateAcceptsCleanCandidate(t *testing.T) {
	candidate := admissibleTestCandidate()
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		if cut.Recommended == nil {
			t.Fatal("BuildPlan cut has no recommended candidate")
		}
		if cut.Recommended.NodeKey != candidate.NodeKey {
			t.Fatalf("BuildPlan candidate = %+v, want %+v", cut.Recommended.NodeKey, candidate.NodeKey)
		}
		return &Plan{
			CutPoint: CutPoint{Key: candidate.NodeKey},
			Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})

	verdict, plan, err := tryAdmitCandidate(reportv2.Report{}, candidate)
	if err != nil {
		t.Fatalf("tryAdmitCandidate returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("tryAdmitCandidate refused clean candidate: %s", verdict.Error())
	}
	if plan == nil {
		t.Fatal("tryAdmitCandidate returned nil plan")
	}
}

func TestTryAdmitCandidateReturnsAdmitCutRefusal(t *testing.T) {
	candidate := admissibleTestCandidate()
	candidate.BoundaryData = activation.ProxyRequired
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		t.Fatal("BuildPlan should not run when AdmitCut refuses")
		return nil, nil
	})

	verdict, plan, err := tryAdmitCandidate(reportv2.Report{}, candidate)
	if err != nil {
		t.Fatalf("tryAdmitCandidate returned error: %v", err)
	}
	if verdict.Accepted {
		t.Fatal("tryAdmitCandidate accepted candidate refused by AdmitCut")
	}
	if !hasRefusal(verdict, "unsupported_boundary_data") {
		t.Fatalf("refusals = %+v, want unsupported_boundary_data", verdict.Refusals)
	}
	if plan != nil {
		t.Fatalf("plan = %+v, want nil", plan)
	}
}

func TestTryAdmitCandidateReturnsAdmitPlanRefusal(t *testing.T) {
	candidate := admissibleTestCandidate()
	candidate.NodeKey.Receiver = "*NeedsState"
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return &Plan{
			CutPoint: CutPoint{
				Key:      candidate.NodeKey,
				Receiver: "*NeedsState",
			},
			Results: []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})

	verdict, plan, err := tryAdmitCandidate(reportv2.Report{}, candidate)
	if err != nil {
		t.Fatalf("tryAdmitCandidate returned error: %v", err)
	}
	if verdict.Accepted {
		t.Fatal("tryAdmitCandidate accepted candidate refused by AdmitPlan")
	}
	if !hasRefusal(verdict, "receiver_requires_reconstruction") {
		t.Fatalf("refusals = %+v, want receiver_requires_reconstruction", verdict.Refusals)
	}
	if plan == nil {
		t.Fatal("plan = nil, want plan returned with AdmitPlan refusal")
	}
}

func TestTryAdmitCandidateReturnsPlanBuildTimeout(t *testing.T) {
	candidate := admissibleTestCandidate()
	unblock := make(chan struct{})
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		<-unblock
		return &Plan{Results: []Result{{GoType: "string", Codec: CodecPrimitive}}}, nil
	})
	t.Cleanup(func() { close(unblock) })
	candidatePlanTimeout = time.Nanosecond

	verdict, plan, err := tryAdmitCandidate(reportv2.Report{}, candidate)
	if err != nil {
		t.Fatalf("tryAdmitCandidate returned error: %v", err)
	}
	if verdict.Accepted {
		t.Fatal("tryAdmitCandidate accepted candidate whose BuildPlan timed out")
	}
	if !hasRefusal(verdict, "plan_build_timeout") {
		t.Fatalf("refusals = %+v, want plan_build_timeout", verdict.Refusals)
	}
	if plan != nil {
		t.Fatalf("plan = %+v, want nil", plan)
	}
}

func TestAdmitCutCandidatesDemotesRefusedCandidateToCleanCandidate(t *testing.T) {
	refusedCandidate := admissibleTestCandidate()
	refusedCandidate.Step = 2
	refusedCandidate.NodeKey.FuncName = "NeedsReceiver"
	refusedCandidate.NodeName = "NeedsReceiver"
	cleanCandidate := admissibleTestCandidate()
	cleanCandidate.Step = 3
	cleanCandidate.NodeKey.FuncName = "Leaf"
	cleanCandidate.NodeName = "Leaf"
	cleanCandidate.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{refusedCandidate, cleanCandidate},
	}
	cut.Recommended = &cut.Candidates[0]
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "NeedsReceiver":
			return &Plan{
				CutPoint: CutPoint{
					Key:      cut.Recommended.NodeKey,
					Receiver: "*NeedsState",
				},
				Results: []Result{{GoType: "string", Codec: CodecPrimitive}},
			}, nil
		case "Leaf":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
			}, nil
		default:
			t.Fatalf("unexpected candidate %s", cut.Recommended.NodeKey.FuncName)
			return nil, nil
		}
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused after demotion: %s", verdict.Error())
	}
	if cut.Recommended == nil || cut.Recommended.NodeName != "Leaf" {
		t.Fatalf("Recommended = %+v, want Leaf", cut.Recommended)
	}
	if got, want := len(chain), 1; got != want {
		t.Fatalf("demotion chain length = %d, want %d", got, want)
	}
	if chain[0].RefusalCode != "receiver_requires_reconstruction" {
		t.Fatalf("demotion refusal code = %q, want receiver_requires_reconstruction", chain[0].RefusalCode)
	}
}

func TestAdmitCutCandidatesPreservesAcceptedParentOverAdmissibleLeaf(t *testing.T) {
	parent := admissibleTestCandidate()
	parent.Step = 2
	parent.NodeKey.FuncName = "RefreshFeed"
	parent.NodeName = "RefreshFeed"
	parent.Surface = activation.Small
	leaf := admissibleTestCandidate()
	leaf.Step = 3
	leaf.NodeKey.FuncName = "UpdateOrCreateFeedIcon"
	leaf.NodeName = "UpdateOrCreateFeedIcon"
	leaf.Surface = activation.Minimal
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{parent, leaf},
	}
	cut.Recommended = &cut.Candidates[0]

	var builds []string
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		builds = append(builds, cut.Recommended.NodeKey.FuncName)
		if cut.Recommended.NodeKey.FuncName != "RefreshFeed" {
			t.Fatalf("BuildPlan candidate = %s, want RefreshFeed only", cut.Recommended.NodeKey.FuncName)
		}
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused accepted parent: %s", verdict.Error())
	}
	if cut.Recommended == nil || cut.Recommended.NodeName != "RefreshFeed" {
		t.Fatalf("Recommended = %+v, want RefreshFeed", cut.Recommended)
	}
	if len(chain) != 0 {
		t.Fatalf("demotion chain = %+v, want none", chain)
	}
	if got, want := builds, []string{"RefreshFeed"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("BuildPlan calls = %+v, want %+v", got, want)
	}
}

func TestAdmitCutCandidatesReturnsFinalRefusalWhenAllCandidatesRefused(t *testing.T) {
	first := admissibleTestCandidate()
	first.Step = 2
	first.NodeKey.FuncName = "First"
	first.NodeName = "First"
	second := admissibleTestCandidate()
	second.Step = 3
	second.NodeKey.FuncName = "Second"
	second.NodeName = "Second"
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{first, second},
	}
	cut.Recommended = &cut.Candidates[0]
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return &Plan{
			CutPoint: CutPoint{
				Key:      cut.Recommended.NodeKey,
				Receiver: "*NeedsState",
			},
			Results: []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if verdict.Accepted {
		t.Fatal("admitCutCandidates accepted when every candidate was refused")
	}
	if !hasRefusal(verdict, "receiver_requires_reconstruction") {
		t.Fatalf("refusals = %+v, want receiver_requires_reconstruction", verdict.Refusals)
	}
	if got, want := len(chain), 2; got != want {
		t.Fatalf("demotion chain length = %d, want %d", got, want)
	}
	if chain[0].NodeKey.FuncName != "First" || chain[1].NodeKey.FuncName != "Second" {
		t.Fatalf("demotion chain = %+v, want First then Second", chain)
	}
}

func TestAdmissionAwareRankingPreservesSprint0048HandPickedRecommendations(t *testing.T) {
	cases := []struct {
		target string
		key    activation.FunctionKey
	}{
		{
			target: "activation-caddy-cleanpath",
			key:    activation.FunctionKey{PackagePath: "github.com/caddyserver/caddy/v2/modules/caddyhttp", FuncName: "CleanPath"},
		},
		{
			target: "activation-gitea-pathescapesegments",
			key:    activation.FunctionKey{PackagePath: "code.gitea.io/gitea/modules/util", FuncName: "PathEscapeSegments"},
		},
		{
			target: "activation-listmonk-sanitizeuri",
			key:    activation.FunctionKey{PackagePath: "github.com/knadh/listmonk/internal/utils", FuncName: "SanitizeURI"},
		},
		{
			target: "activation-mattermost-publiclinkhash",
			key:    activation.FunctionKey{PackagePath: "github.com/mattermost/mattermost/server/v8/channels/app", FuncName: "GeneratePublicLinkHash"},
		},
		{
			target: "activation-miniflux-sanitizehtml",
			key:    activation.FunctionKey{PackagePath: "miniflux.app/v2/internal/reader/sanitizer", FuncName: "SanitizeHTML"},
		},
		{
			target: "activation-miniflux-striptags",
			key:    activation.FunctionKey{PackagePath: "miniflux.app/v2/internal/reader/sanitizer", FuncName: "StripTags"},
		},
		{
			target: "activation-pocketbase-columnify",
			key:    activation.FunctionKey{PackagePath: "github.com/pocketbase/pocketbase/tools/inflector", FuncName: "Columnify"},
		},
	}

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			candidate := admissibleTestCandidate()
			candidate.NodeKey = tc.key
			candidate.NodeName = tc.key.FuncName
			cut := &activation.CutResult{Candidates: []activation.CutCandidate{candidate}}
			cut.Recommended = &cut.Candidates[0]

			verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
			if err != nil {
				t.Fatalf("admitCutCandidates returned error: %v", err)
			}
			if !verdict.Accepted {
				t.Fatalf("admitCutCandidates refused cached recommendation: %s", verdict.Error())
			}
			if len(chain) != 0 {
				t.Fatalf("demotion chain = %+v, want none", chain)
			}
			if cut.Recommended == nil || cut.Recommended.NodeKey != tc.key {
				t.Fatalf("Recommended = %+v, want %+v", cut.Recommended, tc.key)
			}
		})
	}
}

func TestBoundaryAdapterEnabledReadsEnvVar(t *testing.T) {
	// Default (unset): enabled.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "")
	if !boundaryAdapterEnabled() {
		t.Fatal("boundaryAdapterEnabled() = false with empty env, want true")
	}

	// Explicit "1": enabled.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")
	if !boundaryAdapterEnabled() {
		t.Fatal("boundaryAdapterEnabled() = false with env=1, want true")
	}

	// Explicit "0": disabled.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "0")
	if boundaryAdapterEnabled() {
		t.Fatal("boundaryAdapterEnabled() = true with env=0, want false")
	}

	// Whitespace around "0": disabled.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", " 0 ")
	if boundaryAdapterEnabled() {
		t.Fatal("boundaryAdapterEnabled() = true with env=' 0 ', want false")
	}
}

func TestIsAdapterEligibleRefusal(t *testing.T) {
	eligible := []string{"unsupported_boundary_data", "unsupported_result_shape", "unsupported_param_shape"}
	for _, code := range eligible {
		if !isAdapterEligibleRefusal(AdmissionRefusal{Code: code}) {
			t.Errorf("isAdapterEligibleRefusal(%q) = false, want true", code)
		}
	}

	ineligible := []string{
		"receiver_requires_reconstruction",
		"non_serializable_receiver",
		"missing_reconstructor",
		"plan_build_timeout",
		"streaming_type",
	}
	for _, code := range ineligible {
		if isAdapterEligibleRefusal(AdmissionRefusal{Code: code}) {
			t.Errorf("isAdapterEligibleRefusal(%q) = true, want false", code)
		}
	}
}

func TestAdmitCutCandidatesFlagOffParitySkipsAdapterBranch(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=0, the admission loop should behave
	// identically to the SPRINT-0050 baseline — demotion proceeds without
	// any adapter recovery attempt. This test uses a candidate that would
	// produce an adapter-eligible refusal (unsupported_result_shape) and
	// confirms it still demotes to the next candidate.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "0")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "ProcessImage"
	awkward.NodeName = "ProcessImage"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "UploadMedia"
	clean.NodeName = "UploadMedia"
	clean.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{awkward, clean},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "ProcessImage":
			// Return a plan with >2 results to trigger unsupported_result_shape.
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "*bytes.Reader", Codec: CodecJSON},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "error", Codec: CodecError},
				},
			}, nil
		case "UploadMedia":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
			}, nil
		default:
			t.Fatalf("unexpected candidate %s", cut.Recommended.NodeKey.FuncName)
			return nil, nil
		}
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused after demotion: %s", verdict.Error())
	}
	// With flag off, ProcessImage should be demoted and UploadMedia selected.
	if cut.Recommended == nil || cut.Recommended.NodeName != "UploadMedia" {
		t.Fatalf("Recommended = %+v, want UploadMedia (flag-off parity)", cut.Recommended)
	}
	if len(chain) != 1 {
		t.Fatalf("demotion chain length = %d, want 1", len(chain))
	}
	if chain[0].RefusalCode != "unsupported_result_shape" {
		t.Fatalf("demotion refusal code = %q, want unsupported_result_shape", chain[0].RefusalCode)
	}
}

func TestAdmitCutCandidatesFlagOnMarksAdapterEligibility(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=1 (or unset), the admission loop
	// should mark adapter-eligible candidates with AdapterUnknown before
	// demoting them. This proves the flag gate is not a no-op: the
	// candidate's AdapterClass is updated when the branch fires.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "ProcessImage"
	awkward.NodeName = "ProcessImage"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "UploadMedia"
	clean.NodeName = "UploadMedia"
	clean.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{awkward, clean},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "ProcessImage":
			// Return a plan with >2 results to trigger unsupported_result_shape.
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "*bytes.Reader", Codec: CodecJSON},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "error", Codec: CodecError},
				},
			}, nil
		case "UploadMedia":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
			}, nil
		default:
			t.Fatalf("unexpected candidate %s", cut.Recommended.NodeKey.FuncName)
			return nil, nil
		}
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused after demotion: %s", verdict.Error())
	}
	// With flag on, ProcessImage should still be demoted (Phase 5 hasn't
	// wired tryAdapterPass yet), but the candidate should be marked with
	// AdapterUnknown to prove the branch fired.
	if cut.Recommended == nil || cut.Recommended.NodeName != "UploadMedia" {
		t.Fatalf("Recommended = %+v, want UploadMedia", cut.Recommended)
	}
	if len(chain) != 1 {
		t.Fatalf("demotion chain length = %d, want 1", len(chain))
	}
	// Verify the demoted candidate was marked by the adapter branch.
	var demoted *activation.CutCandidate
	for i := range cut.Candidates {
		if cut.Candidates[i].NodeKey.FuncName == "ProcessImage" {
			demoted = &cut.Candidates[i]
			break
		}
	}
	if demoted == nil {
		t.Fatal("ProcessImage candidate not found in cut.Candidates")
	}
	if demoted.AdapterClass != activation.AdapterUnknown {
		t.Fatalf("demoted candidate AdapterClass = %s, want %s", demoted.AdapterClass, activation.AdapterUnknown)
	}
	if demoted.AdapterReason == "" {
		t.Fatal("demoted candidate AdapterReason is empty, want non-empty reason from adapter branch")
	}
}

func TestAdmitCutCandidatesFlagOffDoesNotMarkAdapterEligibility(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=0, the adapter branch should NOT fire,
	// so the candidate's AdapterClass should remain at its default value.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "0")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "ProcessImage"
	awkward.NodeName = "ProcessImage"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "UploadMedia"
	clean.NodeName = "UploadMedia"
	clean.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{awkward, clean},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "ProcessImage":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "*bytes.Reader", Codec: CodecJSON},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "error", Codec: CodecError},
				},
			}, nil
		case "UploadMedia":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
			}, nil
		default:
			t.Fatalf("unexpected candidate %s", cut.Recommended.NodeKey.FuncName)
			return nil, nil
		}
	})

	verdict, _, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused after demotion: %s", verdict.Error())
	}
	// Verify the demoted candidate was NOT marked by the adapter branch.
	var demoted *activation.CutCandidate
	for i := range cut.Candidates {
		if cut.Candidates[i].NodeKey.FuncName == "ProcessImage" {
			demoted = &cut.Candidates[i]
			break
		}
	}
	if demoted == nil {
		t.Fatal("ProcessImage candidate not found in cut.Candidates")
	}
	// With flag off, AdapterClass should remain at its initial value (empty
	// string, since admissibleTestCandidate doesn't set it).
	if demoted.AdapterClass == activation.AdapterUnknown {
		t.Fatalf("demoted candidate AdapterClass = %s with flag off; adapter branch should not have fired", demoted.AdapterClass)
	}
	if demoted.AdapterReason != "" {
		t.Fatalf("demoted candidate AdapterReason = %q with flag off; should be empty", demoted.AdapterReason)
	}
}

func admissibleTestCandidate() activation.CutCandidate {
	return activation.CutCandidate{
		Step:         1,
		NodeKey:      activation.FunctionKey{PackagePath: "example.com/app/pkg", FuncName: "Handle"},
		NodeName:     "Handle",
		Feasibility:  activation.Feasible,
		BoundaryData: activation.Serializable,
		Callbacks:    activation.ZeroConfirmed,
		State:        activation.Stateless,
		Surface:      activation.Minimal,
	}
}

func hasRefusal(verdict AdmissionVerdict, code string) bool {
	for _, refusal := range verdict.Refusals {
		if refusal.Code == code {
			return true
		}
	}
	return false
}

func withCandidatePlanBuilder(t *testing.T, build func(reportv2.Report, activation.CutResult) (*Plan, error)) {
	t.Helper()
	resetCandidateAdmitCacheForTest()
	oldBuilder := buildCandidatePlan
	oldTimeout := candidatePlanTimeout
	buildCandidatePlan = build
	t.Cleanup(func() {
		buildCandidatePlan = oldBuilder
		candidatePlanTimeout = oldTimeout
		resetCandidateAdmitCacheForTest()
	})
}

func resetCandidateAdmitCacheForTest() {
	candidateAdmitCache.Lock()
	defer candidateAdmitCache.Unlock()
	candidateAdmitCache.results = map[candidateAdmitCacheKey]candidateAdmitResult{}
}
