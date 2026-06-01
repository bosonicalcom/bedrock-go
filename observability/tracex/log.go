package tracex

import (
	"context"
	"log/slog"

	"github.com/bosonicalcom/bedrock-go/observability/logx"
)

// LoggerInterceptorFunc is an interceptor that adds the trace ID to the log record.
var LoggerInterceptorFunc logx.HandleFunc = func(ctx context.Context, r *slog.Record) error {
	traceID, ok := TraceIDFromContext(ctx)
	if !ok {
		return nil
	}
	r.AddAttrs(slog.String("trace_id", traceID))
	return nil
}
