package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"

	"github.com/bosonicalcom/bedrock-go/proc"
)

// ServerConfig represents a configuration for an [http.Server].
type ServerConfig struct {
	Addr string `env:"HTTP_SERVER_ADDR"`
}

func newDefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Addr: ":0", // random port
	}
}

// NewServer allocates an [http.Server]. Customize its behavior by passing [ServerOption] arguments.
func NewServer(opts ...ServerOption) (*http.Server, error) {
	options := newDefaultServerOptions()
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return nil, err
		}
	}

	// setup interceptors
	// - base interceptors
	// ordering matter, first=outermost
	interceptors := make([]ServerInterceptor, 0, 4+len(options.interceptors))
	interceptors = append(interceptors, IdentifierServerInterceptor())
	if options.logger != nil {
		interceptors = append(interceptors, LogServerInterceptor(options.logger, options.logLevel))
	}
	if options.enableTracing {
		interceptors = append(interceptors, TraceServerInterceptor())
	}
	if options.meter != nil {
		counter, err := options.meter.Int64Counter("http.server.errors")
		if err != nil {
			return nil, fmt.Errorf("bedrock.httpx: cannot create otel error counter")
		}
		interceptors = append(interceptors, MetricServerInterceptor(counter))
	}
	interceptors = append(interceptors, RecoverServerInterceptor())
	// if options.enableLocalization {
	// 	interceptors = append(interceptors, LanguageLoaderServerInterceptor())
	// }
	if len(options.interceptors) > 0 {
		// custom interceptors
		interceptors = append(interceptors, options.interceptors...)
	}

	// setup controllers
	mux := http.NewServeMux()
	if options.enableHealth {
		// options.controllers is always non-nil thanks to newDefaultServerOptions and WithControllers safety nil-check.
		options.controllers = append(options.controllers, HealthController{})
	}
	for i := range options.controllers {
		options.controllers[i].RegisterEndpoints(mux)
	}

	// finish server setup
	handlerHTTP := ServerInterceptorChain(mux, interceptors...)
	if options.enableTracing {
		handlerHTTP = otelhttp.NewHandler(handlerHTTP, "http.server")
	}
	return &http.Server{
		Addr:    options.config.Addr,
		Handler: handlerHTTP,
	}, nil
}

// NewServerBackgroundProcess allocates a new [proc.BackgroundProcess] that runs an [http.Server].
func NewServerBackgroundProcess(srv *http.Server) proc.BackgroundProcess {
	return proc.BackgroundProcess{
		Name: "http.server",
		RunFunc: func(ctx context.Context) error {
			err := srv.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		},
		ShutdownFunc: func(ctx context.Context) error {
			return srv.Shutdown(ctx)
		},
	}
}

// - Options

type serverOptions struct {
	config             *ServerConfig
	enableLocalization bool
	logger             *slog.Logger
	enableTracing      bool
	meter              metric.Meter
	logLevel           slog.Level
	enableHealth       bool
	interceptors       []ServerInterceptor
	controllers        []Controller
}

func newDefaultServerOptions() *serverOptions {
	return &serverOptions{
		config:             newDefaultServerConfig(),
		enableLocalization: false,
		logLevel:           slog.LevelInfo,
		enableHealth:       true,
		interceptors:       make([]ServerInterceptor, 0),
		controllers:        make([]Controller, 0),
	}
}

// ServerOption is an optional routine that enables a more granular way of configuring an [http.Server].
type ServerOption func(options *serverOptions) error

// WithServerConfig sets the server configuration for an [http.Server] allocated by [NewServer].
func WithServerConfig(cfg *ServerConfig) ServerOption {
	return func(options *serverOptions) error {
		if cfg == nil {
			return errors.New("bedrock.httpx: server config cannot be nil")
		}
		options.config = cfg
		return nil
	}
}

// WithServerAddress sets a custom address of an [http.Server] allocated by [NewServer].
func WithServerAddress(addr string) ServerOption {
	return func(options *serverOptions) error {
		if addr == "" {
			return errors.New("bedrock.httpx: server address cannot be empty")
		}
		options.config.Addr = addr
		return nil
	}
}

// WithServerLogger sets the [slog.Logger] of logging for an [http.Server] allocated by [NewServer].
//
// Automatically enables logging if not already enabled.
func WithServerLogger(logger *slog.Logger) ServerOption {
	return func(options *serverOptions) error {
		options.logger = logger
		return nil
	}
}

// WithServerLogLevel sets the [slog.Level] of logging for an [http.Server] allocated by [NewServer].
func WithServerLogLevel(lvl slog.Level) ServerOption {
	return func(options *serverOptions) error {
		options.logLevel = lvl
		return nil
	}
}

// EnableServerLocalization enables localization for an [http.Server] allocated by [NewServer].
//
// This routine requires that localization dependencies are properly initialized and configured (prior loading of
// localization bundles).
func EnableServerLocalization() ServerOption {
	return func(options *serverOptions) error {
		options.enableLocalization = true
		return nil
	}
}

// DisableServerHealthExport disables health endpoint export for the [http.Server] allocated by [NewServer].
func DisableServerHealthExport() ServerOption {
	return func(options *serverOptions) error {
		options.enableHealth = false
		return nil
	}
}

// WithServerControllers sets the [Controller](s) for an [http.Server] allocated by [NewServer].
//
// This routine will cause the server to register the provided [Controller](s) with the server's router ([http.ServeMux]).
func WithServerControllers(ctrl []Controller) ServerOption {
	return func(options *serverOptions) error {
		if ctrl == nil {
			return errors.New("bedrock.httpx: controllers cannot be nil")
		}

		options.controllers = ctrl
		return nil
	}
}

// WithServerInterceptors appends one or many [ServerInterceptor](s) to the [http.Server] allocated by [NewServer].
func WithServerInterceptors(interceptors ...ServerInterceptor) ServerOption {
	return func(options *serverOptions) error {
		if options.interceptors == nil {
			options.interceptors = make([]ServerInterceptor, 0, len(interceptors))
		}
		options.interceptors = append(options.interceptors, interceptors...)
		return nil
	}
}

// EnableServerTracing enables tracing capabilities (using OpenTelemetry) to an [http.Server] allocated by [NewServer].
func EnableServerTracing() ServerOption {
	return func(options *serverOptions) error {
		options.enableTracing = true
		return nil
	}
}

// EnableServerMetrics enables metrics capabilities (using OpenTelemetry) to an [http.Server] allocated by [NewServer].
func EnableServerMetrics(meter metric.Meter) ServerOption {
	return func(options *serverOptions) error {
		options.meter = meter
		return nil
	}
}
