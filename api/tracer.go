// Package api
// Author: momentics
//
// Tracing and span API for distributed observability and performance analysis.

package api

// Tracer manages distributed trace spans and events.
type Tracer interface {
    // StartSpan starts a new trace span.
    StartSpan(name string, opts ...SpanOption) Span

    // Inject encodes span context for propagation.
    Inject(span Span, carrier map[string]any)

    // Extract decodes span context from carrier.
    Extract(carrier map[string]any) (Span, error)
}

// Span describes a unit of trace.
type Span interface {
    Finish()
    SetTag(key string, value any)
    Log(fields map[string]any)
    Context() map[string]any // Returns internal propagation context.
}

// SpanOption declares options for custom span start.
type SpanOption interface{}
