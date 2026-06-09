package httpx

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/bosonicalcom/bedrock-go/proc"
)

// ServerConfig represents a configuration for an [http.Server].
type ServerConfig struct {
	Addr string
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
	// ordering matters, first=outermost
	interceptors := make([]ServerInterceptor, 0, 4+len(options.interceptors))
	interceptors = append(interceptors, IdentifierServerInterceptor())
	if options.logger != nil {
		interceptors = append(interceptors,
			LogServerInterceptor(options.logger, options.logLevel,
				WithServerSkipper(skipHealth),
			),
		)
	}
	if options.tracerProvider != nil {
		interceptors = append(interceptors, TraceServerInterceptor(WithServerSkipper(skipHealth)))
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
	if options.tracerProvider != nil || options.meterProvider != nil {
		otelOpts := []otelhttp.Option{
			otelhttp.WithFilter(func(r *http.Request) bool {
				return !skipHealth(r)
			}),
		}
		if options.tracerProvider != nil {
			otelOpts = append(otelOpts, otelhttp.WithTracerProvider(options.tracerProvider))
		}
		if options.meterProvider != nil {
			otelOpts = append(otelOpts, otelhttp.WithMeterProvider(options.meterProvider))
		}
		handlerHTTP = otelhttp.NewHandler(handlerHTTP, cmp.Or(options.name, "http.server"), otelOpts...)
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
	name               string
	enableLocalization bool
	logger             *slog.Logger
	logLevel           slog.Level
	tracerProvider     trace.TracerProvider
	meterProvider      metric.MeterProvider
	enableHealth       bool
	interceptors       []ServerInterceptor
	controllers        []Controller
}

func newDefaultServerOptions() *serverOptions {
	return &serverOptions{
		config:             newDefaultServerConfig(),
		name:               "http.server",
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

// WithServerName sets the name of the [http.Server] allocated by [NewServer].
func WithServerName(name string) ServerOption {
	return func(options *serverOptions) error {
		options.name = name
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

// WithServerTracerProvider enables tracing by setting the [trace.TracerProvider] instance to
// an [http.Server] built with [NewServer].
func WithServerTracerProvider(p trace.TracerProvider) ServerOption {
	return func(options *serverOptions) error {
		options.tracerProvider = p
		return nil
	}
}

// WithServerMeterProvider enables metrics by setting the [metric.MetricProvider] instance to
// an [http.Server] built with [NewServer].
func WithServerMeterProvider(p metric.MeterProvider) ServerOption {
	return func(options *serverOptions) error {
		options.meterProvider = p
		return nil
	}
}

func skipHealth(r *http.Request) bool {
	return r.URL.Path == "/healthz" || r.URL.Path == "/readyz"
}
