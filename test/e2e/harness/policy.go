package harness

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/tgoodwin/monolift/pkg/codegen"
	"sigs.k8s.io/yaml"
)

func DirectInvokeExpectationFor(check DirectInvokeCheck, hasOracle bool) DirectInvokeExpectation {
	if check.Expectation != "" {
		return check.Expectation
	}
	if hasOracle {
		return DirectInvokeOracleCompare
	}
	return DirectInvokeNonNilResult
}

func (e DirectInvokeExpectation) Valid() bool {
	switch e {
	case "",
		DirectInvokeOracleCompare,
		DirectInvokeNonNilResult,
		DirectInvokeNullableLocalizedError,
		DirectInvokeStatusOnly,
		DirectInvokeBehavioralInvariant,
		DirectInvokeWorkloadCallsDelta:
		return true
	default:
		return false
	}
}

func (t TargetCase) ValidateStagePolicy() error {
	if !t.DirectInvoke.Expectation.Valid() {
		return fmt.Errorf("unknown direct-invoke expectation %q", t.DirectInvoke.Expectation)
	}
	switch t.DirectInvoke.Expectation {
	case DirectInvokeNullableLocalizedError, DirectInvokeBehavioralInvariant, DirectInvokeWorkloadCallsDelta:
		if strings.TrimSpace(t.DirectInvoke.Predicate) == "" && len(t.BehavioralPredicates) == 0 {
			return fmt.Errorf("direct-invoke expectation %q requires a declared predicate", t.DirectInvoke.Expectation)
		}
	}
	for _, predicate := range t.BehavioralPredicates {
		if strings.TrimSpace(predicate.Name) == "" {
			return fmt.Errorf("behavioral predicate has empty name")
		}
		if strings.TrimSpace(predicate.Description) == "" {
			return fmt.Errorf("behavioral predicate %q has empty description", predicate.Name)
		}
	}
	for _, requirement := range t.WorkloadRequirements {
		if strings.TrimSpace(requirement.Name) == "" {
			return fmt.Errorf("workload requirement has empty name")
		}
		if strings.TrimSpace(requirement.Description) == "" {
			return fmt.Errorf("workload requirement %q has empty description", requirement.Name)
		}
		if strings.TrimSpace(requirement.EnvVar) == "" {
			return fmt.Errorf("workload requirement %q has empty env var", requirement.Name)
		}
		if t.ActivationLift != nil && strings.TrimSpace(requirement.Value) != "" && !hostEnvHasValue(t.ActivationLift.Deploy.HostEnvVars, requirement.EnvVar, requirement.Value) {
			return fmt.Errorf("workload requirement %q requires %s=%q in activation host env", requirement.Name, requirement.EnvVar, requirement.Value)
		}
	}
	if t.FreshResourcePolicy.ResourceKind != "" {
		if strings.TrimSpace(t.FreshResourcePolicy.Scope) == "" {
			return fmt.Errorf("fresh-resource policy for %s has empty scope", t.FreshResourcePolicy.ResourceKind)
		}
		if strings.TrimSpace(t.FreshResourcePolicy.Description) == "" {
			return fmt.Errorf("fresh-resource policy for %s has empty description", t.FreshResourcePolicy.ResourceKind)
		}
	}
	return nil
}

func (t TargetCase) ValidateWorkloadFitness() error {
	return validateWorkloadFitness(t, func(path string) ([]byte, error) {
		return os.ReadFile(FromRepoRoot(path))
	})
}

func validateWorkloadFitness(t TargetCase, readManifest func(string) ([]byte, error)) error {
	for _, requirement := range t.WorkloadRequirements {
		if strings.TrimSpace(requirement.Value) == "" || len(t.BaselineManifests) == 0 {
			continue
		}
		found, err := baselineManifestsHaveEnv(t.BaselineManifests, requirement.EnvVar, requirement.Value, readManifest)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("workload requirement %q requires %s=%q in baseline manifests", requirement.Name, requirement.EnvVar, requirement.Value)
		}
	}
	return nil
}

func baselineManifestsHaveEnv(manifests []string, name, value string, readManifest func(string) ([]byte, error)) (bool, error) {
	for _, manifest := range manifests {
		data, err := readManifest(manifest)
		if err != nil {
			return false, fmt.Errorf("read baseline manifest %s: %w", manifest, err)
		}
		for _, doc := range splitYAMLDocuments(data) {
			if len(bytes.TrimSpace(doc)) == 0 {
				continue
			}
			var decoded any
			if err := yaml.Unmarshal(doc, &decoded); err != nil {
				return false, fmt.Errorf("parse baseline manifest %s: %w", manifest, err)
			}
			if yamlValueHasEnv(decoded, name, value) {
				return true, nil
			}
		}
	}
	return false, nil
}

func yamlValueHasEnv(value any, name, want string) bool {
	switch typed := value.(type) {
	case map[string]any:
		if env, ok := typed["env"].([]any); ok && envListHasValue(env, name, want) {
			return true
		}
		for _, child := range typed {
			if yamlValueHasEnv(child, name, want) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if yamlValueHasEnv(child, name, want) {
				return true
			}
		}
	}
	return false
}

func envListHasValue(env []any, name, want string) bool {
	for _, item := range env {
		envVar, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if envVar["name"] == name && fmt.Sprint(envVar["value"]) == want {
			return true
		}
	}
	return false
}

func hostEnvHasValue(env []codegen.EnvVar, name, value string) bool {
	for _, item := range env {
		if item.Name == name && item.Value == value {
			return true
		}
	}
	return false
}

func CheckDirectInvokeResult(expectation DirectInvokeExpectation, got, oracle any, hasOracle bool, predicate string) error {
	if !expectation.Valid() {
		return fmt.Errorf("unknown direct-invoke expectation %q", expectation)
	}
	switch expectation {
	case "":
		return CheckDirectInvokeResult(DirectInvokeExpectationFor(DirectInvokeCheck{}, hasOracle), got, oracle, hasOracle, predicate)
	case DirectInvokeOracleCompare:
		if !hasOracle {
			return fmt.Errorf("oracle-compare requires a target oracle")
		}
		if fmt.Sprint(got) != fmt.Sprint(oracle) {
			return fmt.Errorf("oracle mismatch: got=%v want=%v", got, oracle)
		}
	case DirectInvokeNonNilResult:
		if got == nil {
			return fmt.Errorf("non-nil-result expectation got nil")
		}
	case DirectInvokeNullableLocalizedError:
		if strings.TrimSpace(predicate) == "" {
			return fmt.Errorf("nullable-localized-error requires a declared predicate")
		}
	case DirectInvokeStatusOnly:
		// postInvoke already proved HTTP 200; no result value is required.
	case DirectInvokeBehavioralInvariant:
		if strings.TrimSpace(predicate) == "" {
			return fmt.Errorf("behavioral-invariant requires a declared predicate")
		}
	case DirectInvokeWorkloadCallsDelta:
		if strings.TrimSpace(predicate) == "" {
			return fmt.Errorf("workload-calls-delta requires a declared predicate")
		}
	}
	return nil
}
