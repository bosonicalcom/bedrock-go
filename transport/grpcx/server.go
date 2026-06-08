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
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"

	"github.com/bosonicalcom/bedrock-go/proc"
)

// ServerConfig represents the configuration for a gRPC server.
type ServerConfig struct {
	Addr string `env:"GRPC_SERVER_ADDR"`
}

func newDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Addr: ":0", // random port
	}
}

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

	// 2. Access logging via grpc-middleware (if logger set)
	if options.logger != nil {
		logOpts := []grpcmiddleware_logging.Option{
			grpcmiddleware_logging.WithLogOnEvents(grpcmiddleware_logging.StartCall, grpcmiddleware_logging.FinishCall),
			grpcmiddleware_logging.WithLevels(grpcmiddleware_logging.DefaultServerCodeToLevel),
		}
		addPair(
			grpcmiddleware_logging.UnaryServerInterceptor(SlogAdapter(options.logger), logOpts...),
			grpcmiddleware_logging.StreamServerInterceptor(SlogAdapter(options.logger), logOpts...),
		)

		// 3. Error log — emits a rich slog error entry when a syserr is recorded
		addPair(ErrorLogServerInterceptor(options.logger))
	}

	// 4. Span annotator — adds syserr attributes to the otelgrpc span (if tracing enabled)
	if options.enableTracing {
		addPair(SpanAnnotatorServerInterceptor())
	}

	// 5. Metric counter (if meter set)
	if options.meter != nil {
		counter, err := options.meter.Int64Counter("grpc.server.errors")
		if err != nil {
			return nil, fmt.Errorf("bedrock.grpcx: cannot create otel error counter: %w", err)
		}
		addPair(MetricServerInterceptor(counter))
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
	if options.enableTracing {
		grpcOpts = append(grpcOpts, grpc.StatsHandler(otelgrpc.NewServerHandler()))
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
	config             *ServerConfig
	logger             *slog.Logger
	logLevel           slog.Level
	enableTracing      bool
	meter              metric.Meter
	enableHealth       bool
	controllers        []Controller
	unaryInterceptors  []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
}

func newDefaultServerOptions() *serverOptions {
	return &serverOptions{
		config:             newDefaultServerConfig(),
		logLevel:           slog.LevelInfo,
		enableHealth:       true,
		controllers:        make([]Controller, 0),
		unaryInterceptors:  make([]grpc.UnaryServerInterceptor, 0),
		streamInterceptors: make([]grpc.StreamServerInterceptor, 0),
	}
}

// ServerOption is an optional routine that enables a more granular way of configuring a [grpc.Server].
type ServerOption func(options *serverOptions) error

// WithServerConfig sets the server configuration for a [grpc.Server] allocated by [NewServer].
func WithServerConfig(cfg *ServerConfig) ServerOption {
	return func(options *serverOptions) error {
		if cfg == nil {
			return errors.New("bedrock.grpcx: server config cannot be nil")
		}
		options.config = cfg
		return nil
	}
}

// WithServerAddress sets the listen address for a [grpc.Server] allocated by [NewServer].
func WithServerAddress(addr string) ServerOption {
	return func(options *serverOptions) error {
		if addr == "" {
			return errors.New("bedrock.grpcx: server address cannot be empty")
		}
		options.config.Addr = addr
		return nil
	}
}

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

// EnableServerTracing enables OpenTelemetry tracing for a [grpc.Server] allocated by [NewServer].
// Uses otelgrpc stats handler for span creation and a span annotator for syserr attributes.
func EnableServerTracing() ServerOption {
	return func(options *serverOptions) error {
		options.enableTracing = true
		return nil
	}
}

// EnableServerMetrics enables OpenTelemetry metrics for a [grpc.Server] allocated by [NewServer].
// Increments a grpc.server.errors counter for every syserr returned by a handler.
func EnableServerMetrics(meter metric.Meter) ServerOption {
	return func(options *serverOptions) error {
		options.meter = meter
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
