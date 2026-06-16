package ips

import (
	"sync"
	"time"
)

const (
	// window is the duration over which to calculate IPS.
	window = 5 * time.Second
	// updateInterval is how often the background worker calculates the IPS values.
	updateInterval = 1 * time.Second
)

// Monitor tracks invocations per second for different named entities.
// It uses a background goroutine to periodically calculate IPS values,
// making reads from GetIPS fast and non-blocking.
type Monitor struct {
	// mu protects access to both invocations and ipsValues maps.
	mu sync.RWMutex
	// invocations stores counts per second (unix timestamp) for each named entity.
	invocations map[string]map[int64]int
	// ipsValues stores the latest calculated IPS value for each named entity.
	ipsValues map[string]float64
	// ticker triggers the periodic calculation.
	ticker *time.Ticker
	// done is used to signal the background goroutine to stop.
	done chan struct{}
}

// NewMonitor creates a new IPS monitor and starts its background calculation worker.
func NewMonitor() *Monitor {
	m := &Monitor{
		invocations: make(map[string]map[int64]int),
		ipsValues:   make(map[string]float64),
		ticker:      time.NewTicker(updateInterval),
		done:        make(chan struct{}),
	}
	go m.runCalculator()
	return m
}

// RecordInvocation records a single invocation for the given name.
// It increments a counter for the current second.
func (m *Monitor) RecordInvocation(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	if _, ok := m.invocations[name]; !ok {
		m.invocations[name] = make(map[int64]int)
	}
	m.invocations[name][now]++
}

// GetIPS returns the most recently calculated invocations per second for the given name.
// This is a fast read operation.
func (m *Monitor) GetIPS(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// Return the pre-calculated value.
	return m.ipsValues[name]
}

// Close stops the background worker.
func (m *Monitor) Close() {
	close(m.done)
}

// runCalculator is the background worker loop.
func (m *Monitor) runCalculator() {
	for {
		select {
		case <-m.ticker.C:
			m.calculateAll()
		case <-m.done:
			m.ticker.Stop()
			return
		}
	}
}

// calculateAll iterates through all monitored entities to update their IPS
// and clean up old data.
func (m *Monitor) calculateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().Unix()
	// Cutoff for invocations to include in the window.
	cutoff := now - int64(window.Seconds())

	for name, buckets := range m.invocations {
		var totalInvocationsInWindow int
		// Sum invocations within the window and clean up old buckets.
		for timestamp, count := range buckets {
			if timestamp >= cutoff {
				totalInvocationsInWindow += count
			} else {
				// This timestamp bucket is outside the window, so delete it.
				delete(buckets, timestamp)
			}
		}
		// Update the calculated IPS value for the current entity.
		m.ipsValues[name] = float64(totalInvocationsInWindow) / window.Seconds()
	}
}

// --- Global Monitor Instance ---

var globalMonitor = NewMonitor()

// Record records an invocation for a globally accessible monitor.
func Record(name string) {
	globalMonitor.RecordInvocation(name)
}

// Get returns the IPS for a globally accessible monitor.
func Get(name string) float64 {
	return globalMonitor.GetIPS(name)
}
