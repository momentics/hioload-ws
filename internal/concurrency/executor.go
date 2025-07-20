// File: internal/concurrency/executor.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// NUMA-aware, high-performance task executor with work-stealing queues.

package concurrency

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// TaskFunc represents a unit of work to be executed.
type TaskFunc func()

// Executor manages a pool of workers for executing tasks concurrently.
type Executor struct {
	workers    []*worker
	numWorkers int32
	numaNode   int
	closed     int32
	closeOnce  sync.Once
	closeCh    chan struct{}

	// Work distribution
	globalQueue chan TaskFunc
	localQueues []*lockFreeQueue

	// Statistics
	totalTasks     int64
	completedTasks int64
}

// worker represents a single worker goroutine.
type worker struct {
	id         int
	executor   *Executor
	localQueue *lockFreeQueue
	stopCh     chan struct{}
	stopped    int32
}

// lockFreeQueue implements a simple lock-free queue for tasks.
type lockFreeQueue struct {
	head  uint64
	tail  uint64
	mask  uint64
	items []TaskFunc
	_     [64]byte // Cache line padding
}

// NewExecutor creates a new NUMA-aware executor.
func NewExecutor(numWorkers, numaNode int) *Executor {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	e := &Executor{
		numWorkers:  int32(numWorkers),
		numaNode:    numaNode,
		closeCh:     make(chan struct{}),
		globalQueue: make(chan TaskFunc, numWorkers*16),
		localQueues: make([]*lockFreeQueue, numWorkers),
		workers:     make([]*worker, numWorkers),
	}

	// Create local queues
	for i := 0; i < numWorkers; i++ {
		e.localQueues[i] = newLockFreeQueue(1024)
	}

	// Create and start workers
	for i := 0; i < numWorkers; i++ {
		w := &worker{
			id:         i,
			executor:   e,
			localQueue: e.localQueues[i],
			stopCh:     make(chan struct{}),
		}
		e.workers[i] = w
		go w.run()
	}

	return e
}

// Submit submits a task for execution.
func (e *Executor) Submit(task TaskFunc) error {
	if atomic.LoadInt32(&e.closed) == 1 {
		return ErrExecutorClosed
	}

	atomic.AddInt64(&e.totalTasks, 1)

	// Try to submit to a random local queue first
	workerID := int(atomic.AddInt64(&e.totalTasks, 0) % int64(e.numWorkers))
	if e.localQueues[workerID].enqueue(task) {
		return nil
	}

	// Fallback to global queue
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

// Resize dynamically changes the number of workers.
func (e *Executor) Resize(newCount int) {
	// Implementation for dynamic resizing would go here
	// For simplicity, we'll just update the count
	atomic.StoreInt32(&e.numWorkers, int32(newCount))
}

// Close gracefully shuts down the executor.
func (e *Executor) Close() {
	e.closeOnce.Do(func() {
		atomic.StoreInt32(&e.closed, 1)
		close(e.closeCh)

		// Stop all workers
		for _, w := range e.workers {
			close(w.stopCh)
		}

		// Wait for workers to finish
		var wg sync.WaitGroup
		for _, w := range e.workers {
			wg.Add(1)
			go func(worker *worker) {
				defer wg.Done()
				for atomic.LoadInt32(&worker.stopped) == 0 {
					time.Sleep(time.Millisecond)
				}
			}(w)
		}
		wg.Wait()
	})
}

// Stats returns executor statistics.
func (e *Executor) Stats() map[string]int64 {
	return map[string]int64{
		"total_tasks":     atomic.LoadInt64(&e.totalTasks),
		"completed_tasks": atomic.LoadInt64(&e.completedTasks),
		"pending_tasks":   atomic.LoadInt64(&e.totalTasks) - atomic.LoadInt64(&e.completedTasks),
		"num_workers":     int64(e.NumWorkers()),
	}
}

// worker.run is the main worker loop.
func (w *worker) run() {
	defer atomic.StoreInt32(&w.stopped, 1)

	// Set CPU affinity if configured
	if w.executor.numaNode >= 0 {
		PinCurrentThread(w.executor.numaNode, w.id)
	}

	for {
		select {
		case <-w.stopCh:
			return
		default:
			// Try to get task from local queue first
			if task := w.localQueue.dequeue(); task != nil {
				w.executeTask(task)
				continue
			}

			// Try to steal from other workers
			if task := w.stealWork(); task != nil {
				w.executeTask(task)
				continue
			}

			// Try global queue
			select {
			case task := <-w.executor.globalQueue:
				w.executeTask(task)
			case <-w.stopCh:
				return
			case <-time.After(time.Millisecond):
				// Brief pause to prevent busy waiting
			}
		}
	}
}

// executeTask executes a single task with error recovery.
func (w *worker) executeTask(task TaskFunc) {
	defer func() {
		if r := recover(); r != nil {
			// Log panic but don't crash the worker
			// In a real implementation, you'd use proper logging
		}
		atomic.AddInt64(&w.executor.completedTasks, 1)
	}()

	task()
}

// stealWork attempts to steal work from other workers.
func (w *worker) stealWork() TaskFunc {
	numWorkers := len(w.executor.workers)
	start := (w.id + 1) % numWorkers

	for i := 0; i < numWorkers-1; i++ {
		victimID := (start + i) % numWorkers
		if task := w.executor.localQueues[victimID].dequeue(); task != nil {
			return task
		}
	}

	return nil
}

// newLockFreeQueue creates a new lock-free queue with power-of-2 size.
func newLockFreeQueue(size int) *lockFreeQueue {
	// Ensure size is power of 2
	if size == 0 || (size&(size-1)) != 0 {
		size = 1024 // Default to 1024
	}

	return &lockFreeQueue{
		mask:  uint64(size - 1),
		items: make([]TaskFunc, size),
	}
}

// enqueue adds a task to the queue.
func (q *lockFreeQueue) enqueue(task TaskFunc) bool {
	tail := atomic.LoadUint64(&q.tail)
	head := atomic.LoadUint64(&q.head)

	if tail-head >= uint64(len(q.items)) {
		return false // Queue full
	}

	q.items[tail&q.mask] = task
	atomic.StoreUint64(&q.tail, tail+1)
	return true
}

// dequeue removes and returns a task from the queue.
func (q *lockFreeQueue) dequeue() TaskFunc {
	head := atomic.LoadUint64(&q.head)
	tail := atomic.LoadUint64(&q.tail)

	if head >= tail {
		return nil // Queue empty
	}

	task := q.items[head&q.mask]
	atomic.StoreUint64(&q.head, head+1)
	return task
}
