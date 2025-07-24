// File: facade/scheduler.go
// Package facade exposes Scheduler for heartbeat and timeouts.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package server

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// NewScheduler returns a Scheduler instance for timed tasks.
func NewScheduler() api.Scheduler {
	return concurrency.NewScheduler()
}
