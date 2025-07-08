package pragma

import (
	"fmt"
	"log"

	"github.com/tgoodwin/monolift/pkg/metrics"
)

// NewDeciderFromPragma is a factory function that creates the appropriate
// Decider implementation based on the pragma configuration.
func NewDeciderFromPragma(p *Pragma, monitor *metrics.Monitor) (Decider, error) {
	if p == nil {
		return nil, fmt.Errorf("pragma cannot be nil")
	}
	if monitor == nil {
		return nil, fmt.Errorf("metrics monitor cannot be nil")
	}

	switch p.SignalType {
	case CPUTrigger:
		log.Printf("Creating CPUDecider with threshold %.2f%%", p.Threshold*100)
		return NewCPUDecider(monitor, p.Threshold), nil
	case MemTrigger:
		log.Printf("Creating MemDecider with threshold %.2f%%", p.Threshold*100)
		return NewMemDecider(monitor, p.Threshold), nil
	default:
		return nil, fmt.Errorf("unsupported signal type for decider: %s", p.SignalType)
	}
}

// CPUDecider decides whether to delegate based on CPU usage.
// It provides a simple, stateless check against a threshold.
type CPUDecider struct {
	monitor   *metrics.Monitor
	threshold float64
}

// NewCPUDecider creates a new CPU-based decider.
func NewCPUDecider(monitor *metrics.Monitor, threshold float64) *CPUDecider {
	return &CPUDecider{
		monitor:   monitor,
		threshold: threshold,
	}
}

// ShouldDelegate returns true if the current CPU usage exceeds the threshold.
// This is a non-blocking read of the monitor's last known state.
func (d *CPUDecider) ShouldDelegate() bool {
	// The monitor provides CPU usage as a percentage (0.0 to 1.0).
	cpuUsage, ok := d.monitor.CPUUsagePercent()
	if !ok {
		return false
	}
	return cpuUsage > d.threshold
}

// MemDecider decides whether to delegate based on memory usage.
// It provides a simple, stateless check against a threshold.
type MemDecider struct {
	monitor   *metrics.Monitor
	threshold float64
}

// NewMemDecider creates a new memory-based decider.
func NewMemDecider(monitor *metrics.Monitor, threshold float64) *MemDecider {
	return &MemDecider{
		monitor:   monitor,
		threshold: threshold,
	}
}

// ShouldDelegate returns true if the current memory usage exceeds the threshold.
// This is a non-blocking read of the monitor's last known state.
func (d *MemDecider) ShouldDelegate() bool {
	// The monitor provides memory usage as a percentage (0.0 to 1.0).
	memUsage, ok := d.monitor.MemoryUsagePercent()
	if !ok {
		return false
	}
	return memUsage > d.threshold
}
