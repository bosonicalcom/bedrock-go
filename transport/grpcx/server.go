package grpcx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	grpcmiddleware_logging "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"

	"github.com/bosonicalcom/bedrock-go/proc"
)

// NewServer allocates a [grpc.Server]. Customize its behavior by passing [ServerOption] arguments.
func NewServer(opts ...ServerOption) (*grpc.Server, error) {
	options := newDefaultServerOptions()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	// Build interceptor chain: outermost first.
	//   Identifier → ErrorLog → SpanAnnotator → Metric → Syserr → Recovery → custom
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	addPair := func(u grpc.UnaryServerInterceptor, s grpc.StreamServerInterceptor) {
		unaryInterceptors = append(unaryInterceptors, u)
		streamInterceptors = append(streamInterceptors, s)
	}

	// 1. Identifier — creates observation, injects request/correlation IDs
	addPair(IdentifierServerInterceptor())

	// 2. Access logging via grpc-middleware (if logger set), skipping health check methods.
	// grpc-middleware/v2 has no built-in decider API, so we wrap the interceptors.
	if options.logger != nil {
		logOpts := []grpcmiddleware_logging.Option{
			grpcmiddleware_logging.WithLogOnEvents(grpcmiddleware_logging.StartCall, grpcmiddleware_logging.FinishCall),
			grpcmiddleware_logging.WithLevels(grpcmiddleware_logging.DefaultServerCodeToLevel),
		}
		baseUnary := grpcmiddleware_logging.UnaryServerInterceptor(SlogAdapter(options.logger), logOpts...)
		baseStream := grpcmiddleware_logging.StreamServerInterceptor(SlogAdapter(options.logger), logOpts...)
		addPair(
			func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				if isHealthCheckMethod(info.FullMethod) {
					return handler(ctx, req)
				}
				return baseUnary(ctx, req, info, handler)
			},
			func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
				if isHealthCheckMethod(info.FullMethod) {
					return handler(srv, ss)
				}
				return baseStream(srv, ss, info, handler)
			},
		)

		// 3. Error log — emits a rich slog error entry when a syserr is recorded
		addPair(ErrorLogServerInterceptor(options.logger))
	}

	// 4. Span annotator — adds syserr attributes to the otelgrpc span (if tracing enabled)
	if options.enableStats {
		addPair(SpanAnnotatorServerInterceptor())
	}

	// 6. Syserr conversion — converts *syserr.Error returns into gRPC status errors
	addPair(SyserrServerInterceptor())

	// 7. Recovery — catches panics and converts to gRPC status errors (always on)
	recoveryOpt := recovery.WithRecoveryHandlerContext(PanicRecoveryHandler)
	addPair(
		recovery.UnaryServerInterceptor(recoveryOpt),
		recovery.StreamServerInterceptor(recoveryOpt),
	)

	// 8. Custom user interceptors (innermost)
	unaryInterceptors = append(unaryInterceptors, options.unaryInterceptors...)
	streamInterceptors = append(streamInterceptors, options.streamInterceptors...)

	// Assemble grpc.Server options
	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	}
	grpcOpts = append(grpcOpts, options.serverOptions...)
	if options.enableStats {
		grpcOpts = append(grpcOpts,
			grpc.StatsHandler(
				otelgrpc.NewServerHandler(
					otelgrpc.WithFilter(func(ri *stats.RPCTagInfo) bool {
						return !isHealthCheckMethod(ri.FullMethodName)
					}),
				),
			),
		)
	}

	srv := grpc.NewServer(grpcOpts...)

	// Register controllers
	if options.enableHealth {
		options.controllers = append(options.controllers, NewHealthController())
	}
	for _, ctrl := range options.controllers {
		ctrl.RegisterServers(srv)
	}

	return srv, nil
}

// NewServerBackgroundProcess allocates a [proc.BackgroundProcess] that runs a [grpc.Server].
func NewServerBackgroundProcess(srv *grpc.Server, addr string) proc.BackgroundProcess {
	return proc.BackgroundProcess{
		Name: "grpc.server",
		RunFunc: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("bedrock.grpcx: listen %s: %w", addr, err)
			}
			err = srv.Serve(lis)
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			return err
		},
		ShutdownFunc: func(ctx context.Context) error {
			srv.GracefulStop()
			return nil
		},
	}
}

// -- Options

type serverOptions struct {
	logger             *slog.Logger
	logLevel           slog.Level
	enableStats        bool
	enableHealth       bool
	controllers        []Controller
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	serverOptions      []grpc.ServerOption
}

func newDefaultServerOptions() *serverOptions {
	return &serverOptions{
		logLevel:           slog.LevelInfo,
		enableHealth:       true,
		controllers:        make([]Controller, 0),
		unaryInterceptors:  make([]grpc.UnaryServerInterceptor, 0),
		streamInterceptors: make([]grpc.StreamServerInterceptor, 0),
		serverOptions:      make([]grpc.ServerOption, 0),
	}
}

// ServerOption is an optional routine that enables a more granular way of configuring a [grpc.Server].
type ServerOption func(options *serverOptions) error

// WithServerLogger sets the [slog.Logger] for a [grpc.Server] allocated by [NewServer].
// Automatically enables access logging and error logging.
func WithServerLogger(logger *slog.Logger) ServerOption {
	return func(options *serverOptions) error {
		options.logger = logger
		return nil
	}
}

// WithServerLogLevel sets the log level for a [grpc.Server] allocated by [NewServer].
func WithServerLogLevel(lvl slog.Level) ServerOption {
	return func(options *serverOptions) error {
		options.logLevel = lvl
		return nil
	}
}

// WithServerControllers sets the [Controller](s) for a [grpc.Server] allocated by [NewServer].
func WithServerControllers(ctrl []Controller) ServerOption {
	return func(options *serverOptions) error {
		if ctrl == nil {
			return errors.New("bedrock.grpcx: controllers cannot be nil")
		}
		options.controllers = ctrl
		return nil
	}
}

// WithServerInterceptors appends custom unary [grpc.UnaryServerInterceptor](s) to the server.
func WithServerInterceptors(interceptors ...grpc.UnaryServerInterceptor) ServerOption {
	return func(options *serverOptions) error {
		options.unaryInterceptors = append(options.unaryInterceptors, interceptors...)
		return nil
	}
}

// WithServerStreamInterceptors appends custom stream [grpc.StreamServerInterceptor](s) to the server.
func WithServerStreamInterceptors(interceptors ...grpc.StreamServerInterceptor) ServerOption {
	return func(options *serverOptions) error {
		options.streamInterceptors = append(options.streamInterceptors, interceptors...)
		return nil
	}
}

// EnableServerTelemetry enables OpenTelemetry metrics + tracing for a [grpc.Server] allocated by [NewServer].
// Uses otelgrpc stats handler for span creation and a span annotator for syserr attributes.
func EnableServerTelemetry() ServerOption {
	return func(options *serverOptions) error {
		options.enableStats = true
		return nil
	}
}

// DisableServerHealthExport disables the automatic gRPC health protocol export.
func DisableServerHealthExport() ServerOption {
	return func(options *serverOptions) error {
		options.enableHealth = false
		return nil
	}
}

// WithServerOptions appends custom [grpc.ServerOption](s) to the server.
func WithServerOptions(options ...grpc.ServerOption) ServerOption {
	return func(serverOptions *serverOptions) error {
		serverOptions.serverOptions = append(serverOptions.serverOptions, options...)
		return nil
	}
}
