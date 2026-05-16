package harness

import (
	"errors"
	"fmt"
)

type FailureKind string

const (
	KindHarness  FailureKind = "harness"
	KindCompiler FailureKind = "compiler"
	KindArtifact FailureKind = "artifact"
	KindWorkload FailureKind = "workload"
	KindOracle   FailureKind = "oracle"
	KindFitness  FailureKind = "workload-fitness"
)

type ClassifiedError struct {
	Kind FailureKind
	Err  error
}

func Classify(kind FailureKind, err error) error {
	if err == nil {
		return nil
	}
	return ClassifiedError{Kind: kind, Err: err}
}

func Classified(kind FailureKind, format string, args ...any) error {
	return Classify(kind, fmt.Errorf(format, args...))
}

func (e ClassifiedError) Error() string {
	return e.Err.Error()
}

func (e ClassifiedError) Unwrap() error {
	return e.Err
}

func ErrorKind(err error, fallback FailureKind) FailureKind {
	var classified ClassifiedError
	if errors.As(err, &classified) && classified.Kind != "" {
		return classified.Kind
	}
	return fallback
}

func StageError(stage int, target string, kind FailureKind, format string, args ...any) error {
	return fmt.Errorf(StagePrefix(stage, target, kind)+" "+format, args...)
}

func StagePrefix(stage int, target string, kind FailureKind) string {
	return fmt.Sprintf("[stage=%d target=%s kind=%s]", stage, target, kind)
}
