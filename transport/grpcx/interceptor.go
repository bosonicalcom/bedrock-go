package grpcx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	grpcmiddleware_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/bosonicalcom/bedrock-go/observability/logx"
	"github.com/bosonicalcom/bedrock-go/syserr"
)

const (
	MetadataRequestID     = "x-request-id"
	MetadataCorrelationID = "x-correlation-id"
)

// -- Request observation (shared state across interceptors via context)

type requestObservationContextKey struct{}

type requestObservation struct {
	startTime time.Time
	sysErr    *syserr.Error

	once     sync.Once
	observed observedError
}

type observedError struct {
	Code         syserr.Code
	Type         string
	Reason       string
	Domain       string
	RequestID    string
	DebugDetail  string
	StackEntries []string
	Causes       []string
}

func newRequestObservation() *requestObservation {
	return &requestObservation{startTime: time.Now()}
}

func withRequestObservation(ctx context.Context, obs *requestObservation) context.Context {
	return context.WithValue(ctx, requestObservationContextKey{}, obs)
}

func getRequestObservation(ctx context.Context) (*requestObservation, bool) {
	obs, ok := ctx.Value(requestObservationContextKey{}).(*requestObservation)
	return obs, ok
}

func (o *requestObservation) SetSystemError(err *syserr.Error) {
	o.sysErr = err
}

func (o *requestObservation) SystemError() *syserr.Error {
	return o.sysErr
}

func (o *requestObservation) ObservedError() observedError {
	o.once.Do(func() {
		if err := o.sysErr; err != nil {
			o.observed = extractObservedError(err)
		}
	})
	return o.observed
}

func extractObservedError(err *syserr.Error) observedError {
	out := observedError{
		Code: err.Code,
		Type: string(err.Code),
	}

	for _, detail := range err.Details {
		switch v := detail.(type) {
		case syserr.ErrorInfo:
			if out.Reason == "" && v.Reason != "" {
				out.Reason = v.Reason
				out.Domain = v.Domain
				out.Type = v.Reason
			}
		case syserr.RequestInfo:
			if out.RequestID == "" {
				out.RequestID = v.RequestID
			}
		case syserr.DebugInfo:
			if out.DebugDetail == "" {
				out.DebugDetail = v.Detail
			}
			if len(out.StackEntries) == 0 {
				out.StackEntries = append([]string(nil), v.StackEntries...)
			}
		}
	}

	out.Causes = flattenCauseMessages(err)
	return out
}

func flattenCauseMessages(err *syserr.Error) []string {
	if err == nil {
		return nil
	}

	seenErrs := map[error]struct{}{}
	seenMsgs := map[string]struct{}{}
	out := make([]string, 0, 4)

	var add func(error)
	add = func(e error) {
		if e == nil {
			return
		}
		if _, ok := seenErrs[e]; ok {
			return
		}
		seenErrs[e] = struct{}{}

		msg := e.Error()
		if msg != "" {
			if _, ok := seenMsgs[msg]; !ok {
				seenMsgs[msg] = struct{}{}
				out = append(out, msg)
			}
		}

		switch x := any(e).(type) {
		case interface{ Unwrap() []error }:
			for _, child := range x.Unwrap() {
				add(child)
			}
		case interface{ Unwrap() error }:
			add(x.Unwrap())
		}
	}

	for _, staticErr := range err.Causes {
		add(staticErr)
	}

	return out
}

// -- Stream wrapper

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }

// -- Identifier interceptor

// IdentifierServerInterceptor injects a unique request ID and propagates a correlation ID
// via gRPC metadata, and makes both available to the grpc-middleware logger via field injection.
func IdentifierServerInterceptor() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx = injectIdentifiers(ctx)
		return handler(ctx, req)
	}

	stream := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := injectIdentifiers(ss.Context())
		return handler(srv, &wrappedServerStream{ServerStream: ss, ctx: ctx})
	}

	return unary, stream
}

func injectIdentifiers(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)

	correlationID := ""
	if vals := md.Get(MetadataCorrelationID); len(vals) > 0 {
		correlationID = vals[0]
	}

	requestID := ""
	if id, err := uuid.NewV7(); err == nil {
		requestID = id.String()
	}

	if correlationID == "" {
		correlationID = requestID
	}

	// send IDs back to the client as response headers
	_ = grpc.SetHeader(ctx, metadata.Pairs(
		MetadataRequestID, requestID,
		MetadataCorrelationID, correlationID,
	))

	// inject into grpc-middleware logging context so all log entries carry these fields
	ctx = grpcmiddleware_logging.InjectFields(ctx, grpcmiddleware_logging.Fields{
		"request_id", requestID,
		"correlation_id", correlationID,
	})

	// store observation with start time for later interceptors
	obs := newRequestObservation()
	ctx = withRequestObservation(ctx, obs)

	return ctx
}

// -- Recovery handler (wired into grpc-middleware recovery interceptor)

// PanicRecoveryHandler converts a panic value into a gRPC status error.
// Pass this to [recovery.WithRecoveryHandlerContext] when building a recovery interceptor.
func PanicRecoveryHandler(ctx context.Context, p any) error {
	var sysErr *syserr.Error

	switch v := p.(type) {
	case *syserr.Error:
		sysErr = v
	case error:
		sysErr = syserr.New(syserr.CodeInternal, fmt.Sprintf("panic: %v", v))
	default:
		sysErr = syserr.New(syserr.CodeInternal, fmt.Sprintf("panic: %v", p))
	}

	if obs, ok := getRequestObservation(ctx); ok {
		obs.SetSystemError(sysErr)
	}

	return statusFromSystemError(sysErr).Err()
}

// -- syserr conversion helper (used by recovery and custom handlers)

// StatusFromError converts a [syserr.Error] to a gRPC status error, storing it in the
// request observation so tracing and metrics interceptors can inspect it.
func StatusFromError(ctx context.Context, err *syserr.Error) error {
	if obs, ok := getRequestObservation(ctx); ok {
		obs.SetSystemError(err)
	}
	return statusFromSystemError(err).Err()
}

// -- slog adapter for grpc-middleware logging

// SlogAdapter wraps a [slog.Logger] to implement the [grpcmiddleware_logging.Logger] interface.
func SlogAdapter(logger *slog.Logger) grpcmiddleware_logging.Logger {
	return grpcmiddleware_logging.LoggerFunc(func(ctx context.Context, level grpcmiddleware_logging.Level, msg string, fields ...any) {
		logger.Log(ctx, slog.Level(level), msg, fields...)
	})
}

// -- Span annotator interceptor

// SpanAnnotatorServerInterceptor annotates the active OpenTelemetry span with syserr-specific
// attributes (reason, domain, debug info, stack, causes) after the handler returns.
// It extends the baseline attributes that otelgrpc already records.
func SpanAnnotatorServerInterceptor() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	unary := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		annotateSpan(ctx, info.FullMethod, err)
		return resp, err
	}

	stream := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		annotateSpan(ss.Context(), info.FullMethod, err)
		return err
	}

	return unary, stream
}

func annotateSpan(ctx context.Context, method string, err error) {
	if err == nil {
		return
	}

	obs, ok := getRequestObservation(ctx)
	if !ok || obs.SystemError() == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	e := obs.ObservedError()
	sysErr := obs.SystemError()

	span.RecordError(sysErr)
	span.SetStatus(otelcodes.Error, sysErr.Message)

	attrs := []attribute.KeyValue{
		attribute.String("transport", "grpc"),
		attribute.String("rpc.method", method),
		attribute.String("app.error.code", string(e.Code)),
		attribute.String("error.type", e.Type),
	}
	if e.Reason != "" {
		attrs = append(attrs, attribute.String("app.error.reason", e.Reason))
	}
	if e.Domain != "" {
		attrs = append(attrs, attribute.String("app.error.domain", e.Domain))
	}
	if e.RequestID != "" {
		attrs = append(attrs, attribute.String("app.request.id", e.RequestID))
	}
	span.SetAttributes(attrs...)

	if e.DebugDetail != "" {
		span.AddEvent("app.error.debug", trace.WithAttributes(attribute.String("detail", e.DebugDetail)))
	}
	if len(e.StackEntries) > 0 {
		span.AddEvent("app.error.stack", trace.WithAttributes(attribute.StringSlice("entries", e.StackEntries)))
	}
	if len(e.Causes) > 0 {
		span.AddEvent("app.error.causes", trace.WithAttributes(attribute.StringSlice("messages", e.Causes)))
	}
}

// -- Metric interceptor

// MetricServerInterceptor increments an error counter when the handler returns a syserr.
// Keep labels low-cardinality — no request IDs, debug details, or raw paths.
func MetricServerInterceptor(counter metric.Int64Counter) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	record := func(ctx context.Context, method string) {
		obs, ok := getRequestObservation(ctx)
		if !ok || obs.SystemError() == nil {
			return
		}
		e := obs.ObservedError()
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("transport", "grpc"),
				attribute.String("rpc.method", method),
				attribute.String("app.error.code", string(e.Code)),
				attribute.String("error.type", e.Type),
			),
		)
	}

	unary := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		record(ctx, info.FullMethod)
		return resp, err
	}

	stream := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		record(ss.Context(), info.FullMethod)
		return err
	}

	return unary, stream
}

// -- Error logging interceptor

// ErrorLogServerInterceptor logs a structured error entry when the handler returns a syserr.
// Wire this after the logger (grpc-middleware) so that request/correlation IDs are already in context.
func ErrorLogServerInterceptor(logger *slog.Logger) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	logErr := func(ctx context.Context, method string) {
		obs, ok := getRequestObservation(ctx)
		if !ok || obs.SystemError() == nil {
			return
		}
		sysErr := obs.SystemError()
		logger.ErrorContext(ctx, "grpc request failed",
			slog.String("rpc.method", method),
			slog.String("took", time.Since(obs.startTime).String()),
			logx.Error(sysErr),
		)
	}

	unary := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		logErr(ctx, info.FullMethod)
		return resp, err
	}

	stream := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		logErr(ss.Context(), info.FullMethod)
		return err
	}

	return unary, stream
}

// -- Syserr interceptor
// Converts *syserr.Error returns from handlers into gRPC status errors automatically,
// so service implementations can return syserr.Error directly without manual conversion.

// SyserrServerInterceptor converts any *syserr.Error returned by a handler into a
// gRPC status error, and records it in the request observation for downstream interceptors.
func SyserrServerInterceptor() (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	convert := func(ctx context.Context, err error) error {
		if err == nil {
			return nil
		}
		var sysErr *syserr.Error
		if errors.As(err, &sysErr) {
			return StatusFromError(ctx, sysErr)
		}
		return err
	}

	unary := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		return resp, convert(ctx, err)
	}

	stream := func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return convert(ss.Context(), handler(srv, ss))
	}

	return unary, stream
}
