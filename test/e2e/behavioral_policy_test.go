//go:build e2e

package e2e

import (
	"context"
	"testing"

	"github.com/tgoodwin/monolift/test/e2e/harness"
	activation_gitea_argon2hash "github.com/tgoodwin/monolift/test/e2e/targets/activation_gitea_argon2hash"
)

type behaviorVerifierWorkload struct{}

func (behaviorVerifierWorkload) Setup(context.Context, string) error { return nil }
func (behaviorVerifierWorkload) Action(context.Context, string) (harness.Transcript, error) {
	return harness.Transcript{}, nil
}
func (behaviorVerifierWorkload) Verify(context.Context, string, harness.Transcript) error {
	return nil
}
func (behaviorVerifierWorkload) VerifyBehavior(context.Context, string, harness.Transcript) error {
	return nil
}

type noBehaviorVerifierWorkload struct{}

func (noBehaviorVerifierWorkload) Setup(context.Context, string) error { return nil }
func (noBehaviorVerifierWorkload) Action(context.Context, string) (harness.Transcript, error) {
	return harness.Transcript{}, nil
}
func (noBehaviorVerifierWorkload) Verify(context.Context, string, harness.Transcript) error {
	return nil
}

func TestAssertBehavioralPredicatesRequiresVerifier(t *testing.T) {
	target := harness.TargetCase{
		Name:     "target",
		Workload: noBehaviorVerifierWorkload{},
		BehavioralPredicates: []harness.BehavioralPredicate{{
			Name:        "predicate",
			Description: "predicate description",
		}},
	}
	if err := assertBehavioralPredicates(context.Background(), target, "http://example", harness.Transcript{}); err == nil {
		t.Fatalf("target without verifier was accepted")
	}
	target.Workload = behaviorVerifierWorkload{}
	if err := assertBehavioralPredicates(context.Background(), target, "http://example", harness.Transcript{}); err != nil {
		t.Fatalf("target with verifier was refused: %v", err)
	}
}

func TestCompareTargetTranscriptsClassifiesNormalizerFailure(t *testing.T) {
	target := harness.TargetCase{
		Name: "target",
		TranscriptNormalizers: []harness.TranscriptNormalizer{
			func(*harness.Transcript) {
				panic("bad normalizer")
			},
		},
	}
	err := compareTargetTranscripts(target, harness.Transcript{}, harness.Transcript{})
	if err == nil {
		t.Fatalf("normalizer panic was accepted")
	}
	if got := harness.ErrorKind(err, harness.KindWorkload); got != harness.KindOracle {
		t.Fatalf("ErrorKind=%s want %s", got, harness.KindOracle)
	}
}

func TestGiteaArgon2WorkloadRequirementFitness(t *testing.T) {
	target := activation_gitea_argon2hash.Target()
	if err := target.ValidateStagePolicy(); err != nil {
		t.Fatalf("stage policy invalid: %v", err)
	}
	if err := target.ValidateWorkloadFitness(); err != nil {
		t.Fatalf("workload fitness invalid: %v", err)
	}
}
