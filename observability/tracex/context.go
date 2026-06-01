// Package tracex provides trace-ID propagation via context and slog integration.
package tracex

import "context"

type contextKeyType struct{}

var traceIDKey contextKeyType = struct{}{}

// WithTraceID returns a new context with the trace ID set.
func WithTraceID(parent context.Context, traceID string) context.Context {
	ctx := context.WithValue(parent, traceIDKey, traceID)
	return ctx
}

// TraceIDFromContext returns the trace ID from the context, if one is set.
func TraceIDFromContext(ctx context.Context) (string, bool) {
	traceID, ok := ctx.Value(traceIDKey).(string)
	return traceID, ok
}
