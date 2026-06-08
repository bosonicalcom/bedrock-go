package httpx

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bosonicalcom/bedrock-go/internal/containerx/sets"
	"github.com/bosonicalcom/bedrock-go/observability/logx"
	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	HeaderRequestID     = "X-Request-Id"
	HeaderCorrelationID = "X-Correlation-Id"
)

// - Server

// ServerInterceptor is a function type used to chain behaviors for server requests.
type ServerInterceptor func(next http.Handler) http.Handler

// ServerInterceptorChain applies interceptors in order (first = outermost)
func ServerInterceptorChain(h http.Handler, interceptors ...ServerInterceptor) http.Handler {
	// Apply in reverse so the first interceptor is outermost
	for i := len(interceptors) - 1; i >= 0; i-- {
		h = interceptors[i](h)
	}
	return h
}

// IdentifierServerInterceptor wraps server requests with a request and correlation ID.
func IdentifierServerInterceptor() ServerInterceptor {
	return func(next http.Handler) http.Handler {
		return Handler(func(w http.ResponseWriter, r *http.Request) {
			correlationID := r.Header.Get(HeaderCorrelationID)
			requestID, err := uuid.NewV7()
			if err != nil {
				w.Header().Set(HeaderCorrelationID, correlationID)
				next.ServeHTTP(w, r)
				return
			}

			requestIDStr := requestID.String()
			if correlationID == "" {
				correlationID = requestIDStr
			}

			r.Header.Set(HeaderCorrelationID, correlationID)
			w.Header().Set(HeaderCorrelationID, correlationID)
			r.Header.Set(HeaderRequestID, requestIDStr)
			w.Header().Set(HeaderRequestID, requestIDStr)
			next.ServeHTTP(w, r)
		})
	}
}

// RecoverServerInterceptor wraps server requests with a panic recovery.
//
// If a panic happens, it will write an error response (most likely with [http.StatusInternalServerError]).
// This way, other [ServerInterceptor] instances can act accordingly.
func RecoverServerInterceptor() ServerInterceptor {
	return func(next http.Handler) http.Handler {
		return Handler(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter, r, _ := ensureRequestObservation(w, r)

			defer func() {
				if errRaw := recover(); errRaw != nil {
					if errRaw == http.ErrAbortHandler {
						panic(errRaw)
					}

					err, ok := errRaw.(error)
					if !ok {
						err = fmt.Errorf("panic: %v", errRaw)
					}
					RespondErrorJSON(wrappedWriter, err, WithRequestID(r.Header.Get(HeaderRequestID)))
				}
			}()

			next.ServeHTTP(wrappedWriter, r)
		})
	}
}

var _acceptedLogAuthHeaderPrefixes = sync.OnceValue[sets.HashSet[string]](func() sets.HashSet[string] {
	return sets.NewHashSet("Bearer", "API-Key", "Basic")
})()

// LogServerInterceptor wraps server requests with structured logs.
//
// It emits a normal access log for every request and a second error log when a system error was written.
func LogServerInterceptor(logger *slog.Logger, lvl slog.Level) ServerInterceptor {
	return func(next http.Handler) http.Handler {
		return Handler(func(w http.ResponseWriter, r *http.Request) {
			// prepare
			authPrefix := "None"
			if apiKey := r.Header.Get("X-Api-Key"); apiKey != "" {
				authPrefix = "API-Key"
			} else if authHeader := r.Header.Get("Authorization"); authHeader != "" {
				authPrefix = strings.SplitN(authHeader, " ", 2)[0]
			}

			if authPrefix != "None" && !_acceptedLogAuthHeaderPrefixes.Contains(authPrefix) {
				authPrefix = "" // non-allowed auth header value prefixes might contain sensitive info, so this removes them
			}

			reqLogGroup := slog.Group("request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("authorization_type", authPrefix),
				slog.String("accept_language", r.Header.Get("Accept-Language")),
			)
			logger.Log(r.Context(), lvl, "received http request",
				slog.String("correlation_id", r.Header.Get(HeaderCorrelationID)),
				slog.String("request_id", r.Header.Get(HeaderRequestID)),
				reqLogGroup,
			)

			// act
			wrappedWriter, r, obs := ensureRequestObservation(w, r)
			next.ServeHTTP(wrappedWriter, r)

			// log error (if any)
			sysErr := obs.SystemError()
			if sysErr == nil {
				return
			}

			attrs := []any{
				slog.String("correlation_id", r.Header.Get(HeaderCorrelationID)),
				slog.String("request_id", r.Header.Get(HeaderRequestID)),
				slog.String("took", time.Since(obs.startTime).String()),
				reqLogGroup,
				slog.Group("response",
					slog.Int("status", wrappedWriter.status),
					slog.Int("body_bytes", wrappedWriter.size),
				),
				logx.Error(sysErr),
			}
			logger.ErrorContext(r.Context(), "http request failed", attrs...)
		})
	}
}

// TraceServerInterceptor records internal errors on the active span.
//
// It expects some upstream middleware to have already created a span and placed it in the request context.
func TraceServerInterceptor() ServerInterceptor {
	return func(next http.Handler) http.Handler {
		return Handler(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter, r, obs := ensureRequestObservation(w, r)
			next.ServeHTTP(wrappedWriter, r)

			sysErr := obs.SystemError()
			if sysErr == nil {
				return
			}

			span := trace.SpanFromContext(r.Context())
			if !span.IsRecording() {
				return
			}

			e := obs.ObservedError()
			span.RecordError(sysErr)
			span.SetStatus(otelcodes.Error, sysErr.Message)

			attrs := []attribute.KeyValue{
				attribute.String("transport", "http"),
				attribute.String("http.request.method", r.Method),
				attribute.Int("http.response.status_code", wrappedWriter.status),
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
				span.AddEvent("app.error.debug",
					trace.WithAttributes(
						attribute.String("detail", e.DebugDetail),
					),
				)
			}
			if len(e.StackEntries) > 0 {
				span.AddEvent("app.error.stack",
					trace.WithAttributes(
						attribute.StringSlice("entries", e.StackEntries),
					),
				)
			}
			if len(e.Causes) > 0 {
				span.AddEvent("app.error.causes",
					trace.WithAttributes(
						attribute.StringSlice("messages", e.Causes),
					),
				)
			}
		})
	}
}

// MetricServerInterceptor increments an error counter for responses that wrote a system error.
//
// Keep labels low-cardinality. Do not add request IDs, debug details, stack traces, or raw paths here.
func MetricServerInterceptor(counter metric.Int64Counter) ServerInterceptor {
	return func(next http.Handler) http.Handler {
		return Handler(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter, r, obs := ensureRequestObservation(w, r)
			next.ServeHTTP(wrappedWriter, r)

			if obs.SystemError() == nil {
				return
			}

			e := obs.ObservedError()
			counter.Add(r.Context(), 1,
				metric.WithAttributes(
					attribute.String("transport", "http"),
					attribute.String("http.request.method", r.Method),
					attribute.Int("http.response.status_code", wrappedWriter.status),
					attribute.String("app.error.code", string(e.Code)),
					attribute.String("error.type", e.Type),
				),
			)
		})
	}
}

// LanguageLoaderServerInterceptor wraps server requests with a language loader.
// func LanguageLoaderServerInterceptor() ServerInterceptor {
// 	return func(next http.Handler) http.Handler {
// 		return Handler(func(w http.ResponseWriter, r *http.Request) {
// 			ctx := i18n.WithLocalizerContext(r.Context(), r.Header.Get("Accept-Language"), language.English.String())
// 			next.ServeHTTP(w, r.WithContext(ctx))
// 		})
// 	}
// }

// -- Util

// Handler returns an [http.Handler], formed from fn ([http.HandlerFunc]).
func Handler(fn http.HandlerFunc) http.Handler {
	return fn
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	size        int
	systemErr   *syserr.Error
}

var _ http.ResponseWriter = (*responseRecorder)(nil)

func (r *responseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return // WriteHeader can only be called once
	}
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK) // implicit 200 if Write called first
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func (r *responseRecorder) SetSystemError(err *syserr.Error) {
	r.systemErr = err
}

func (r *responseRecorder) SystemError() *syserr.Error {
	return r.systemErr
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Push(target string, opts *http.PushOptions) error {
	if f, ok := r.ResponseWriter.(http.Pusher); ok {
		return f.Push(target, opts)
	}
	return errors.New("bedrock.httpx: push not implemented")
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if f, ok := r.ResponseWriter.(http.Hijacker); ok {
		return f.Hijack()
	}
	return nil, nil, errors.New("bedrock.httpx: hijack not implemented")
}

// Constructor with sane default.
func wrapResponseWriter(w http.ResponseWriter) *responseRecorder {
	if rw, ok := w.(*responseRecorder); ok {
		return rw
	}
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

type requestObservationContextKey struct{}

type requestObservation struct {
	startTime time.Time
	rw        *responseRecorder

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
	DetailTypes  []string
	Causes       []string
}

func withRequestObservation(r *http.Request, obs *requestObservation) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), requestObservationContextKey{}, obs))
}

func getRequestObservation(ctx context.Context) (*requestObservation, bool) {
	obs, ok := ctx.Value(requestObservationContextKey{}).(*requestObservation)
	return obs, ok
}

func ensureRequestObservation(w http.ResponseWriter, r *http.Request) (*responseRecorder, *http.Request, *requestObservation) {
	rw := wrapResponseWriter(w)
	if obs, ok := getRequestObservation(r.Context()); ok {
		return rw, r, obs
	}

	obs := &requestObservation{
		startTime: time.Now(),
		rw:        rw,
	}
	return rw, withRequestObservation(r, obs), obs
}

func (o *requestObservation) SystemError() *syserr.Error {
	return o.rw.SystemError()
}

func (o *requestObservation) ObservedError() observedError {
	o.once.Do(func() {
		if err := o.SystemError(); err != nil {
			o.observed = extractObservedError(err)
		}
	})
	return o.observed
}

func extractObservedError(err *syserr.Error) observedError {
	out := observedError{
		Code: err.Code,
		Type: string(err.Code), // coarse fallback
	}

	for _, detail := range err.Details {
		out.DetailTypes = append(out.DetailTypes, detail.DetailType())

		switch v := detail.(type) {
		case syserr.ErrorInfo:
			if out.Reason == "" && v.Reason != "" {
				out.Reason = v.Reason
				out.Domain = v.Domain
				out.Type = v.Reason
			}

		case syserr.BadRequest:
			if out.Reason == "" && len(v.Violations) > 0 && v.Violations[0].Reason != "" {
				out.Type = v.Violations[0].Reason
			}

		case syserr.RequestInfo:
			if out.RequestID == "" {
				out.RequestID = v.RequestID
			}

		case syserr.DebugInfo:
			if out.DebugDetail == "" {
				out.DebugDetail = v.Detail
			}
			if len(out.StackEntries) == 0 && len(v.StackEntries) > 0 {
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

// - Client
