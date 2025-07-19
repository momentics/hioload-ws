// Author: momentics <momentics@gmail.com>
// License: Apache-2.0
//
// High-precision scheduler with prefetch optimizations.

package concurrency

import (
    "container/heap"
    "sync"
    "time"

    "golang.org/x/sys/cpu"
)

type Scheduler struct {
    mu      sync.Mutex
    timerQ  taskHeap
    notify  chan struct{}
    stop    chan struct{}
}

func (s *Scheduler) run() {
    for {
        s.mu.Lock()
        if s.timerQ.Len() == 0 {
            s.mu.Unlock()
            select {
            case <-s.notify:
            case <-s.stop:
                return
            }
            continue
        }

        task := s.timerQ[0]
        if cpu.X86.HasSSE2 {
            cpu.Prefetch(unsafe.Pointer(&task))
        }
        // … остальная логика без изменений
        s.mu.Unlock()
    }
}
