package fake

import (
	"github.com/momentics/hioload-ws/api"
)

// FakePoller implements api.Poller interface for testing
type FakePoller struct {
	RegisterCalls   []api.Handler
	RegisteredCount int
	PushCalls       []api.Event
	PushedCount     int
	PollCount       int
}

func NewFakePoller() *FakePoller {
	return &FakePoller{
		RegisterCalls: make([]api.Handler, 0),
		PushCalls:     make([]api.Event, 0),
	}
}

func (fp *FakePoller) Register(h api.Handler) error {
	fp.RegisterCalls = append(fp.RegisterCalls, h)
	fp.RegisteredCount++
	return nil
}

func (fp *FakePoller) Poll(maxEvents int) (int, error) {
	fp.PollCount++
	return 0, nil // Return 0 events for now
}

func (fp *FakePoller) Unregister(h api.Handler) error {
	// Remove from registered calls
	newCalls := make([]api.Handler, 0)
	for _, call := range fp.RegisterCalls {
		if call != h {
			newCalls = append(newCalls, call)
		}
	}
	fp.RegisterCalls = newCalls
	return nil
}

func (fp *FakePoller) Stop() {
	// Nothing to do in fake implementation
}

func (fp *FakePoller) Push(ev api.Event) bool {
	fp.PushCalls = append(fp.PushCalls, ev)
	fp.PushedCount++
	return true
}