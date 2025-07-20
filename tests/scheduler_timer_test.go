// Copyright 2025 momentics@gmail.com
// Licensed under the Apache License, Version 2.0.

// scheduler_timer_test.go — Scheduler contract: timer expiration, cancel, async run.
package tests

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

func TestScheduler_DelayedExecution(t *testing.T) {
	s := concurrency.NewScheduler()
	var count int32

	s.Schedule(10_000_000, func() { atomic.AddInt32(&count, 1) }) // 10 ms

	time.Sleep(25 * time.Millisecond)
	if atomic.LoadInt32(&count) != 1 {
		t.Error("Scheduled function did not run after delay")
	}
}

func TestScheduler_Cancel(t *testing.T) {
	s := concurrency.NewScheduler()
	start := time.Now()
	c, _ := s.Schedule(50_000_000, func() { t.Error("shouldn't execute") })
	_ = s.Cancel(c)

	time.Sleep(60 * time.Millisecond)
	// No error means function was not run
	elapsed := time.Since(start)
	if elapsed < 50*time.Millisecond {
		t.Error("Sleep too short for scheduler test")
	}
}
