// File: adapters/executor_adapter.go
// Package adapters provides glue between internal concurrency and api.Executor.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// ExecutorAdapter implements the api.Executor interface by delegating to the internal
// concurrency.Executor. It provides asynchronous task submission, dynamic resizing,
// and telemetry hooks, while preserving lock-free and NUMA-aware execution semantics.

package adapters

import (
	"github.com/momentics/hioload-ws/api"
	"github.com/momentics/hioload-ws/internal/concurrency"
)

// ExecutorAdapter wraps an internal concurrency.Executor to satisfy the api.Executor contract.
type ExecutorAdapter struct {
	exec *concurrency.Executor
}

// NewExecutorAdapter constructs an api.Executor with the given number of worker goroutines.
// It pins each worker thread to the configured NUMA node for locality, ensuring low latency.
func NewExecutorAdapter(workers int, numaNode int) api.Executor {
	// Create a new internal Executor: lock-free local queues + global fallback queue.
	e := concurrency.NewExecutor(workers, numaNode)
	return &ExecutorAdapter{exec: e}
}

// Submit dispatches a task function to be executed asynchronously.
// Returns an error if the executor has been closed.
func (ea *ExecutorAdapter) Submit(task func()) error {
	// Delegates to internal Executor.Submit, which enqueues in a lock-free queue.
	return ea.exec.Submit(task)
}

// NumWorkers returns the current number of active worker goroutines.
// Under the hood, this reads the length of the worker slice managed by the internal Executor.
func (ea *ExecutorAdapter) NumWorkers() int {
	return ea.exec.NumWorkers()
}

// Resize dynamically adjusts the size of the worker pool.
// Expanding or contracting the pool pins new threads to the NUMA node if provided.
func (ea *ExecutorAdapter) Resize(newCount int) {
	ea.exec.Resize(newCount)
}

// Close shuts down the executor, signaling all workers to exit and waiting for completion.
// This method ensures a graceful teardown: all submitted tasks are either executed or discarded safely.
func (ea *ExecutorAdapter) Close() {
	ea.exec.Close()
}
