// hioload-ws/internal/concurrency/executor.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform Executor and Worker Pool for high-load, NUMA-aware workloads.
// Designed for both Linux and Windows platforms.
// Integrates with the exported api.Executor interface.

package concurrency

import (
	"runtime"
	"sync"
)

// TaskFunc defines the signature for jobs executed by the Executor.
type TaskFunc func()

// Executor is a high-performance, NUMA-aware worker pool supporting dynamic scaling.
// All methods are thread-safe and non-blocking (where possible).
type Executor struct {
	mu       sync.Mutex
	workers  []*worker
	taskQ    chan TaskFunc
	numa     int // Preferred NUMA node (for pinning/thread allocation), -1=any.
	stop     chan struct{}
	wg       sync.WaitGroup
}

// NewExecutor creates an Executor with the given number of workers and preferred NUMA node.
// If numaNode < 0, NUMA-awareness is disabled.
func NewExecutor(numWorkers int, numaNode int) *Executor {
	exec := &Executor{
		taskQ:   make(chan TaskFunc, numWorkers*256), // Bulk buffer for burst workloads.
		stop:    make(chan struct{}),
		numa:    numaNode,
	}
	exec.Resize(numWorkers)
	return exec
}

// Submit dispatches a task to the pool. Returns nil if accepted, else error.
func (e *Executor) Submit(task TaskFunc) error {
	select {
	case e.taskQ <- task:
		return nil
	case <-e.stop:
		return ErrExecutorClosed
	default:
		return ErrTaskQueueFull
	}
}

// NumWorkers returns current number of active workers.
func (e *Executor) NumWorkers() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.workers)
}

// Resize adjusts the worker pool size at runtime.
func (e *Executor) Resize(count int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	diff := count - len(e.workers)
	if diff > 0 {
		for i := 0; i < diff; i++ {
			w := &worker{
				exec: e,
			}
			go w.run()
			if e.numa >= 0 {
				go PinCurrentThread(e.numa, i%runtime.NumCPU()) // Pin worker to CPU/NUMA node if requested.
			}
			e.workers = append(e.workers, w)
		}
	} else if diff < 0 {
		for i := 0; i < -diff; i++ {
			w := e.workers[0]
			w.stop()
			e.workers = e.workers[1:]
		}
	}
}

// Close stops all workers and releases resources. It is safe to call multiple times.
func (e *Executor) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	select {
	case <-e.stop:
		return
	default:
		close(e.stop)
	}
	for _, w := range e.workers {
		w.stop()
	}
	e.workers = nil
	close(e.taskQ)
	e.wg.Wait()
}

// Error types.
var (
	ErrExecutorClosed = &ExecutorError{"executor: closed"}
	ErrTaskQueueFull  = &ExecutorError{"executor: task queue full"}
)

// ExecutorError is a simple error implementation.
type ExecutorError struct{ msg string }

func (e *ExecutorError) Error() string { return e.msg }

// worker is a single background goroutine in the pool.
type worker struct {
	exec *Executor
	quit chan struct{}
}

func (w *worker) run() {
	w.quit = make(chan struct{})
	w.exec.wg.Add(1)
	defer w.exec.wg.Done()
	for {
		select {
		case task := <-w.exec.taskQ:
			task()
		case <-w.quit:
			return
		case <-w.exec.stop:
			return
		}
	}
}

func (w *worker) stop() {
	close(w.quit)
}
