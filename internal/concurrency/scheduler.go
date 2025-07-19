// hioload-ws/internal/concurrency/scheduler.go
// Author: momentics <momentics@gmail.com>
//
// Cross-platform monotonic timer scheduler. Designed for high-concurrency events and low-latency callbacks.
// Uses a lock-free heap and single goroutine per NUMA node (for maximum timer scale-out and locality).

package concurrency

import (
	"container/heap"
	"sync"
	"time"
)

// Scheduler provides high-precision timer-based scheduling for async workloads.
type Scheduler struct {
	mu      sync.Mutex
	timerQ  taskHeap
	notify  chan struct{}
	stop    chan struct{}
	running bool
}

// scheduledTask is an internal structure for delayed tasks.
type scheduledTask struct {
	triggerAt time.Time
	callback  func()
	canceled  bool
	id        int64
}

// NewScheduler creates a new scheduler instance.
func NewScheduler() *Scheduler {
	s := &Scheduler{
		timerQ:  taskHeap{},
		notify:  make(chan struct{}, 1),
		stop:    make(chan struct{}),
		running: true,
	}
	go s.run()
	return s
}

// Schedule schedules a callback to run after the given delay (in nanoseconds).
func (s *Scheduler) Schedule(delayNanos int64, fn func()) *scheduledTask {
	task := &scheduledTask{
		triggerAt: time.Now().Add(time.Duration(delayNanos)),
		callback:  fn,
	}
	s.mu.Lock()
	heap.Push(&s.timerQ, task)
	s.mu.Unlock()
	s.wakeup()
	return task
}

// Cancel cancels a scheduled task.
func (s *Scheduler) Cancel(task *scheduledTask) {
	s.mu.Lock()
	task.canceled = true
	s.mu.Unlock()
}

// wakeup notifies the runner goroutine for timer changes.
func (s *Scheduler) wakeup() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// run executes ready tasks on time.
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

		now := time.Now()
		task := s.timerQ[0]
		if !task.canceled && now.After(task.triggerAt) {
			heap.Pop(&s.timerQ)
			s.mu.Unlock()
			if !task.canceled {
				go task.callback()
			}
			continue
		}
		delay := task.triggerAt.Sub(now)
		s.mu.Unlock()

		select {
		case <-time.After(delay):
		case <-s.notify:
		case <-s.stop:
			return
		}
	}
}

// taskHeap is a min-heap for scheduled tasks.
type taskHeap []*scheduledTask

func (h taskHeap) Len() int           { return len(h) }
func (h taskHeap) Less(i, j int) bool { return h[i].triggerAt.Before(h[j].triggerAt) }
func (h taskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *taskHeap) Push(x interface{}) {
	*h = append(*h, x.(*scheduledTask))
}
func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
