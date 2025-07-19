// control/metrics.go
// Author: momentics <momentics@gmail.com>
//
// Runtime metrics collector for system-level monitoring.
// Exposes counters in a thread-safe map with dynamic registration.

package control

import (
	"sync"
	"time"
)

// MetricsRegistry holds mutable and read-only metrics.
type MetricsRegistry struct {
	mu      sync.RWMutex
	metrics map[string]any
	updated time.Time
}

// NewMetricsRegistry creates an empty registry.
func NewMetricsRegistry() *MetricsRegistry {
	return &MetricsRegistry{
		metrics: make(map[string]any),
	}
}

// Set sets or updates a metric key.
func (mr *MetricsRegistry) Set(key string, value any) {
	mr.mu.Lock()
	mr.metrics[key] = value
	mr.updated = time.Now()
	mr.mu.Unlock()
}

// GetSnapshot returns the latest metrics.
func (mr *MetricsRegistry) GetSnapshot() map[string]any {
	mr.mu.RLock()
	defer mr.mu.RUnlock()
	out := make(map[string]any, len(mr.metrics))
	for k, v := range mr.metrics {
		out[k] = v
	}
	return out
}
