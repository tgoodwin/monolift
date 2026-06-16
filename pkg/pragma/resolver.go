package pragma

import (
	"fmt"
	"log"

	"github.com/tgoodwin/monolift/pkg/ips"
	"github.com/tgoodwin/monolift/pkg/metrics"
)

// NewDeciderFromPragma is a factory function that creates the appropriate
// Decider implementation based on the pragma configuration.
func NewDeciderFromPragma(p *Pragma, monitor *metrics.Monitor, name string) (Decider, error) {
	if p == nil {
		return nil, fmt.Errorf("pragma cannot be nil")
	}

	switch p.SignalType {
	case CPUTrigger:
		if monitor == nil {
			return nil, fmt.Errorf("metrics monitor cannot be nil for CPU trigger")
		}
		log.Printf("Creating CPUDecider with threshold %.2f%%", p.Threshold*100)
		return NewCPUDecider(monitor, p.Threshold), nil
	case MemTrigger:
		if monitor == nil {
			return nil, fmt.Errorf("metrics monitor cannot be nil for MEM trigger")
		}
		log.Printf("Creating MemDecider with threshold %.2f%%", p.Threshold*100)
		return NewMemDecider(monitor, p.Threshold), nil
	case IPSTrigger:
		log.Printf("Creating IPSDecider for '%s' with threshold %.2f", name, p.Threshold)
		return NewIPSDecider(name, p.Threshold), nil
	default:
		return nil, fmt.Errorf("unsupported signal type for decider: %s", p.SignalType)
	}
}

// CPUDecider decides whether to delegate based on CPU usage.
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
func (d *CPUDecider) ShouldDelegate() bool {
	cpuUsage, ok := d.monitor.CPUUsagePercent()
	if !ok {
		return false
	}
	return cpuUsage > d.threshold
}

// MemDecider decides whether to delegate based on memory usage.
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
func (d *MemDecider) ShouldDelegate() bool {
	memUsage, ok := d.monitor.MemoryUsagePercent()
	if !ok {
		return false
	}
	return memUsage > d.threshold
}

// IPSDecider decides whether to delegate based on invocations per second.
type IPSDecider struct {
	name      string
	threshold float64
}

// NewIPSDecider creates a new IPS-based decider.
func NewIPSDecider(name string, threshold float64) *IPSDecider {
	return &IPSDecider{
		name:      name,
		threshold: threshold,
	}
}

// ShouldDelegate returns true if the current IPS exceeds the threshold.
func (d *IPSDecider) ShouldDelegate() bool {
	return ips.Get(d.name) > d.threshold
}
