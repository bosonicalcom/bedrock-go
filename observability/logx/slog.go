// Package logx provides slog handler extensions, including interceptor-based enrichment of log records.
package logx

import (
	"context"
	"log/slog"
)

type interceptorHandler struct {
	next              slog.Handler
	chainedHandleFunc []HandleFunc
}

// HandleFunc is a function that intercepts and handles a log record.
type HandleFunc func(ctx context.Context, r *slog.Record) error

var _ slog.Handler = (*interceptorHandler)(nil)

// NewInterceptorHandler wraps parent with a chain of [HandleFunc] interceptors.
// Interceptors run in reverse registration order before the record is forwarded
// to parent, allowing each one to mutate or enrich the [slog.Record].
// If any interceptor returns an error the chain stops and parent is not called.
func NewInterceptorHandler(parent slog.Handler, interceptorFuncs ...HandleFunc) slog.Handler {
	return &interceptorHandler{
		next:              parent,
		chainedHandleFunc: interceptorFuncs,
	}
}

// Enabled implements [slog.Handler].
func (i *interceptorHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return i.next.Enabled(ctx, level)
}

// Handle implements [slog.Handler].
func (i *interceptorHandler) Handle(ctx context.Context, r slog.Record) error {
	// exec outermost
	for j := len(i.chainedHandleFunc) - 1; j >= 0; j-- {
		if err := i.chainedHandleFunc[j](ctx, &r); err != nil {
			return err
		}
	}
	return i.next.Handle(ctx, r)
}

// WithAttrs implements [slog.Handler].
func (i *interceptorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return i.next.WithAttrs(attrs)
}

// WithGroup implements [slog.Handler].
func (i *interceptorHandler) WithGroup(name string) slog.Handler {
	return i.next.WithGroup(name)
}
