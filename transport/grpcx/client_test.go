package grpcx_test

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/bosonicalcom/bedrock-go/syserr"
	"github.com/bosonicalcom/bedrock-go/transport/grpcx"
)

// validationError mirrors what validator.GoPlaygroundValidator produces, which is the
// shape these tests care most about round-tripping.
func validationError() *syserr.Error {
	return syserr.New(syserr.CodeInvalidArgument, "validation failed",
		syserr.WithDetails(
			syserr.ErrorInfo{
				Reason: syserr.ReasonValidationFailed,
				Domain: syserr.GlobalDomain,
			},
			syserr.BadRequest{Violations: []syserr.FieldViolation{
				{
					Field:       "user.email",
					Description: "must be a valid email",
					Reason:      syserr.ReasonInvalidFormat,
					LocalizedMessage: syserr.LocalizedMessage{
						Locale:  language.Make("es-MX"),
						Message: "correo inválido",
					},
				},
				{Field: "user.name", Description: "is required", Reason: syserr.ReasonRequiredValue},
			}},
			// must never reach the client
			syserr.DebugInfo{Detail: "internal state", StackEntries: []string{"main.go:10"}},
		),
	)
}

// stubHealthServer answers both health methods with err. A nil err makes Watch emit two
// messages and complete normally, which is how the io.EOF passthrough is exercised.
type stubHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
	err error
}

func (s *stubHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func (s *stubHealthServer) Watch(_ *grpc_health_v1.HealthCheckRequest, ss grpc.ServerStreamingServer[grpc_health_v1.HealthCheckResponse]) error {
	if s.err != nil {
		return s.err
	}
	for range 2 {
		if err := ss.Send(&grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}); err != nil {
			return err
		}
	}
	return nil
}

// RegisterServers implements [grpcx.Controller].
func (s *stubHealthServer) RegisterServers(r grpc.ServiceRegistrar) {
	grpc_health_v1.RegisterHealthServer(r, s)
}

// newStubHealthClient serves a stubHealthServer over an in-memory listener and returns a
// client built through grpcx.NewClient, so the interceptor wiring itself is under test.
func newStubHealthClient(t *testing.T, handlerErr error, opts ...grpcx.ClientOpt) grpc_health_v1.HealthClient {
	t.Helper()

	srv, err := grpcx.NewServer(
		// required: NewServer registers its own health service otherwise, and the duplicate panics
		grpcx.DisableServerHealthExport(),
		grpcx.WithServerControllers([]grpcx.Controller{&stubHealthServer{err: handlerErr}}),
	)
	require.NoError(t, err)

	lis := bufconn.Listen(1 << 20)
	go srv.Serve(lis) //nolint:errcheck
	t.Cleanup(func() { srv.Stop(); lis.Close() })

	opts = append(opts, grpcx.WithClientOptions(
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
	))

	conn, err := grpcx.NewClient("stub-health", "passthrough:///bufconn", opts...)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return grpc_health_v1.NewHealthClient(conn)
}

// assertValidationError checks that err carries every field validationError put on the wire.
func assertValidationError(t *testing.T, err error) {
	t.Helper()

	sysErr, ok := errors.AsType[*syserr.Error](err)
	require.True(t, ok, "expected a *syserr.Error, got %T: %v", err, err)
	assert.Equal(t, syserr.CodeInvalidArgument, sysErr.Code)
	assert.Equal(t, "validation failed", sysErr.Message)

	assert.Contains(t, sysErr.Details, syserr.ErrorInfo{
		Reason: syserr.ReasonValidationFailed,
		Domain: syserr.GlobalDomain,
	})
	assert.Contains(t, sysErr.Details, syserr.BadRequest{Violations: []syserr.FieldViolation{
		{
			Field:       "user.email",
			Description: "must be a valid email",
			Reason:      syserr.ReasonInvalidFormat,
			LocalizedMessage: syserr.LocalizedMessage{
				Locale:  language.Make("es-MX"),
				Message: "correo inválido",
			},
		},
		{Field: "user.name", Description: "is required", Reason: syserr.ReasonRequiredValue},
	}})

	assert.False(t, syserr.IsDetailType[syserr.DebugInfo](sysErr), "DebugInfo must not reach the client")

	// the underlying status is still reachable through the causes
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestNewClient_SyserrUnary(t *testing.T) {
	client := newStubHealthClient(t, validationError())

	_, err := client.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})

	assertValidationError(t, err)
}

func TestNewClient_SyserrStream(t *testing.T) {
	client := newStubHealthClient(t, validationError())

	stream, err := client.Watch(t.Context(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	_, err = stream.Recv()

	assertValidationError(t, err)
}

// TestNewClient_SyserrStreamEndsWithEOF guards the one thing the interceptor must never
// break: a stream that completes normally still terminates with io.EOF.
func TestNewClient_SyserrStreamEndsWithEOF(t *testing.T) {
	client := newStubHealthClient(t, nil)

	stream, err := client.Watch(t.Context(), &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	received := 0
	for {
		_, err := stream.Recv()
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			_, isSyserr := errors.AsType[*syserr.Error](err)
			require.False(t, isSyserr, "io.EOF must not be converted into a syserr")
			break
		}
		received++
	}

	assert.Equal(t, 2, received)
}

func TestNewClient_SyserrDisabled(t *testing.T) {
	client := newStubHealthClient(t, validationError(), grpcx.DisableClientSyserrInterceptor())

	_, err := client.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})

	require.Error(t, err)
	_, isSyserr := errors.AsType[*syserr.Error](err)
	assert.False(t, isSyserr, "conversion must be off when disabled")
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestNewClient_SyserrNonSyserrHandlerError checks that a plain status error returned by a
// handler still becomes a syserr, just without details.
func TestNewClient_SyserrNonSyserrHandlerError(t *testing.T) {
	client := newStubHealthClient(t, status.Error(codes.NotFound, "no such service"))

	_, err := client.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})

	sysErr, ok := errors.AsType[*syserr.Error](err)
	require.True(t, ok)
	assert.Equal(t, syserr.CodeNotFound, sysErr.Code)
	assert.Equal(t, "no such service", sysErr.Message)
	assert.Empty(t, sysErr.Details)
}
