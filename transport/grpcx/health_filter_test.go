package grpcx_test

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/test/bufconn"

	"github.com/bosonicalcom/bedrock-go/transport/grpcx"
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

// newHealthClient builds a grpc.Server from opts on an in-memory listener and
// returns a HealthClient wired to it. Both are cleaned up when t ends.
func newHealthClient(t *testing.T, opts ...grpcx.ServerOption) grpc_health_v1.HealthClient {
	t.Helper()
	srv, err := grpcx.NewServer(opts...)
	require.NoError(t, err)

	lis := bufconn.Listen(1 << 20)
	go srv.Serve(lis) //nolint:errcheck
	t.Cleanup(func() { srv.Stop(); lis.Close() })

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return grpc_health_v1.NewHealthClient(conn)
}

// TestNewServer_LoggingSkipsHealthChecks verifies that the logging interceptor
// does not emit log records for any gRPC health-protocol method.
func TestNewServer_LoggingSkipsHealthChecks(t *testing.T) {
	tests := []struct {
		name string
		call func(ctx context.Context, c grpc_health_v1.HealthClient) error
	}{
		{
			name: "Check",
			call: func(ctx context.Context, c grpc_health_v1.HealthClient) error {
				_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
				return err
			},
		},
		{
			name: "List",
			call: func(ctx context.Context, c grpc_health_v1.HealthClient) error {
				_, err := c.List(ctx, &grpc_health_v1.HealthListRequest{})
				return err
			},
		},
		{
			name: "Watch",
			call: func(ctx context.Context, c grpc_health_v1.HealthClient) error {
				watchCtx, cancel := context.WithCancel(ctx)
				stream, err := c.Watch(watchCtx, &grpc_health_v1.HealthCheckRequest{})
				if err != nil {
					cancel()
					return err
				}
				_, err = stream.Recv()
				cancel()
				if err != nil {
					return err
				}
				// drain until the server notices the cancellation
				for {
					if _, err := stream.Recv(); err != nil {
						return nil
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := &logCapture{}
			client := newHealthClient(t, grpcx.WithServerLogger(slog.New(logs)))

			require.NoError(t, tt.call(context.Background(), client))
			assert.Equal(t, 0, logs.count(), "health check calls must not produce log entries")
		})
	}
}

// TestNewServer_OTELSkipsHealthChecks verifies that the otelgrpc stats handler
// does not create spans for any gRPC health-protocol method.
func TestNewServer_OTELSkipsHealthChecks(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	tests := []struct {
		name string
		call func(ctx context.Context, c grpc_health_v1.HealthClient) error
	}{
		{
			name: "Check",
			call: func(ctx context.Context, c grpc_health_v1.HealthClient) error {
				_, err := c.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
				return err
			},
		},
		{
			name: "List",
			call: func(ctx context.Context, c grpc_health_v1.HealthClient) error {
				_, err := c.List(ctx, &grpc_health_v1.HealthListRequest{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp.Reset()
			// Create the server after the global provider is set so otelgrpc captures it.
			client := newHealthClient(t, grpcx.EnableServerTelemetry())

			require.NoError(t, tt.call(context.Background(), client))
			require.NoError(t, tp.ForceFlush(context.Background()))

			assert.Empty(t, exp.GetSpans(), "health check calls must not produce OTEL spans")
		})
	}
}
