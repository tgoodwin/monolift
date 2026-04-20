package harness

import "fmt"

type FailureKind string

const (
	KindHarness  FailureKind = "harness"
	KindCompiler FailureKind = "compiler"
	KindArtifact FailureKind = "artifact"
	KindWorkload FailureKind = "workload"
)

func StageError(stage int, target string, kind FailureKind, format string, args ...any) error {
	return fmt.Errorf(StagePrefix(stage, target, kind)+" "+format, args...)
}

func StagePrefix(stage int, target string, kind FailureKind) string {
	return fmt.Sprintf("[stage=%d target=%s kind=%s]", stage, target, kind)
}
