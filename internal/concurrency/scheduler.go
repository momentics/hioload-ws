// File: internal/concurrency/scheduler.go
// Package concurrency implements a simple Scheduler for timed tasks.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0

package concurrency

import (
	"sync"
	"time"

	"github.com/momentics/hioload-ws/api"
)

// scheduler implements api.Scheduler.
type scheduler struct {
	mu      sync.Mutex
	timers  map[*time.Timer]struct{}
	running bool
}

// NewScheduler creates a new Scheduler.
func NewScheduler() api.Scheduler {
	return &scheduler{
		timers: make(map[*time.Timer]struct{}),
	}
}

// Schedule registers a function to be executed after delayNanos.
// Returns a Cancelable for the scheduled task.
func (s *scheduler) Schedule(delayNanos int64, fn func()) (api.Cancelable, error) {
	timer := time.NewTimer(time.Duration(delayNanos))
	s.mu.Lock()
	s.timers[timer] = struct{}{}
	s.mu.Unlock()
	done := make(chan struct{})
	go func() {
		select {
		case <-timer.C:
			fn()
		case <-done:
			// cancelled
		}
		s.mu.Lock()
		delete(s.timers, timer)
		s.mu.Unlock()
	}()
	return &schedCancelable{timer: timer, done: done}, nil
}

// Cancel removes a previously scheduled task.
func (s *scheduler) Cancel(c api.Cancelable) error {
	if sc, ok := c.(*schedCancelable); ok {
		sc.cancel()
		s.mu.Lock()
		delete(s.timers, sc.timer)
		s.mu.Unlock()
	}
	return nil
}

// Now returns the current monotonic nanosecond time.
func (s *scheduler) Now() int64 {
	return time.Now().UnixNano()
}

type schedCancelable struct {
	timer *time.Timer
	done  chan struct{}
}

func (c *schedCancelable) Cancel() error {
	c.cancel()
	return nil
}

func (c *schedCancelable) cancel() {
	if c.timer.Stop() {
		close(c.done)
	}
}

func (c *schedCancelable) Done() <-chan struct{} {
	return c.done
}

func (c *schedCancelable) Err() error {
	// no error state
	return nil
}
