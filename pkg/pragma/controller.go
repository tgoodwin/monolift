package pragma

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/tgoodwin/monolift/pkg/metrics"
)

// Decider is the simple, clean interface that a delegate client uses
// to determine if a request should be processed locally or offloaded to the
// remote service.
type Decider interface {
	// ShouldDelegate returns true if the request should be sent to the remote service.
	// This method is designed to be called in the hot path of a request and
	// should be a fast, non-blocking read of the controller's current state.
	ShouldDelegate() bool
}

// ThresholdController is a stateful implementation of OffloadDecisionMaker.
// It monitors a specific metric (CPU or Memory) against a threshold defined
// in a Pragma and implements hysteresis to avoid rapidly switching between
// local and remote execution (flapping).
type ThresholdController struct {
	pragma         *Pragma
	metricsMonitor *metrics.Monitor

	// Configuration for hysteresis
	triggerDuration time.Duration // Time usage must be above threshold to start offloading.
	resetDuration   time.Duration // Time usage must be below resetThreshold to stop offloading.
	resetThreshold  float64       // A lower threshold to stop offloading (e.g., 0.9 * pragma.Threshold).

	// Internal state, protected by a mutex
	mu                  sync.RWMutex
	isOffloading        bool      // The final decision: true if we are offloading.
	thresholdExceededAt time.Time // When the trigger threshold was first continuously exceeded.
	resetConditionMetAt time.Time // When the reset threshold was first continuously met.

	// For managing the background polling goroutine
	ctx    context.Context
	cancel context.CancelFunc
}

// NewThresholdController creates and starts a new controller.
// It launches a background goroutine to periodically poll metrics and update its
// internal state. The caller is responsible for calling Close() when done.
func NewThresholdController(
	p *Pragma,
	monitor *metrics.Monitor,
	pollInterval time.Duration,
	triggerDuration time.Duration,
) *ThresholdController {
	ctx, cancel := context.WithCancel(context.Background())

	// A sensible default for the reset threshold is 90% of the trigger threshold.
	resetThreshold := p.Threshold * 0.9

	c := &ThresholdController{
		pragma:          p,
		metricsMonitor:  monitor,
		triggerDuration: triggerDuration,
		resetDuration:   triggerDuration, // Can be configured separately if needed
		resetThreshold:  resetThreshold,
		ctx:             ctx,
		cancel:          cancel,
	}

	go c.pollAndUpdateState(pollInterval)

	return c
}

// ShouldDelegate provides a fast, thread-safe read of the controller's decision.
func (c *ThresholdController) ShouldDelegate() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isOffloading
}

// Close stops the background polling goroutine.
func (c *ThresholdController) Close() {
	c.cancel()
}

// pollAndUpdateState runs in the background, periodically checking metrics
// and applying the hysteresis logic to update the `isOffloading` flag.
func (c *ThresholdController) pollAndUpdateState(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Starting pragma controller for trigger=%s, threshold=%.2f", c.pragma.SignalType, c.pragma.Threshold)

	for {
		select {
		case <-ticker.C:
			c.updateState()
		case <-c.ctx.Done():
			log.Printf("Stopping pragma controller for trigger=%s", c.pragma.SignalType)
			return
		}
	}
}

// updateState contains the core state machine logic for hysteresis.
// This function is NOT thread-safe and should only be called from the single
// background polling goroutine.
func (c *ThresholdController) updateState() {
	// TODO: Implement the state machine logic here as brainstormed.
	// 1. Get the correct metric (CPU % or Mem %) from c.metricsMonitor.
	// 2. Lock the mutex: c.mu.Lock()
	// 3. Check if we are currently in `isOffloading` state or not.
	// 4. Apply the trigger/reset logic with durations.
	// 5. Update `c.isOffloading` if state changes and log the change.
	// 6. Unlock the mutex: c.mu.Unlock()
}
