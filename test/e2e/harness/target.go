package harness

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/activation"
	"github.com/tgoodwin/monolift/pkg/codegen"
)

type TargetCase struct {
	Name                          string
	ExpectedVerdict               string
	ExpectedRoot                  string
	ExpectedRoots                 []string
	ExpectedRootShape             string
	ExpectedTransport             string
	ExpectedArchetypeKind         string
	ExpectedPrimary               ExpectedArchetypeChoice
	ExpectedAlternatives          []ExpectedArchetypeChoice
	ExpectedAdapterKind           string
	ExpectedAdapterID             string
	RequiredRootFacts             []ExpectedPropertyFact
	StopAtStage                   int
	RequiredDiagnostics           []string
	SkipReason                    string
	SpecTrace                     string
	BaselineManifests             []string
	BaselineManifestPhases        [][]string
	BaselineReadyTimeout          time.Duration
	LiftedReadyTimeout            time.Duration
	Dockerfile                    string
	ContextDir                    string
	SourceDirs                    []string
	ImageTag                      string
	LiftedInfrastructureManifests []string
	LiftedHostBuild               *HostBuildSpec
	LiftedExtractedServices       []ExtractedServiceSpec
	LiftedOracleServices          []ExtractedServiceSpec
	ActivationLift                *ActivationLiftSpec
	GoldenReport                  string
	EntryPathProbePackage         string
	EntryPathProbeRoots           []string
	Workload                      WorkloadExecutor
	Oracle                        SymbolInvoker
	Invariants                    []Invariant
	DirectInvoke                  DirectInvokeCheck
	BehavioralPredicates          []BehavioralPredicate
	TranscriptNormalizers         []TranscriptNormalizer
	FreshResourcePolicy           FreshResourcePolicy
	WorkloadRequirements          []WorkloadRequirement
	ServiceSymbols                map[string]string
	InvokePayloads                map[string]map[string]any
	ServiceName                   string
	ServicePort                   int
}

type ActivationLiftSpec struct {
	Target                       string
	ServiceName                  string
	Augment                      activation.AugmentMode
	Deploy                       codegen.DeployOptions
	ExpectedEnvVarPrefix         string
	DirectInvocationProbePayload map[string]any
	GoWorkModules                []string
}

type SymbolInvoker interface {
	Invoke(args map[string]any) (any, error)
}

type DirectInvokeExpectation string

const (
	DirectInvokeOracleCompare          DirectInvokeExpectation = "oracle-compare"
	DirectInvokeNonNilResult           DirectInvokeExpectation = "non-nil-result"
	DirectInvokeNullableLocalizedError DirectInvokeExpectation = "nullable-localized-error"
	DirectInvokeStatusOnly             DirectInvokeExpectation = "status-only"
	DirectInvokeBehavioralInvariant    DirectInvokeExpectation = "behavioral-invariant"
	DirectInvokeWorkloadCallsDelta     DirectInvokeExpectation = "workload-calls-delta"
)

type DirectInvokeCheck struct {
	Expectation DirectInvokeExpectation
	Predicate   string
}

type BehavioralPredicate struct {
	Name        string
	Description string
}

type FreshResourcePolicy struct {
	ResourceKind string
	Scope        string
	Description  string
}

type WorkloadRequirement struct {
	Name        string
	Description string
	EnvVar      string
	Value       string
}

type HostBuildSpec struct {
	Dockerfile     string
	ContextRoot    string
	ImageTag       string
	ServiceName    string
	DeploymentYAML string
	ServiceYAML    string
}

type ExtractedServiceSpec struct {
	Name           string
	Dockerfile     string
	ContextRoot    string
	ImageTag       string
	DeploymentYAML string
	ServiceYAML    string
	ReadinessPath  string
}

type ExpectedPropertyFact struct {
	PropertyID string
	Verdict    string
}

type ExpectedArchetypeChoice struct {
	Archetype              string
	ContributingArchetypes []string
	Alias                  string
	Emittable              *bool
	RuntimeSelectable      *bool
	RationaleTierEqual     string
	RationaleNonEmpty      bool
}
