package metrics

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Monitor periodically polls for cgroup metrics and caches the latest values.
// This provides fast, non-blocking access to recent resource consumption data.
// It also reads K8s resource limits to provide usage as a percentage.
type Monitor struct {
	cgroupReader *CgroupReader

	// Cached usage values
	cpuUsage    float64 // in cores
	memoryUsage uint64  // in bytes

	// Resource limits from environment
	cpuLimitCores    float64
	memoryLimitBytes uint64

	mu        sync.RWMutex
	hasPolled bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMonitor creates and starts a new Monitor.
// It reads K8s resource limits from environment variables and will return an
// error if they are not present.
// It begins polling for metrics immediately in the background at the specified
// interval. The Close method must be called to stop the background polling
// and release resources.
func NewMonitor(interval time.Duration) (*Monitor, error) {
	reader, err := NewCgroupReader()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cgroup reader: %w", err)
	}

	cpuLimit, err := CPULimitFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to read CPU limit: %w", err)
	}

	memLimit, err := MemoryLimitFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to read memory limit: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	monitor := &Monitor{
		cgroupReader:     reader,
		cpuLimitCores:    cpuLimit,
		memoryLimitBytes: memLimit,
		ctx:              ctx,
		cancel:           cancel,
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

// CPUUsagePercent returns the CPU usage as a percentage of the configured limit.
// The boolean return value is true if at least one poll has successfully completed.
// If the CPU limit is zero, it returns 0.
func (m *Monitor) CPUUsagePercent() (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.hasPolled {
		return 0, false
	}
	if m.cpuLimitCores == 0 {
		return 0, true // Limit is 0, so usage is technically 0% of an unconstrained resource.
	}

	percent := (m.cpuUsage / m.cpuLimitCores) * 100.0
	return percent, true
}

// MemoryUsage returns the most recently polled memory usage in bytes.
// The boolean return value is true if at least one poll has successfully completed.
func (m *Monitor) MemoryUsage() (uint64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.memoryUsage, m.hasPolled
}

// MemoryUsagePercent returns the memory usage as a percentage of the configured limit.
// The boolean return value is true if at least one poll has successfully completed.
// If the memory limit is zero, it returns 0.
func (m *Monitor) MemoryUsagePercent() (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.hasPolled {
		return 0, false
	}
	if m.memoryLimitBytes == 0 {
		return 0, true
	}

	percent := (float64(m.memoryUsage) / float64(m.memoryLimitBytes)) * 100.0
	return percent, true
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
			cpuPercent, _ := m.CPUUsagePercent()
			memPercent, _ := m.MemoryUsagePercent()

			if cpuPolled || memPolled {
				fmt.Printf(
					"CPU: %.2f/%.2f cores (%.1f%%) | Memory: %.2f/%.2f MiB (%.1f%%)\n",
					cpu, m.cpuLimitCores, cpuPercent,
					float64(mem)/(1024*1024), float64(m.memoryLimitBytes)/(1024*1024), memPercent,
				)
			} else {
				println("No metrics available yet.")
			}
		case <-m.ctx.Done():
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
