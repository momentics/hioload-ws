// hioload-ws/internal/concurrency/threadpool.go
// Author: momentics <momentics@gmail.com>
//
// Simple, NUMA-aware goroutine worker pool with affinity pinning.
// Used for low-latency event dispatch and batch scheduling.

package concurrency


// ThreadPool is an async pool for running burstable workloads with locality hints.
type ThreadPool struct {
	executor *Executor
}

// NewThreadPool creates a new NUMA-aware thread pool.
func NewThreadPool(size int, numaNode int) *ThreadPool {
	return &ThreadPool{
		executor: NewExecutor(size, numaNode),
	}
}

// Submit schedules a job on the thread pool.
func (tp *ThreadPool) Submit(f func()) error {
	return tp.executor.Submit(f)
}

// Len returns number of worker threads.
func (tp *ThreadPool) Len() int {
	return tp.executor.NumWorkers()
}

// Resize grows or shrinks thread pool.
func (tp *ThreadPool) Resize(n int) {
	tp.executor.Resize(n)
}

// Close stops the thread pool, waits for workers to finish.
func (tp *ThreadPool) Close() {
	tp.executor.Close()
}
