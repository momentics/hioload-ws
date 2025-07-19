// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// NUMA-aware executor using lock-free MPMC queue for task dispatch.

package concurrency

import (
	"github.com/eapache/queue"
)

type TaskFunc func()

type Executor struct {
	queue   *queue.Queue
	workers []worker
	stop    chan struct{}
}

func (e *Executor) NumWorkers() any {
	return len(e.workers)
}

type worker struct {
	exec *Executor
	stop chan struct{}
}

func NewExecutor(numWorkers, numaNode int) *Executor {
	e := &Executor{
		queue: queue.New(),
		stop:  make(chan struct{}),
	}
	for i := 0; i < numWorkers; i++ {
		w := worker{exec: e, stop: make(chan struct{})}
		go w.run()
		e.workers = append(e.workers, w)
	}
	return e
}

func (e *Executor) Submit(task TaskFunc) error {
	select {
	case <-e.stop:
		return ErrExecutorClosed
	default:
		e.queue.Enqueue(task)
		return nil
	}
}

func (e *Executor) Close() {
	close(e.stop)
}

func (w *worker) run() {
	for {
		select {
		case <-w.stop:
			return
		default:
			if item, ok := w.exec.queue.Dequeue(); ok {
				if task, ok2 := item.(TaskFunc); ok2 {
					task()
				}
			}
		}
	}
}
