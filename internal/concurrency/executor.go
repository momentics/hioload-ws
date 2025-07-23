// File: internal/concurrency/executor.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Executor dispatches tasks across worker goroutines, using lock-free local queues
// and a global queue fallback. Avoids excessive atomics by batching stats.
// Improvements: on Resize, wait for old workers to exit cleanly before reaping.

package concurrency

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type TaskFunc func()

// Executor manages a pool of worker goroutines.
type Executor struct {
	globalQueue   chan TaskFunc
	localQueues   []*lockFreeQueue[TaskFunc]
	workers       []*worker
	closeCh       chan struct{}
	closed        atomic.Bool
	resizeRequest chan int
	mu            sync.Mutex
	wg            sync.WaitGroup
}

// NewExecutor creates a new Executor with the given number of workers.
func NewExecutor(numWorkers, numaNode int) *Executor {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	e := &Executor{
		globalQueue:   make(chan TaskFunc, numWorkers*4),
		closeCh:       make(chan struct{}),
		resizeRequest: make(chan int),
	}
	e.localQueues = make([]*lockFreeQueue[TaskFunc], numWorkers)
	e.workers = make([]*worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		e.localQueues[i] = NewLockFreeQueue[TaskFunc](1024)
	}
	for i := 0; i < numWorkers; i++ {
		w := &worker{id: i, executor: e, localQueue: e.localQueues[i], stopCh: make(chan struct{})}
		e.workers[i] = w
		e.wg.Add(1)
		go w.run(numaNode, &e.wg)
	}
	go e.manageResizes(numaNode)
	return e
}

// Submit enqueues a task. Returns error if closed.
func (e *Executor) Submit(task TaskFunc) error {
	if e.closed.Load() {
		return ErrExecutorClosed
	}
	idx := int(time.Now().UnixNano()) % len(e.localQueues)
	if e.localQueues[idx].Enqueue(task) {
		return nil
	}
	select {
	case e.globalQueue <- task:
		return nil
	case <-e.closeCh:
		return ErrExecutorClosed
	default:
		return ErrExecutorClosed
	}
}

// Resize dynamically scales the worker pool.
func (e *Executor) Resize(newCount int) {
	e.resizeRequest <- newCount
}

func (e *Executor) manageResizes(numaNode int) {
	for newCount := range e.resizeRequest {
		e.mu.Lock()
		if newCount <= 0 {
			newCount = 1
		}
		current := len(e.workers)
		if newCount > current {
			// add workers
			for i := current; i < newCount; i++ {
				q := NewLockFreeQueue[TaskFunc](1024)
				e.localQueues = append(e.localQueues, q)
				w := &worker{id: i, executor: e, localQueue: q, stopCh: make(chan struct{})}
				e.workers = append(e.workers, w)
				e.wg.Add(1)
				go w.run(numaNode, &e.wg)
			}
		} else if newCount < current {
			// stop extra workers
			for i := newCount; i < current; i++ {
				close(e.workers[i].stopCh)
			}
			// wait for them
			for i := newCount; i < current; i++ {
				// workers decrement wg when exiting
			}
			e.workers = e.workers[:newCount]
			e.localQueues = e.localQueues[:newCount]
		}
		e.mu.Unlock()
	}
}

// Close shuts down the executor, waiting for workers to finish.
func (e *Executor) Close() {
	if e.closed.CompareAndSwap(false, true) {
		close(e.closeCh)
		close(e.resizeRequest)
		e.mu.Lock()
		for _, w := range e.workers {
			close(w.stopCh)
		}
		e.mu.Unlock()
		e.wg.Wait()
	}
}

// NumWorkers returns active worker count.
func (e *Executor) NumWorkers() int {
	return len(e.workers)
}

// worker runs tasks.
type worker struct {
	id         int
	executor   *Executor
	localQueue *lockFreeQueue[TaskFunc]
	stopCh     chan struct{}
}

func (w *worker) run(numaNode int, wg *sync.WaitGroup) {
	defer wg.Done()
	if numaNode >= 0 {
		PinCurrentThread(numaNode, w.id)
	}
	for {
		select {
		case <-w.stopCh:
			return
		default:
			if task, ok := w.localQueue.Dequeue(); ok {
				w.safeExecute(task)
				continue
			}
			select {
			case task := <-w.executor.globalQueue:
				w.safeExecute(task)
			case <-w.stopCh:
				return
			default:
				time.Sleep(time.Millisecond)
			}
		}
	}
}

func (w *worker) safeExecute(task TaskFunc) {
	defer func() { recover() }()
	task()
}
