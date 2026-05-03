package main

import "testing"

func TestSplitPatterns(t *testing.T) {
	got := splitPatterns(" ./cmd , ., ")
	want := []string{"./cmd", "."}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunRejectsMissingTarget(t *testing.T) {
	if code := run([]string{"--packages", "."}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
