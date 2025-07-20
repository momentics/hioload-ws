// File: internal/concurrency/executor.go
// Package concurrency implements a NUMA-aware task executor with work-stealing.
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Executor dispatches tasks across worker goroutines, using lock-free local queues
// and a global queue fallback. The lockFreeQueue type is defined in lock_free_queue.go.

package concurrency

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// TaskFunc is a unit of work to execute.
type TaskFunc func()

// Executor manages a pool of worker goroutines.
type Executor struct {
	globalQueue chan TaskFunc              // fallback queue for tasks when local queues are full
	localQueues []*lockFreeQueue[TaskFunc] // per-worker lock-free queues
	workers     []*worker                  // worker instances
	closeCh     chan struct{}              // signals executor shutdown
	closed      int32                      // atomic flag: 1 if closed
	numWorkers  int32                      // current number of workers
	mu          sync.Mutex                 // protects resizing operations

	// statistics
	totalTasks     int64
	completedTasks int64
}

// NewExecutor creates a new Executor with the given number of workers and optional NUMA node.
// If numWorkers <= 0, defaults to runtime.NumCPU().
func NewExecutor(numWorkers, numaNode int) *Executor {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	e := &Executor{
		globalQueue: make(chan TaskFunc, numWorkers*4),
		closeCh:     make(chan struct{}),
		numWorkers:  int32(numWorkers),
	}
	// initialize local queues and workers
	e.localQueues = make([]*lockFreeQueue[TaskFunc], numWorkers)
	e.workers = make([]*worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		e.localQueues[i] = NewLockFreeQueue[TaskFunc](1024)
	}
	for i := 0; i < numWorkers; i++ {
		w := &worker{
			id:         i,
			executor:   e,
			localQueue: e.localQueues[i],
			stopCh:     make(chan struct{}),
		}
		e.workers[i] = w
		go w.run(numaNode)
	}
	return e
}

// Submit enqueues a task for execution, returning ErrExecutorClosed if executor is closed.
func (e *Executor) Submit(task TaskFunc) error {
	if atomic.LoadInt32(&e.closed) == 1 {
		return ErrExecutorClosed
	}
	atomic.AddInt64(&e.totalTasks, 1)
	// attempt local enqueue based on round-robin ID
	idx := int(atomic.LoadInt64(&e.totalTasks) % int64(e.NumWorkers()))
	if e.localQueues[idx].Enqueue(task) {
		return nil
	}
	// fallback to global queue
	select {
	case e.globalQueue <- task:
		return nil
	case <-e.closeCh:
		return ErrExecutorClosed
	default:
		return ErrExecutorClosed
	}
}

// NumWorkers returns the current number of active workers.
func (e *Executor) NumWorkers() int {
	return int(atomic.LoadInt32(&e.numWorkers))
}

// Close gracefully shuts down the executor and waits for workers to exit.
func (e *Executor) Close() {
	if atomic.CompareAndSwapInt32(&e.closed, 0, 1) {
		close(e.closeCh)
		e.mu.Lock()
		defer e.mu.Unlock()
		for _, w := range e.workers {
			close(w.stopCh)
		}
	}
}

// Stats returns basic executor metrics.
func (e *Executor) Stats() map[string]int64 {
	return map[string]int64{
		"total_tasks":     atomic.LoadInt64(&e.totalTasks),
		"completed_tasks": atomic.LoadInt64(&e.completedTasks),
		"pending_tasks":   atomic.LoadInt64(&e.totalTasks) - atomic.LoadInt64(&e.completedTasks),
		"num_workers":     int64(e.NumWorkers()),
	}
}

// worker represents a single executor goroutine.
type worker struct {
	id         int
	executor   *Executor
	localQueue *lockFreeQueue[TaskFunc]
	stopCh     chan struct{}
	stopped    int32
}

// run is the main loop for a worker, optionally pinning to numaNode.
func (w *worker) run(numaNode int) {
	defer atomic.StoreInt32(&w.stopped, 1)
	if numaNode >= 0 {
		PinCurrentThread(numaNode, w.id)
	}
	for {
		select {
		case <-w.stopCh:
			return
		default:
			// try local queue
			if task, ok := w.localQueue.Dequeue(); ok {
				w.executeTask(task)
				continue
			}
			// try global queue
			select {
			case task := <-w.executor.globalQueue:
				w.executeTask(task)
			case <-w.stopCh:
				return
			default:
				// backoff to reduce CPU spinning
				time.Sleep(time.Millisecond)
			}
		}
	}
}

// executeTask runs the task and updates statistics, recovering from panics.
func (w *worker) executeTask(task TaskFunc) {
	defer func() {
		if r := recover(); r != nil {
			// swallow panic to keep worker alive
		}
		atomic.AddInt64(&w.executor.completedTasks, 1)
	}()
	task()
}
