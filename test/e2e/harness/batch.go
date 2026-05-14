package harness

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// BatchStatus represents the outcome of a single target in a batch run.
type BatchStatus string

const (
	BatchPass          BatchStatus = "pass"
	BatchAdmissionSkip BatchStatus = "admission-skip"
	BatchBuildSkip     BatchStatus = "build-skip"
	BatchE2EFail       BatchStatus = "e2e-fail"
	BatchTimeoutSkip   BatchStatus = "timeout-skip"
	BatchManifestSkip  BatchStatus = "manifest-skip"
	BatchInfraFail     BatchStatus = "infra-fail"
	BatchSkipped       BatchStatus = "skipped"
)

// BatchEntry records the outcome of a single target.
type BatchEntry struct {
	Target   string
	Status   BatchStatus
	Stage    string
	Duration time.Duration
	Error    string
}

// BatchResult collects per-target results and prints a summary table.
type BatchResult struct {
	mu      sync.Mutex
	entries []BatchEntry
}

// Record adds a result entry. Safe for concurrent use.
func (b *BatchResult) Record(entry BatchEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, entry)
}

// Entries returns a copy of all recorded entries.
func (b *BatchResult) Entries() []BatchEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]BatchEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

// SummaryTable returns a formatted summary table suitable for test output.
func (b *BatchResult) SummaryTable() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.entries) == 0 {
		return "No batch results recorded."
	}

	// Count statuses.
	counts := make(map[BatchStatus]int)
	for _, e := range b.entries {
		counts[e.Status]++
	}

	var sb strings.Builder
	sb.WriteString("\n╔══════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    BATCH RESULT SUMMARY                        ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")

	// Status summary line.
	sb.WriteString(fmt.Sprintf("║  Total: %d", len(b.entries)))
	for _, status := range []BatchStatus{BatchPass, BatchE2EFail, BatchAdmissionSkip, BatchBuildSkip, BatchTimeoutSkip, BatchInfraFail, BatchManifestSkip, BatchSkipped} {
		if c := counts[status]; c > 0 {
			sb.WriteString(fmt.Sprintf("  |  %s: %d", status, c))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")
	sb.WriteString("║  TARGET                                  STATUS       DURATION  ║\n")
	sb.WriteString("╠══════════════════════════════════════════════════════════════════╣\n")

	for _, e := range b.entries {
		name := e.Target
		if len(name) > 40 {
			name = name[:37] + "..."
		}
		dur := formatDuration(e.Duration)
		line := fmt.Sprintf("║  %-40s %-12s %8s  ║", name, e.Status, dur)
		sb.WriteString(line)
		sb.WriteString("\n")
		if e.Error != "" {
			errMsg := e.Error
			if len(errMsg) > 58 {
				errMsg = errMsg[:55] + "..."
			}
			sb.WriteString(fmt.Sprintf("║    → %s\n", errMsg))
		}
	}

	sb.WriteString("╚══════════════════════════════════════════════════════════════════╝\n")
	return sb.String()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}
