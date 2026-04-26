package harness

type TargetCase struct {
	Name                    string
	ExpectedVerdict         string
	ExpectedRoot            string
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
	Dockerfile              string
	ContextDir              string
	SourceDirs              []string
	ImageTag                string
	LiftedHostBuild         *HostBuildSpec
	LiftedExtractedServices []ExtractedServiceSpec
	GoldenReport            string
	Workload                WorkloadExecutor
	Oracle                  SymbolInvoker
	Invariants              []Invariant
	ServiceName             string
	ServicePort             int
}

type SymbolInvoker interface {
	Invoke(args map[string]any) (any, error)
}

type HostBuildSpec struct {
	Dockerfile  string
	ContextRoot string
	ImageTag    string
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
