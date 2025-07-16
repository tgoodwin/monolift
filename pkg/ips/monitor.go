package ips

import (
	"sync"
	"time"
)

const (
	// window is the duration over which to calculate IPS.
	window = 10 * time.Second
)

// Monitor tracks invocations per second for different named entities (functions or services).
type Monitor struct {
	mu          sync.Mutex
	invocations map[string][]time.Time
}

// NewMonitor creates a new IPS monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		invocations: make(map[string][]time.Time),
	}
}

// RecordInvocation records a single invocation for the given name.
func (m *Monitor) RecordInvocation(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.invocations[name] = append(m.invocations[name], now)
	m.cleanup(name, now)
}

// GetIPS returns the invocations per second for the given name over the last `window` duration.
func (m *Monitor) GetIPS(name string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.cleanup(name, now)

	invocationsInWindow := len(m.invocations[name])
	return float64(invocationsInWindow) / window.Seconds()
}

// cleanup removes timestamps that are older than the window.
// This must be called with the mutex held.
func (m *Monitor) cleanup(name string, now time.Time) {
	if invs, ok := m.invocations[name]; ok {
		cutoff := now.Add(-window)

		// Find the first index that is not older than the window.
		// This is faster than re-slicing in a loop for long slices.
		firstValidIndex := 0
		for i, ts := range invs {
			if !ts.Before(cutoff) {
				firstValidIndex = i
				break
			}
			// If the last element is still before the cutoff, all elements are old.
			if i == len(invs)-1 {
				m.invocations[name] = []time.Time{}
				return
			}
		}
		m.invocations[name] = invs[firstValidIndex:]
	}
}

// Global monitor instance
var globalMonitor = NewMonitor()

// Record records an invocation for a globally accessible monitor.
func Record(name string) {
	globalMonitor.RecordInvocation(name)
}

// Get returns the IPS for a globally accessible monitor.
func Get(name string) float64 {
	return globalMonitor.GetIPS(name)
}
