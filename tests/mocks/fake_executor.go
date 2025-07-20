// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// fake_executor.go — Extremely simple mock for api.Executor.
package mocks

type FakeExecutor struct {
	Tasks []func()
}

func (e *FakeExecutor) Submit(task func()) error {
	e.Tasks = append(e.Tasks, task)
	task()
	return nil
}

func (e *FakeExecutor) NumWorkers() int        { return 1 }
func (e *FakeExecutor) Resize(newCount int)    {}
