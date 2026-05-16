package harness

import (
	"errors"
	"testing"
)

func TestErrorKindUsesClassifiedError(t *testing.T) {
	sentinel := errors.New("boom")
	err := Classify(KindOracle, sentinel)
	if got := ErrorKind(err, KindWorkload); got != KindOracle {
		t.Fatalf("ErrorKind=%s want %s", got, KindOracle)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("classified error did not unwrap sentinel")
	}
}

func TestErrorKindFallsBack(t *testing.T) {
	if got := ErrorKind(errors.New("boom"), KindWorkload); got != KindWorkload {
		t.Fatalf("ErrorKind=%s want %s", got, KindWorkload)
	}
}
