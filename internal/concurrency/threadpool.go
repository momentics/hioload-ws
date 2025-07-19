// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// ThreadPool wraps Executor with lock-free queue underneath.

package concurrency

type ThreadPool struct {
    executor *Executor
}

func NewThreadPool(size, numaNode int) *ThreadPool {
    return &ThreadPool{
        executor: NewExecutor(size, numaNode),
    }
}

func (tp *ThreadPool) Submit(f func()) error {
    return tp.executor.Submit(f)
}

func (tp *ThreadPool) Close() {
    tp.executor.Close()
}
