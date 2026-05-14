package harness

import (
	"fmt"
	"sync/atomic"
	"time"
)

// StageTracker records the current execution stage for a target.
// When a timeout fires, the tracker can report which stage was active
// and how long the target has been running.
type StageTracker struct {
	target  string
	stage   atomic.Int32
	label   atomic.Value // stores string
	started time.Time
}

// NewStageTracker creates a tracker for the given target.
func NewStageTracker(target string) *StageTracker {
	st := &StageTracker{
		target:  target,
		started: time.Now(),
	}
	st.label.Store("init")
	return st
}

// Enter records entry into a numbered stage with a label.
func (st *StageTracker) Enter(stage int, label string) {
	st.stage.Store(int32(stage))
	st.label.Store(label)
}

// Current returns the current stage number and label.
func (st *StageTracker) Current() (int, string) {
	return int(st.stage.Load()), st.label.Load().(string)
}

// TimeoutMessage returns a formatted message for timeout logging.
func (st *StageTracker) TimeoutMessage(timeout time.Duration) string {
	stage, label := st.Current()
	elapsed := time.Since(st.started)
	return fmt.Sprintf("target %s timed out after %s (limit %s) in stage[%d] %s",
		st.target, elapsed.Round(time.Second), timeout.Round(time.Second), stage, label)
}

// DefaultPerTargetTimeout is the default per-target timeout for batch execution.
const DefaultPerTargetTimeout = 25 * time.Minute
