package harness

import (
	"strings"
	"testing"
)

func TestNamespaceShortNameUntouched(t *testing.T) {
	got := Namespace("baseline", "pragma-parse", "1776793707440413000")
	want := "mlv2-baseline-pragma-parse-1776793707440413000"
	if got != want {
		t.Fatalf("Namespace = %q, want %q", got, want)
	}
	if len(got) > maxNamespaceLen {
		t.Fatalf("len = %d > %d", len(got), maxNamespaceLen)
	}
}

func TestNamespaceLongNameTruncatedWithHash(t *testing.T) {
	target := "state-decl-conflict-stateless-global-store"
	runID := "1776793707440413000"
	got := Namespace("baseline", target, runID)
	if len(got) > maxNamespaceLen {
		t.Fatalf("len = %d > %d: %q", len(got), maxNamespaceLen, got)
	}
	if !strings.HasPrefix(got, "mlv2-baseline-state-decl-conflict") {
		t.Fatalf("expected prefix preserved, got %q", got)
	}
	if !strings.HasSuffix(got, "-"+runID) {
		t.Fatalf("expected runID suffix, got %q", got)
	}
	if strings.Contains(got, "--") {
		t.Fatalf("expected no double dash, got %q", got)
	}
}

func TestNamespaceCollisionResistance(t *testing.T) {
	runID := "1776793707440413000"
	a := Namespace("baseline", "state-decl-conflict-stateless-global-store", runID)
	b := Namespace("baseline", "state-decl-conflict-stateless-global-queue", runID)
	if a == b {
		t.Fatalf("distinct targets collided: %q", a)
	}
}
