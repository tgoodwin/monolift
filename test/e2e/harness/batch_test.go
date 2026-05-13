package harness

import (
	"strings"
	"testing"
	"time"
)

func TestBatchResultSummaryTable(t *testing.T) {
	b := &BatchResult{}
	b.Record(BatchEntry{
		Target:   "activation-miniflux-sanitizehtml",
		Status:   BatchPass,
		Stage:    "complete",
		Duration: 3 * time.Minute,
	})
	b.Record(BatchEntry{
		Target:   "activation-caddy-cleanpath",
		Status:   BatchPass,
		Stage:    "complete",
		Duration: 5 * time.Minute,
	})
	b.Record(BatchEntry{
		Target:   "activation-gitea-pathescapesegments",
		Status:   BatchE2EFail,
		Stage:    "stage-8",
		Duration: 12 * time.Minute,
		Error:    "lifted action failed: status mismatch",
	})
	b.Record(BatchEntry{
		Target: "some-skipped-target",
		Status: BatchSkipped,
		Stage:  "skip",
		Error:  "not implemented",
	})

	table := b.SummaryTable()

	if !strings.Contains(table, "BATCH RESULT SUMMARY") {
		t.Error("missing header")
	}
	if !strings.Contains(table, "Total: 4") {
		t.Errorf("wrong total count, got:\n%s", table)
	}
	if !strings.Contains(table, "pass: 2") {
		t.Errorf("wrong pass count, got:\n%s", table)
	}
	if !strings.Contains(table, "e2e-fail: 1") {
		t.Errorf("wrong fail count, got:\n%s", table)
	}
	if !strings.Contains(table, "activation-miniflux-sanitizehtml") {
		t.Errorf("missing target name, got:\n%s", table)
	}
	if !strings.Contains(table, "lifted action failed") {
		t.Errorf("missing error message, got:\n%s", table)
	}

	t.Logf("Summary table:\n%s", table)
}

func TestBatchResultEmpty(t *testing.T) {
	b := &BatchResult{}
	table := b.SummaryTable()
	if !strings.Contains(table, "No batch results recorded") {
		t.Errorf("empty batch should say so, got: %s", table)
	}
}

func TestBatchResultEntries(t *testing.T) {
	b := &BatchResult{}
	b.Record(BatchEntry{Target: "a", Status: BatchPass})
	b.Record(BatchEntry{Target: "b", Status: BatchE2EFail})
	entries := b.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Target != "a" || entries[1].Target != "b" {
		t.Errorf("unexpected entries: %+v", entries)
	}
}
