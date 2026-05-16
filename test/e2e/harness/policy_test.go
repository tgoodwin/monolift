package harness

import (
	"testing"

	"github.com/tgoodwin/monolift/pkg/codegen"
)

func TestCheckDirectInvokeResultOracleCompare(t *testing.T) {
	if err := CheckDirectInvokeResult(DirectInvokeOracleCompare, "ok", "ok", true, ""); err != nil {
		t.Fatalf("oracle compare accepted result: %v", err)
	}
	if err := CheckDirectInvokeResult(DirectInvokeOracleCompare, "got", "want", true, ""); err == nil {
		t.Fatalf("oracle mismatch was accepted")
	}
	if err := CheckDirectInvokeResult(DirectInvokeOracleCompare, "ok", nil, false, ""); err == nil {
		t.Fatalf("missing oracle was accepted")
	}
}

func TestCheckDirectInvokeResultNonNil(t *testing.T) {
	if err := CheckDirectInvokeResult(DirectInvokeNonNilResult, "ok", nil, false, ""); err != nil {
		t.Fatalf("non-nil result was refused: %v", err)
	}
	if err := CheckDirectInvokeResult(DirectInvokeNonNilResult, nil, nil, false, ""); err == nil {
		t.Fatalf("nil result was accepted")
	}
}

func TestCheckDirectInvokeResultNullableLocalizedErrorRequiresPredicate(t *testing.T) {
	if err := CheckDirectInvokeResult(DirectInvokeNullableLocalizedError, nil, nil, false, "feed entries exist"); err != nil {
		t.Fatalf("nullable localized error with predicate was refused: %v", err)
	}
	if err := CheckDirectInvokeResult(DirectInvokeNullableLocalizedError, nil, nil, false, ""); err == nil {
		t.Fatalf("nullable localized error without predicate was accepted")
	}
}

func TestCheckDirectInvokeResultStatusOnly(t *testing.T) {
	if err := CheckDirectInvokeResult(DirectInvokeStatusOnly, nil, nil, false, ""); err != nil {
		t.Fatalf("status-only result was refused: %v", err)
	}
}

func TestCheckDirectInvokeResultWorkloadCallsDeltaRequiresPredicate(t *testing.T) {
	if err := CheckDirectInvokeResult(DirectInvokeWorkloadCallsDelta, nil, nil, false, "calls delta increases"); err != nil {
		t.Fatalf("workload calls delta with predicate was refused: %v", err)
	}
	if err := CheckDirectInvokeResult(DirectInvokeWorkloadCallsDelta, nil, nil, false, ""); err == nil {
		t.Fatalf("workload calls delta without predicate was accepted")
	}
}

func TestValidateStagePolicyRequiresPredicateForSubstitutions(t *testing.T) {
	target := TargetCase{
		Name:         "target",
		DirectInvoke: DirectInvokeCheck{Expectation: DirectInvokeWorkloadCallsDelta},
	}
	if err := target.ValidateStagePolicy(); err == nil {
		t.Fatalf("target without predicate was accepted")
	}
	target.DirectInvoke.Predicate = "calls delta increases"
	if err := target.ValidateStagePolicy(); err != nil {
		t.Fatalf("target with predicate was refused: %v", err)
	}
}

func TestValidateStagePolicyFreshResourcePolicy(t *testing.T) {
	target := TargetCase{
		Name: "target",
		FreshResourcePolicy: FreshResourcePolicy{
			ResourceKind: "postgres",
		},
	}
	if err := target.ValidateStagePolicy(); err == nil {
		t.Fatalf("fresh resource policy without scope and description was accepted")
	}
	target.FreshResourcePolicy.Scope = "per workload Setup call"
	if err := target.ValidateStagePolicy(); err == nil {
		t.Fatalf("fresh resource policy without description was accepted")
	}
	target.FreshResourcePolicy.Description = "uses a unique fixture resource for every setup call"
	if err := target.ValidateStagePolicy(); err != nil {
		t.Fatalf("fresh resource policy with scope and description was refused: %v", err)
	}
}

func TestValidateStagePolicyWorkloadRequirementMatchesActivationHostEnv(t *testing.T) {
	target := TargetCase{
		Name: "target",
		ActivationLift: &ActivationLiftSpec{
			Deploy: codegen.DeployOptions{
				HostEnvVars: []codegen.EnvVar{{Name: "FEATURE_MODE", Value: "enabled"}},
			},
		},
		WorkloadRequirements: []WorkloadRequirement{{
			Name:        "feature mode",
			Description: "workload must enter the feature path",
			EnvVar:      "FEATURE_MODE",
			Value:       "enabled",
		}},
	}
	if err := target.ValidateStagePolicy(); err != nil {
		t.Fatalf("target with matching workload requirement was refused: %v", err)
	}
	target.WorkloadRequirements[0].Value = "disabled"
	if err := target.ValidateStagePolicy(); err == nil {
		t.Fatalf("target with mismatched workload requirement was accepted")
	}
	target.WorkloadRequirements[0].Value = "enabled"
	target.WorkloadRequirements[0].Description = ""
	if err := target.ValidateStagePolicy(); err == nil {
		t.Fatalf("target with empty workload requirement description was accepted")
	}
}

func TestValidateWorkloadFitnessChecksBaselineManifestEnv(t *testing.T) {
	target := TargetCase{
		Name:              "target",
		BaselineManifests: []string{"deployment.yaml"},
		WorkloadRequirements: []WorkloadRequirement{{
			Name:        "feature mode",
			Description: "workload must enter the feature path",
			EnvVar:      "FEATURE_MODE",
			Value:       "enabled",
		}},
	}
	reader := func(path string) ([]byte, error) {
		return []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  template:
    spec:
      containers:
      - name: app
        env:
        - name: FEATURE_MODE
          value: enabled
`), nil
	}
	if err := validateWorkloadFitness(target, reader); err != nil {
		t.Fatalf("matching baseline manifest was refused: %v", err)
	}
	target.WorkloadRequirements[0].Value = "disabled"
	if err := validateWorkloadFitness(target, reader); err == nil {
		t.Fatalf("baseline manifest missing required env value was accepted")
	}
}
