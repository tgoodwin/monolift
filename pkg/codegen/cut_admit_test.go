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
	// Shape-compatible codes are eligible regardless of Type.
	eligible := []string{
		"unsupported_boundary_data",
		"unsupported_result_shape",
		"unsupported_param_shape",
		"callable_boundary_values",
	}
	for _, code := range eligible {
		if !isAdapterEligibleRefusal(AdmissionRefusal{Code: code}) {
			t.Errorf("isAdapterEligibleRefusal(%q) = false, want true", code)
		}
	}

	ineligible := []string{
		"receiver_requires_reconstruction",
		"non_serializable_receiver",
		"plan_build_timeout",
		"streaming_type",
	}
	for _, code := range ineligible {
		if isAdapterEligibleRefusal(AdmissionRefusal{Code: code}) {
			t.Errorf("isAdapterEligibleRefusal(%q) = true, want false", code)
		}
	}
}

// SPRINT-0052 task 2.3 (flag B-11): missing_reconstructor is adapter-eligible
// only for a boundary parameter value type. Receiver / infrastructure-handle
// reconstructor refusals (e.g. *sql.DB, filesystem) must not enter the
// recovery branch. An empty Type fails closed.
func TestMissingReconstructorAdapterEligibilityByType(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		want bool
	}{
		{"parameter value type", "*multipart.FileHeader", true},
		{"bytes reader param", "*bytes.Reader", true},
		{"sql.DB handle", "*sql.DB", false},
		{"sql.Tx handle", "*sql.Tx", false},
		{"filesystem handle", "filesystem.System", false},
		{"os.File handle", "*os.File", false},
		{"empty type fails closed", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			refusal := AdmissionRefusal{Code: "missing_reconstructor", Type: tc.typ}
			if got := isAdapterEligibleRefusal(refusal); got != tc.want {
				t.Fatalf("isAdapterEligibleRefusal(missing_reconstructor, %q) = %v, want %v", tc.typ, got, tc.want)
			}
			if got := isParameterTypeReconstructorRefusal(refusal); got != tc.want {
				t.Fatalf("isParameterTypeReconstructorRefusal(%q) = %v, want %v", tc.typ, got, tc.want)
			}
		})
	}
}

func TestAdmitCutCandidatesFlagOffParitySkipsAdapterBranch(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=0, the admission loop should behave
	// identically to the SPRINT-0050 baseline — demotion proceeds without
	// any adapter recovery attempt. This test uses a candidate with a
	// non-DTO-normalizable multi-return (chan int is not JSON-codable) to
	// trigger unsupported_result_shape, which is both retryable and
	// adapter-eligible.
	// Note: (*bytes.Reader, int, int, error) is now admitted via DTO
	// normalization (SPRINT-0051 Phase 2).
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "0")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "StreamResults"
	awkward.NodeName = "StreamResults"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "CleanLeaf"
	clean.NodeName = "CleanLeaf"
	clean.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{awkward, clean},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "StreamResults":
			// 3 results with non-JSON-codable func type → DTO fails →
			// unsupported_result_shape refusal. func() error is not caught
			// by the per-result streaming_type check but fails the DTO
			// JSON-codability check.
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "func() error", Codec: CodecJSON},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "error", Codec: CodecError},
				},
			}, nil
		case "CleanLeaf":
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
	// With flag off, StreamResults should be demoted and the next clean candidate selected.
	if cut.Recommended == nil || cut.Recommended.NodeName != "CleanLeaf" {
		t.Fatalf("Recommended = %+v, want CleanLeaf (flag-off parity)", cut.Recommended)
	}
	if len(chain) != 1 {
		t.Fatalf("demotion chain length = %d, want 1", len(chain))
	}
	if chain[0].RefusalCode != "unsupported_result_shape" {
		t.Fatalf("demotion refusal code = %q, want unsupported_result_shape", chain[0].RefusalCode)
	}
}

// TestAdmitCutCandidatesDTONormalizationAcceptsProcessImage verifies that
// the DTO normalization (SPRINT-0051 Phase 2) accepts processImage's
// (*bytes.Reader, int, int, error) shape directly, without demotion.
func TestAdmitCutCandidatesDTONormalizationAcceptsProcessImage(t *testing.T) {
	processImage := admissibleTestCandidate()
	processImage.Step = 2
	processImage.NodeKey.FuncName = "ProcessImage"
	processImage.NodeName = "ProcessImage"
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{processImage},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			Results: []Result{
				{Name: "reader", GoType: "*bytes.Reader", QualifiedGoType: "*bytes.Reader", Codec: CodecJSON, Index: 0},
				{Name: "width", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 1},
				{Name: "height", GoType: "int", QualifiedGoType: "int", Codec: CodecPrimitive, Index: 2},
				{Name: "err", GoType: "error", QualifiedGoType: "error", Codec: CodecError, Index: 3},
			},
		}, nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("admitCutCandidates refused ProcessImage with DTO normalization: %s", verdict.Error())
	}
	if cut.Recommended == nil || cut.Recommended.NodeName != "ProcessImage" {
		t.Fatalf("Recommended = %+v, want ProcessImage", cut.Recommended)
	}
	if len(chain) != 0 {
		t.Fatalf("demotion chain = %+v, want empty (ProcessImage should be accepted)", chain)
	}
}

func TestAdmitCutCandidatesFlagOnMarksAdapterEligibility(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=1, the admission loop marks
	// adapter-eligible candidates with AdapterUnknown before demoting them.
	// This uses a non-DTO-normalizable multi-return (func() error is not
	// JSON-codable) to trigger unsupported_result_shape, which is both
	// retryable and adapter-eligible.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "ProcessStream"
	awkward.NodeName = "ProcessStream"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "CleanLeaf"
	clean.NodeName = "CleanLeaf"
	clean.Surface = activation.Small
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{awkward, clean},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "ProcessStream":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "func() error", Codec: CodecJSON},
					{GoType: "int", Codec: CodecPrimitive},
					{GoType: "error", Codec: CodecError},
				},
			}, nil
		case "CleanLeaf":
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
	if cut.Recommended == nil || cut.Recommended.NodeName != "CleanLeaf" {
		t.Fatalf("Recommended = %+v, want CleanLeaf", cut.Recommended)
	}
	if len(chain) != 1 {
		t.Fatalf("demotion chain length = %d, want 1", len(chain))
	}
	// Verify the demoted candidate was marked by the adapter branch.
	var demoted *activation.CutCandidate
	for i := range cut.Candidates {
		if cut.Candidates[i].NodeKey.FuncName == "ProcessStream" {
			demoted = &cut.Candidates[i]
			break
		}
	}
	if demoted == nil {
		t.Fatal("ProcessStream candidate not found in cut.Candidates")
	}
	if demoted.AdapterClass != activation.AdapterUnknown {
		t.Fatalf("demoted candidate AdapterClass = %s, want %s", demoted.AdapterClass, activation.AdapterUnknown)
	}
	if demoted.AdapterReason == "" {
		t.Fatal("demoted candidate AdapterReason is empty, want non-empty reason from adapter branch")
	}
}

func TestAdmitCutCandidatesAdapterRecoverySelectsProcessImageNotUploadMedia(t *testing.T) {
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")
	processImage := admissibleTestCandidate()
	processImage.Step = 2
	processImage.NodeKey.FuncName = "processImage"
	processImage.NodeName = "processImage"
	uploadMedia := admissibleTestCandidate()
	uploadMedia.Step = 3
	uploadMedia.NodeKey.Receiver = "*App"
	uploadMedia.NodeKey.FuncName = "UploadMedia"
	uploadMedia.NodeName = "(*App).UploadMedia"
	uploadMedia.Surface = activation.Small
	cut := &activation.CutResult{Candidates: []activation.CutCandidate{processImage, uploadMedia}}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		switch cut.Recommended.NodeKey.FuncName {
		case "processImage":
			return processImageRecoveryPlan(cut.Recommended.NodeKey), nil
		case "UploadMedia":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey, Receiver: "*App"},
				Results:  []Result{{GoType: "error", Codec: CodecError}},
			}, nil
		default:
			t.Fatalf("unexpected candidate %s", cut.Recommended.NodeKey.FuncName)
			return nil, nil
		}
	})
	withAdapterRecovery(t, func(report reportv2.Report, candidate activation.CutCandidate, plan *Plan) (*AdapterPlan, []AdmissionRefusal) {
		if candidate.NodeKey.FuncName != "processImage" {
			t.Fatalf("adapter recovery called for %s, want processImage only", candidate.NodeKey.FuncName)
		}
		return processImageRecoveryAdapterPlan(), nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("adapter recovery refused processImage: %s", verdict.Error())
	}
	if cut.Recommended == nil || cut.Recommended.NodeName != "processImage" {
		t.Fatalf("Recommended = %+v, want processImage", cut.Recommended)
	}
	if cut.Recommended.AdapterClass != activation.AdapterPossible {
		t.Fatalf("AdapterClass = %s, want %s", cut.Recommended.AdapterClass, activation.AdapterPossible)
	}
	if len(chain) != 0 {
		t.Fatalf("demotion chain = %+v, want none before UploadMedia", chain)
	}
}

// TestAdmitCutCandidatesCachesAdapterPlanForReuse covers SPRINT-0052 task 2.7
// (B-15): admission runs tryAdapterRecovery exactly once and caches the
// resulting AdapterPlan, and the build-plan phase reuses it via
// cachedAdapterPlanFor without re-running recovery.
func TestAdmitCutCandidatesCachesAdapterPlanForReuse(t *testing.T) {
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")
	processImage := admissibleTestCandidate()
	processImage.Step = 2
	processImage.NodeKey.FuncName = "processImage"
	processImage.NodeName = "processImage"
	cut := &activation.CutResult{Candidates: []activation.CutCandidate{processImage}}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return processImageRecoveryPlan(cut.Recommended.NodeKey), nil
	})
	recoveryCalls := 0
	withAdapterRecovery(t, func(report reportv2.Report, candidate activation.CutCandidate, plan *Plan) (*AdapterPlan, []AdmissionRefusal) {
		recoveryCalls++
		return processImageRecoveryAdapterPlan(), nil
	})

	verdict, _, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("adapter recovery refused processImage: %s", verdict.Error())
	}
	if recoveryCalls != 1 {
		t.Fatalf("tryAdapterRecovery called %d times during admission, want exactly 1", recoveryCalls)
	}

	// Build-plan reuse: the cached plan is available and the invariant holds,
	// and the lookup does not re-invoke recovery.
	cached := cachedAdapterPlanFor(*cut.Recommended)
	if cached == nil {
		t.Fatal("expected cached adapter plan for build-plan reuse, got nil")
	}
	if cached.SourceFunction != "processImage" {
		t.Fatalf("cached adapter plan SourceFunction = %q, want processImage", cached.SourceFunction)
	}
	if recoveryCalls != 1 {
		t.Fatalf("cachedAdapterPlanFor must not invoke recovery; call count now %d", recoveryCalls)
	}

	// Invariant guard: a cache entry whose stored SourceFunction disagrees with
	// the candidate's function name must not be reused (defends against a stale
	// or mis-keyed entry). Store a plan for "processImage" under a "renamed"
	// key and confirm the lookup refuses it.
	renamed := processImage
	renamed.NodeKey.FuncName = "renamed"
	storeCandidateAdapterPlan(candidateAdmitKey(renamed), processImageRecoveryAdapterPlan())
	if cachedAdapterPlanFor(renamed) != nil {
		t.Fatal("invariant guard failed: reused a plan whose SourceFunction != candidate function name")
	}
}

func TestAdmitCutCandidatesAdapterProofFailureDemotes(t *testing.T) {
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")
	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "processImage"
	awkward.NodeName = "processImage"
	clean := admissibleTestCandidate()
	clean.Step = 3
	clean.NodeKey.FuncName = "Leaf"
	clean.NodeName = "Leaf"
	cut := &activation.CutResult{Candidates: []activation.CutCandidate{awkward, clean}}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		if cut.Recommended.NodeKey.FuncName == "processImage" {
			return processImageRecoveryPlan(cut.Recommended.NodeKey), nil
		}
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})
	withAdapterRecovery(t, func(report reportv2.Report, candidate activation.CutCandidate, plan *Plan) (*AdapterPlan, []AdmissionRefusal) {
		return nil, []AdmissionRefusal{{Code: RefusalAdapterUseShape, Message: "multiple Open calls"}}
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
	var failed *activation.CutCandidate
	for i := range cut.Candidates {
		if cut.Candidates[i].NodeName == "processImage" {
			failed = &cut.Candidates[i]
			break
		}
	}
	if failed == nil {
		t.Fatal("processImage candidate not found")
	}
	if failed.AdapterClass != activation.AdapterUnknown {
		t.Fatalf("failed adapter class = %s, want AdapterUnknown", failed.AdapterClass)
	}
}

func TestAdmitCutCandidatesDirectAdmissibleDoesNotRunAdapter(t *testing.T) {
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")
	direct := admissibleTestCandidate()
	direct.NodeKey.FuncName = "Direct"
	direct.NodeName = "Direct"
	cut := &activation.CutResult{Candidates: []activation.CutCandidate{direct}}
	cut.Recommended = &cut.Candidates[0]
	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			Results:  []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})
	withAdapterRecovery(t, func(report reportv2.Report, candidate activation.CutCandidate, plan *Plan) (*AdapterPlan, []AdmissionRefusal) {
		t.Fatalf("adapter recovery should not run for direct-admissible candidate")
		return nil, nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if !verdict.Accepted {
		t.Fatalf("direct candidate refused: %s", verdict.Error())
	}
	if len(chain) != 0 {
		t.Fatalf("demotion chain = %+v, want none", chain)
	}
}

func TestAdmitCutCandidatesFlagOffDoesNotMarkAdapterEligibility(t *testing.T) {
	// With MONOLIFT_BOUNDARY_ADAPTER=0, the adapter branch should NOT fire,
	// so the candidate's AdapterClass should remain at its default value.
	// Uses a non-DTO-normalizable multi-return to trigger demotion.
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "0")

	awkward := admissibleTestCandidate()
	awkward.Step = 2
	awkward.NodeKey.FuncName = "ProcessStream"
	awkward.NodeName = "ProcessStream"
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
		case "ProcessStream":
			return &Plan{
				CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
				Results: []Result{
					{GoType: "func() error", Codec: CodecJSON},
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
		if cut.Candidates[i].NodeKey.FuncName == "ProcessStream" {
			demoted = &cut.Candidates[i]
			break
		}
	}
	if demoted == nil {
		t.Fatal("ProcessStream candidate not found in cut.Candidates")
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

// SPRINT-0052 task 2.2 (flag B-10): with the flag ON, a callable candidate
// that adapter recovery cannot rescue (high callback class + function-typed
// boundary, which adapterRecoveryAllowed rejects) must still be refused with
// callable_boundary_values. Before the fix, the flag silently suppressed the
// refusal in AdmitCut and the candidate was admitted directly as a boundary.
func TestAdmitCutCandidatesFlagOnCallableNotRecoverableStaysRefused(t *testing.T) {
	t.Setenv("MONOLIFT_BOUNDARY_ADAPTER", "1")

	callable := admissibleTestCandidate()
	callable.Step = 2
	callable.NodeKey.FuncName = "RegisterHook"
	callable.NodeName = "RegisterHook"
	callable.Callbacks = activation.Many
	cut := &activation.CutResult{
		Candidates: []activation.CutCandidate{callable},
	}
	cut.Recommended = &cut.Candidates[0]

	withCandidatePlanBuilder(t, func(report reportv2.Report, cut activation.CutResult) (*Plan, error) {
		// A function-typed boundary parameter makes adapterRecoveryAllowed
		// return false for a Many-callback candidate, so recovery is declined
		// and the callable refusal must stand.
		return &Plan{
			CutPoint: CutPoint{Key: cut.Recommended.NodeKey},
			BoundaryParams: []Param{
				{Name: "cb", JSONName: "cb", GoType: "func()", QualifiedGoType: "func()", Codec: CodecJSON, Index: 0},
			},
			Results: []Result{{GoType: "string", Codec: CodecPrimitive}},
		}, nil
	})

	verdict, chain, err := admitCutCandidates(reportv2.Report{}, cut)
	if err != nil {
		t.Fatalf("admitCutCandidates returned error: %v", err)
	}
	if verdict.Accepted {
		t.Fatal("flag-on callable candidate was admitted; callable_boundary_values must still be reported")
	}
	if !hasRefusal(verdict, "callable_boundary_values") {
		t.Fatalf("expected callable_boundary_values to stand, got: %s", verdict.Error())
	}
	if len(chain) == 0 || chain[len(chain)-1].RefusalCode != "callable_boundary_values" {
		t.Fatalf("demotion chain = %+v, want final refusal callable_boundary_values", chain)
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

func withAdapterRecovery(t *testing.T, recover func(reportv2.Report, activation.CutCandidate, *Plan) (*AdapterPlan, []AdmissionRefusal)) {
	t.Helper()
	old := tryAdapterRecovery
	tryAdapterRecovery = recover
	t.Cleanup(func() {
		tryAdapterRecovery = old
	})
}

func resetCandidateAdmitCacheForTest() {
	candidateAdmitCache.Lock()
	candidateAdmitCache.results = map[candidateAdmitCacheKey]candidateAdmitResult{}
	candidateAdmitCache.Unlock()
	resetAdapterSSACache()
}

func processImageRecoveryPlan(key activation.FunctionKey) *Plan {
	plan := &Plan{
		CutPoint: CutPoint{Key: key, FuncName: key.FuncName},
		BoundaryParams: []Param{
			{Name: "file", JSONName: "file", GoType: "*multipart.FileHeader", QualifiedGoType: "*mime/multipart.FileHeader", TypePackagePath: "mime/multipart", Codec: CodecJSON, Index: 0},
		},
		Results: []Result{
			{Name: "result", JSONName: "result", GoType: "func() error", Codec: CodecJSON, Index: 0},
			{Name: "width", JSONName: "width", GoType: "int", Codec: CodecPrimitive, Index: 1},
			{Name: "err", JSONName: "error", GoType: "error", Codec: CodecError, Index: 2},
		},
	}
	return plan
}

func processImageRecoveryAdapterPlan() *AdapterPlan {
	return &AdapterPlan{
		SourceFunction:  "processImage",
		HostSignature:   "(*multipart.FileHeader) (*bytes.Reader, int, int, error)",
		RemoteSignature: "([]byte) ([]byte, int, int, error)",
		InputTransforms: []AdapterPattern{{
			Name:      "multipart_file_read_all",
			ParamName: "file",
			FromType:  "*multipart.FileHeader",
			ToType:    "[]byte",
		}},
		OutputTransforms: []AdapterPattern{{
			Name:     "bytes_reader_return",
			FromType: "func() error",
			ToType:   "[]byte",
		}},
		Proofs: []AdapterProof{
			{Obligation: RefusalAdapterFiniteInput, Satisfied: true},
			{Obligation: RefusalAdapterLocalLifecycle, Satisfied: true},
			{Obligation: RefusalAdapterUseShape, Satisfied: true},
			{Obligation: RefusalAdapterReturnRehydration, Satisfied: true},
			{Obligation: RefusalAdapterErrorOrder, Satisfied: true},
			{Obligation: RefusalAdapterCallSite, Satisfied: true},
		},
		TransportPolicy: AdapterTransportInlineJSONBytes,
	}
}

// TestAdapterParentForbiddenByStructure exercises the structural predicate
// that replaces the SPRINT-0051 isUploadMediaCandidate string match. The
// signal is purely the AdapterClass label on deeper candidates — no
// function names, no types. Phase 1.1 of SPRINT-0052.
func TestAdapterParentForbiddenByStructure(t *testing.T) {
	tests := []struct {
		name       string
		candidates []activation.CutCandidate
		focusStep  int
		wantForbid bool
	}{
		{
			name: "M-4 shape: deeper candidate has AdapterUnknown — parent forbidden",
			candidates: []activation.CutCandidate{
				{Step: 2, NodeKey: activation.FunctionKey{FuncName: "Parent"}, AdapterClass: activation.DirectBoundary},
				{Step: 3, NodeKey: activation.FunctionKey{FuncName: "Leaf"}, AdapterClass: activation.AdapterUnknown},
			},
			focusStep:  2,
			wantForbid: true,
		},
		{
			name: "M-4 shape after adapter recovery refused: leaf labeled AdapterImpossible — parent still forbidden",
			candidates: []activation.CutCandidate{
				{Step: 2, NodeKey: activation.FunctionKey{FuncName: "Parent"}, AdapterClass: activation.DirectBoundary},
				{Step: 3, NodeKey: activation.FunctionKey{FuncName: "Leaf"}, AdapterClass: activation.AdapterImpossible},
			},
			focusStep:  2,
			wantForbid: true,
		},
		{
			name: "Unrelated parent: deeper candidate is DirectBoundary — parent admits",
			candidates: []activation.CutCandidate{
				{Step: 2, NodeKey: activation.FunctionKey{FuncName: "Parent"}, AdapterClass: activation.DirectBoundary},
				{Step: 3, NodeKey: activation.FunctionKey{FuncName: "Leaf"}, AdapterClass: activation.DirectBoundary},
			},
			focusStep:  2,
			wantForbid: false,
		},
		{
			name: "Descendant with no adapter classification (unset): parent admits",
			candidates: []activation.CutCandidate{
				{Step: 2, NodeKey: activation.FunctionKey{FuncName: "Parent"}, AdapterClass: activation.DirectBoundary},
				{Step: 3, NodeKey: activation.FunctionKey{FuncName: "Leaf"}, AdapterClass: ""},
			},
			focusStep:  2,
			wantForbid: false,
		},
		{
			name: "Leaf itself never forbidden — no deeper candidates",
			candidates: []activation.CutCandidate{
				{Step: 2, NodeKey: activation.FunctionKey{FuncName: "Parent"}, AdapterClass: activation.DirectBoundary},
				{Step: 3, NodeKey: activation.FunctionKey{FuncName: "Leaf"}, AdapterClass: activation.AdapterUnknown},
			},
			focusStep:  3,
			wantForbid: false,
		},
		{
			name:       "Nil cut: predicate admits (degenerate)",
			candidates: nil,
			focusStep:  2,
			wantForbid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cut *activation.CutResult
			var candidate activation.CutCandidate
			if tt.candidates != nil {
				cut = &activation.CutResult{Candidates: tt.candidates}
				for _, c := range tt.candidates {
					if c.Step == tt.focusStep {
						candidate = c
						break
					}
				}
			} else {
				candidate = activation.CutCandidate{Step: tt.focusStep}
			}
			got := adapterParentForbiddenForCandidate(candidate, cut)
			if got != tt.wantForbid {
				t.Fatalf("adapterParentForbiddenForCandidate = %v, want %v", got, tt.wantForbid)
			}
		})
	}
}
