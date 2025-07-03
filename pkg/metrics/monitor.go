package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor periodically polls for cgroup metrics and caches the latest values.
// This provides fast, non-blocking access to recent resource consumption data.
type Monitor struct {
	cgroupReader *CgroupReader
	cpuUsage     float64
	memoryUsage  uint64
	mu           sync.RWMutex
	hasPolled    bool
	cancel       context.CancelFunc
}

// NewMonitor creates and starts a new MetricsMonitor.
// It begins polling for metrics immediately in the background at the specified
// interval. The Close method must be called to stop the background polling
// and release resources.
func NewMonitor(interval time.Duration) (*Monitor, error) {
	reader, err := NewCgroupReader()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &Monitor{
		cgroupReader: reader,
		cancel:       cancel,
	}

	go monitor.poll(ctx, interval)

	return monitor, nil
}

// CPUUsage returns the most recently polled CPU usage as a ratio of cores
// (e.g., 1.5 means 1.5 cores). The boolean return value is true if at least
// one poll has successfully completed.
func (m *Monitor) CPUUsage() (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cpuUsage, m.hasPolled
}

// MemoryUsage returns the most recently polled memory usage in bytes.
// The boolean return value is true if at least one poll has successfully

func (m *Monitor) MemoryUsage() (uint64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.memoryUsage, m.hasPolled
}

// Close stops the background polling goroutine.
func (m *Monitor) Close() {
	m.cancel()
}

// poll starts a loop to periodically update metrics.
// It performs an initial poll immediately and then continues on the interval.
func (m *Monitor) poll(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Perform an initial poll immediately to populate data.
	m.updateMetrics(interval)

	for {
		select {
		case <-ticker.C:
			m.updateMetrics(interval)
		case <-ctx.Done():
			return
		}
	}
}

func (m *Monitor) PollPrint(interval time.Duration) {
	// This method is for demonstration purposes to show how to poll and print metrics.
	// It will block until the context is cancelled.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cpu, cpuPolled := m.CPUUsage()
			mem, memPolled := m.MemoryUsage()
			if cpuPolled || memPolled {
				fmt.Printf("CPU Usage: %.2f cores, Memory Usage: %d bytes\n", cpu, mem)
			} else {
				println("No metrics available yet.")
			}
		case <-context.Background().Done():
			return
		}
	}
}

// updateMetrics performs a single poll and updates the cached values.
func (m *Monitor) updateMetrics(sampleDuration time.Duration) {
	// In a production library, these errors should be logged.
	cpu, cpuErr := m.cgroupReader.CPUUsage(sampleDuration)
	mem, memErr := m.cgroupReader.MemoryUsage()

	m.mu.Lock()
	defer m.mu.Unlock()

	if cpuErr == nil {
		m.cpuUsage = cpu
	}
	if memErr == nil {
		m.memoryUsage = mem
	}

	// Mark as polled if at least one metric was successfully read.
	if !m.hasPolled && (cpuErr == nil || memErr == nil) {
		m.hasPolled = true
	}
}
