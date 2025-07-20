// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// executor_resize_test.go — Resizing and dynamic stress-test for NUMA-aware Executor.
package tests

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/momentics/hioload-ws/internal/concurrency"
)

func TestExecutor_Resize_Basic(t *testing.T) {
	ex := concurrency.NewExecutor(4, -1)
	defer ex.Close()

	var counter int64
	task := func() { atomic.AddInt64(&counter, 1) }

	// Submit initial tasks and check execution
	for i := 0; i < 20; i++ {
		_ = ex.Submit(task)
	}
	time.Sleep(100 * time.Millisecond)
	a := atomic.LoadInt64(&counter)
	if a == 0 {
		t.Fatal("Tasks not executed")
	}

	// Resize pattern (simulate Up/Down)
	ex.Resize(8)
	for i := 0; i < 100; i++ {
		_ = ex.Submit(task)
	}
	ex.Resize(2)
	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt64(&counter) < a+20 {
		t.Error("Some tasks lost during resize")
	}
}
