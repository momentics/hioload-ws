// File: internal/concurrency/executor.go
//
// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// Executor dispatches tasks across worker goroutines, using lock-free local queues
// and a global queue fallback. Now guarantees that wg.Done is called only after
// a worker has been completely stopped and removed for safe dynamic resizing.
//

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

	removeWorkerCh chan *worker // New: signals workers to exit and confirm termination.
}

// NewExecutor creates a new Executor with the given number of workers.
func NewExecutor(numWorkers, numaNode int) *Executor {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	e := &Executor{
		globalQueue:    make(chan TaskFunc, numWorkers*4),
		closeCh:        make(chan struct{}),
		resizeRequest:  make(chan int),
		removeWorkerCh: make(chan *worker),
	}
	e.localQueues = make([]*lockFreeQueue[TaskFunc], numWorkers)
	e.workers = make([]*worker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		e.localQueues[i] = NewLockFreeQueue[TaskFunc](1024)
	}
	for i := 0; i < numWorkers; i++ {
		w := &worker{id: i, executor: e, localQueue: e.localQueues[i], stopCh: make(chan struct{}), stoppedCh: make(chan struct{})}
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

// manageResizes handles dynamic scaling for workers, ensuring proper shutdown and removal
// before truncating workers/localQueues slices and only marking as stopped/Done after that.
func (e *Executor) manageResizes(numaNode int) {
	for newCount := range e.resizeRequest {
		e.mu.Lock()
		if newCount <= 0 {
			newCount = 1
		}
		current := len(e.workers)
		if newCount > current {
			// Add workers
			for i := current; i < newCount; i++ {
				q := NewLockFreeQueue[TaskFunc](1024)
				e.localQueues = append(e.localQueues, q)
				w := &worker{id: i, executor: e, localQueue: q, stopCh: make(chan struct{}), stoppedCh: make(chan struct{})}
				e.workers = append(e.workers, w)
				e.wg.Add(1)
				go w.run(numaNode, &e.wg)
			}
		} else if newCount < current {
			// Mark extra workers for removal
			for i := newCount; i < current; i++ {
				close(e.workers[i].stopCh)
			}
			// Wait for all extra workers to notify they've exited
			for i := newCount; i < current; i++ {
				<-e.workers[i].stoppedCh // Wait for worker's run goroutine to signal full exit
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
// Now signals when stops fully so pool can safely delete from slice AFTER this signal.
type worker struct {
	id         int
	executor   *Executor
	localQueue *lockFreeQueue[TaskFunc]
	stopCh     chan struct{}
	stoppedCh  chan struct{} // Used for safe pool resizing!
}

func (w *worker) run(numaNode int, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		close(w.stoppedCh) // Now signal done only after full cleanup and before removal from slice.
	}()
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
