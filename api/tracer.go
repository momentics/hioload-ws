// Package api
// Author: momentics <momentics@gmail.com>
//
// Distributed tracing and span management contract.

package api

// Tracer manages structured spans and context propagation.
type Tracer interface {
    // StartSpan creates a new span for a named action/operation.
    StartSpan(name string, opts ...SpanOption) Span

    // Inject encodes the span context into the downstream carrier.
    Inject(span Span, carrier map[string]any)

    // Extract reconstructs a span from an upstream carrier.
    Extract(carrier map[string]any) (Span, error)
}

// Span represents a unit of work in tracing systems.
type Span interface {
    // Finish marks the span as completed.
    Finish()

    // SetTag attaches metadata to the span.
    SetTag(key string, value any)

    // Log emits a structured event into the span timeline.
    Log(fields map[string]any)

    // Context returns the internal propagation map.
    Context() map[string]any
}

// SpanOption allows future extendability for span customization.
type SpanOption interface{}
