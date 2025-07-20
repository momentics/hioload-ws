// Copyright 2025 momentics@gmail.com
// License: Apache 2.0

// tracer_stub_test.go — Stub implementation and property check for api.Tracer/Span.
package tests

import (
	"testing"
	"github.com/momentics/hioload-ws/api"
)

type FakeSpan struct {
	tags   map[string]any
	logs   []map[string]any
	finish bool
}

func (s *FakeSpan) Finish()                  { s.finish = true }
func (s *FakeSpan) SetTag(k string, v any)   { s.tags[k] = v }
func (s *FakeSpan) Log(fields map[string]any){ s.logs = append(s.logs, fields) }
func (s *FakeSpan) Context() map[string]any  { return map[string]any{"tags": s.tags} }

type FakeTracer struct{}
func (t *FakeTracer) StartSpan(name string, _ ...api.SpanOption) api.Span {
	return &FakeSpan{tags: map[string]any{"name": name}}
}
func (t *FakeTracer) Inject(span api.Span, carrier map[string]any) {
	carrier["trace"] = span.Context()
}
func (t *FakeTracer) Extract(carrier map[string]any) (api.Span, error) {
	return &FakeSpan{tags: carrier}, nil
}

func TestFakeTracer_SpanRoundtrip(t *testing.T) {
	tracer := &FakeTracer{}
	span := tracer.StartSpan("ping")
	span.SetTag("user", "bob")
	span.Log(map[string]any{"op": "msg"})
	carrier := make(map[string]any)
	tracer.Inject(span, carrier)
	s2, err := tracer.Extract(carrier)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}
	ctx := s2.Context()
	if ctx["tags"] == nil {
		t.Error("Trace context not found")
	}
}
