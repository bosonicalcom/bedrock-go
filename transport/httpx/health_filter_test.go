package httpx_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/bosonicalcom/bedrock-go/transport/httpx"
)

// logCapture is a slog.Handler that accumulates log records for test inspection.
type logCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (l *logCapture) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (l *logCapture) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, r.Clone())
	return nil
}
func (l *logCapture) WithAttrs(_ []slog.Attr) slog.Handler { return l }
func (l *logCapture) WithGroup(_ string) slog.Handler      { return l }

func (l *logCapture) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.records)
}

func (l *logCapture) reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = nil
}

// TestNewServer_LoggingSkipsHealthPaths verifies that the logging interceptor
// does not emit log records for /healthz or /readyz, but does log other paths.
func TestNewServer_LoggingSkipsHealthPaths(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantLogs int
	}{
		{name: "/healthz produces no logs", path: "/healthz", wantLogs: 0},
		{name: "/readyz produces no logs", path: "/readyz", wantLogs: 0},
		{name: "other paths are logged", path: "/api/something", wantLogs: 1},
	}

	logs := &logCapture{}
	srv, err := httpx.NewServer(httpx.WithServerLogger(slog.New(logs)))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs.reset()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, req)
			assert.Equal(t, tt.wantLogs, logs.count())
		})
	}
}

// TestNewServer_OTELSkipsHealthPaths verifies that otelhttp does not create spans
// for /healthz or /readyz, but does create them for other paths.
func TestNewServer_OTELSkipsHealthPaths(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	srv, err := httpx.NewServer(httpx.WithServerTracerProvider(tp))
	require.NoError(t, err)

	tests := []struct {
		name      string
		path      string
		wantSpans int
	}{
		{name: "/healthz produces no spans", path: "/healthz", wantSpans: 0},
		{name: "/readyz produces no spans", path: "/readyz", wantSpans: 0},
		{name: "other paths produce a span", path: "/api/something", wantSpans: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp.Reset()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			srv.Handler.ServeHTTP(w, req)
			require.NoError(t, tp.ForceFlush(context.Background()))
			assert.Len(t, exp.GetSpans(), tt.wantSpans)
		})
	}
}
