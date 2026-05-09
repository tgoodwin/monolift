package harness

import (
	"time"

	"github.com/tgoodwin/monolift/pkg/codegen"
)

type TargetCase struct {
	Name                    string
	ExpectedVerdict         string
	ExpectedRoot            string
	ExpectedRoots           []string
	ExpectedRootShape       string
	ExpectedTransport       string
	ExpectedArchetypeKind   string
	ExpectedPrimary         ExpectedArchetypeChoice
	ExpectedAlternatives    []ExpectedArchetypeChoice
	ExpectedAdapterKind     string
	ExpectedAdapterID       string
	RequiredRootFacts       []ExpectedPropertyFact
	StopAtStage             int
	RequiredDiagnostics     []string
	SkipReason              string
	SpecTrace               string
	BaselineManifests       []string
	BaselineManifestPhases  [][]string
	BaselineReadyTimeout    time.Duration
	LiftedReadyTimeout      time.Duration
	Dockerfile              string
	ContextDir              string
	SourceDirs              []string
	ImageTag                string
	LiftedHostBuild         *HostBuildSpec
	LiftedExtractedServices []ExtractedServiceSpec
	LiftedOracleServices    []ExtractedServiceSpec
	ActivationLift          *ActivationLiftSpec
	GoldenReport            string
	EntryPathProbePackage   string
	EntryPathProbeRoots     []string
	Workload                WorkloadExecutor
	Oracle                  SymbolInvoker
	Invariants              []Invariant
	ServiceSymbols          map[string]string
	InvokePayloads          map[string]map[string]any
	ServiceName             string
	ServicePort             int
}

type ActivationLiftSpec struct {
	Target                       string
	ServiceName                  string
	Deploy                       codegen.DeployOptions
	ExpectedEnvVarPrefix         string
	DirectInvocationProbePayload map[string]any
	GoWorkModules                []string
}

type SymbolInvoker interface {
	Invoke(args map[string]any) (any, error)
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
