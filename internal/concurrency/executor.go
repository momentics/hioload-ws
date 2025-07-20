// File: internal/concurrency/executor.go
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// NUMA-aware, high-performance task executor with work-stealing queues.
// Implements dynamic resizing of worker pool with graceful shutdown of excess workers
// and incremental scaling up without disrupting ongoing tasks.

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
	workers    []*worker     // slice of worker instances
	numWorkers int32         // current number of active workers
	numaNode   int           // preferred NUMA node for affinity
	closed     int32         // atomic flag: 1 if executor is closed
	closeOnce  sync.Once     // ensure Close only runs once
	closeCh    chan struct{} // channel to signal executor shutdown

	globalQueue chan TaskFunc    // fallback queue when local queues are full
	localQueues []*lockFreeQueue // per-worker local queues for work-stealing

	resizeMu sync.Mutex // protects resizing operations

	// Statistics
	totalTasks     int64
	completedTasks int64
}

// NewExecutor creates a new NUMA-aware executor.
// numWorkers: initial worker count (<=0 defaults to runtime.NumCPU()).
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

	// initialize per-worker local queues
	for i := 0; i < numWorkers; i++ {
		e.localQueues[i] = newLockFreeQueue(1024)
	}

	// start worker goroutines
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
// It will attempt local queue, then global, and return ErrExecutorClosed if closed.
func (e *Executor) Submit(task TaskFunc) error {
	if atomic.LoadInt32(&e.closed) == 1 {
		return ErrExecutorClosed
	}
	atomic.AddInt64(&e.totalTasks, 1)

	// try local queue of a worker based on round-robin
	idx := int(atomic.AddInt64(&e.totalTasks, 0) % int64(e.NumWorkers()))
	if e.localQueues[idx].enqueue(task) {
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

// Resize dynamically adjusts the worker pool size to newCount.
// - If newCount > current: spawn additional workers.
// - If newCount < current: gracefully stop excess workers.
// This method is thread-safe and blocks until resize is complete.
func (e *Executor) Resize(newCount int) {
	if newCount <= 0 {
		return
	}
	e.resizeMu.Lock()
	defer e.resizeMu.Unlock()

	current := int(atomic.LoadInt32(&e.numWorkers))
	if newCount == current {
		return
	}

	if newCount > current {
		// scale up: create and start new workers
		add := newCount - current
		for i := 0; i < add; i++ {
			idx := current + i
			// expand localQueues and workers slice
			e.localQueues = append(e.localQueues, newLockFreeQueue(1024))
			w := &worker{
				id:         idx,
				executor:   e,
				localQueue: e.localQueues[idx],
				stopCh:     make(chan struct{}),
			}
			e.workers = append(e.workers, w)
			go w.run()
		}
		atomic.StoreInt32(&e.numWorkers, int32(newCount))
	} else {
		// scale down: signal excess workers to stop
		remove := current - newCount
		// workers to stop: last 'remove' entries
		for i := 0; i < remove; i++ {
			w := e.workers[current-1-i]
			close(w.stopCh) // signal worker to exit
		}
		// wait for them to stop
		for i := 0; i < remove; i++ {
			w := e.workers[current-1-i]
			for atomic.LoadInt32(&w.stopped) == 0 {
				time.Sleep(time.Millisecond)
			}
		}
		// truncate slices
		e.workers = e.workers[:newCount]
		e.localQueues = e.localQueues[:newCount]
		atomic.StoreInt32(&e.numWorkers, int32(newCount))
	}
}

// Close gracefully shuts down the executor and all workers.
func (e *Executor) Close() {
	e.closeOnce.Do(func() {
		atomic.StoreInt32(&e.closed, 1)
		close(e.closeCh)
		// stop all workers
		e.resizeMu.Lock()
		for _, w := range e.workers {
			close(w.stopCh)
		}
		e.resizeMu.Unlock()
		// wait for workers to exit
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

// worker represents a single worker goroutine.
type worker struct {
	id         int
	executor   *Executor
	localQueue *lockFreeQueue
	stopCh     chan struct{}
	stopped    int32 // atomic flag: 1 when stopped
}

// run is the main loop for each worker.
func (w *worker) run() {
	defer atomic.StoreInt32(&w.stopped, 1)
	// pin this OS thread to NUMA node and CPU
	if w.executor.numaNode >= 0 {
		PinCurrentThread(w.executor.numaNode, w.id)
	}
	for {
		select {
		case <-w.stopCh:
			return
		default:
			// try local queue
			if task := w.localQueue.dequeue(); task != nil {
				w.executeTask(task)
				continue
			}
			// try stealing from others
			if task := w.stealWork(); task != nil {
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
				// backoff to avoid busy spinning
				time.Sleep(time.Millisecond)
			}
		}
	}
}

// executeTask runs the task with panic recovery and updates completed count.
func (w *worker) executeTask(task TaskFunc) {
	defer func() {
		if r := recover(); r != nil {
			// swallow panic to keep worker alive
		}
		atomic.AddInt64(&w.executor.completedTasks, 1)
	}()
	task()
}

// stealWork attempts to steal a task from other workers' local queues.
func (w *worker) stealWork() TaskFunc {
	num := w.executor.NumWorkers()
	start := (w.id + 1) % num
	for i := 0; i < num-1; i++ {
		victim := (start + i) % num
		if task := w.executor.localQueues[victim].dequeue(); task != nil {
			return task
		}
	}
	return nil
}

// lockFreeQueue is a simple ring buffer based lock-free queue for TaskFunc.
type lockFreeQueue struct {
	head  uint64
	tail  uint64
	mask  uint64
	items []TaskFunc
}

// newLockFreeQueue creates a new lock-free queue with power-of-two capacity.
func newLockFreeQueue(size int) *lockFreeQueue {
	if size == 0 || (size&(size-1)) != 0 {
		size = 1024
	}
	return &lockFreeQueue{
		mask:  uint64(size - 1),
		items: make([]TaskFunc, size),
	}
}

// enqueue adds a task to the queue. Returns false if full.
func (q *lockFreeQueue) enqueue(task TaskFunc) bool {
	tail := atomic.LoadUint64(&q.tail)
	head := atomic.LoadUint64(&q.head)
	if tail-head >= uint64(len(q.items)) {
		return false
	}
	q.items[tail&q.mask] = task
	atomic.StoreUint64(&q.tail, tail+1)
	return true
}

// dequeue removes and returns a task from the queue. Returns nil if empty.
func (q *lockFreeQueue) dequeue() TaskFunc {
	head := atomic.LoadUint64(&q.head)
	tail := atomic.LoadUint64(&q.tail)
	if head >= tail {
		return nil
	}
	task := q.items[head&q.mask]
	atomic.StoreUint64(&q.head, head+1)
	return task
}
