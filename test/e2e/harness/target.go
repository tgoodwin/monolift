package harness

type TargetCase struct {
	Name                string
	ExpectedVerdict     string
	ExpectedRoot        string
	StopAtStage         int
	RequiredDiagnostics []string
	SkipReason          string
	SpecTrace           string
	BaselineManifests   []string
	Dockerfile          string
	ContextDir          string
	SourceDirs          []string
	ImageTag            string
	GoldenReport        string
	Workload            WorkloadExecutor
	Invariants          []Invariant
	ServiceName         string
	ServicePort         int
}
