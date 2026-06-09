package grpcx

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NewClient builds a new [grpc.ClientConn] with sane configurations like observability, insecure transport (for internal communications), etc.
func NewClient(serviceName, addr string, opts ...ClientOpt) (*grpc.ClientConn, error) {
	options := newDefaultClientOptions()
	for _, opt := range opts {
		opt(options)
	}

	clientOpts := make([]grpc.DialOption, 0, len(options.extraOpts)*2)
	if len(options.extraOpts) > 0 {
		clientOpts = append(clientOpts, options.extraOpts...)
	}
	if options.insecure {
		clientOpts = append(clientOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if options.tracerProvider != nil || options.metricProvider != nil {
		otelOptions := make([]otelgrpc.Option, 0, 4)
		otelOptions = append(otelOptions,
			otelgrpc.WithSpanAttributes(
				semconv.ServicePeerName(serviceName),
			),
			otelgrpc.WithMetricAttributes(
				semconv.ServicePeerName(serviceName),
			),
		)
		if options.tracerProvider != nil {
			otelOptions = append(otelOptions, otelgrpc.WithTracerProvider(options.tracerProvider))
		}
		if options.metricProvider != nil {
			otelOptions = append(otelOptions, otelgrpc.WithMeterProvider(options.metricProvider))
		}
		clientOpts = append(clientOpts,
			grpc.WithStatsHandler(
				otelgrpc.NewClientHandler(),
			),
		)
	}
	return grpc.NewClient(addr, clientOpts...)
}

type clientOptions struct {
	insecure       bool
	tracerProvider trace.TracerProvider
	metricProvider metric.MeterProvider
	extraOpts      []grpc.DialOption
}

func newDefaultClientOptions() *clientOptions {
	return &clientOptions{
		insecure: true,
	}
}

type ClientOpt func(*clientOptions)

// DisableInsecureClient disables insecure transport for [grpc.ClientConn] built by [NewClient].
func DisableInsecureClient() ClientOpt {
	return func(co *clientOptions) {
		co.insecure = false
	}
}

// WithClientTracerProvider enables tracing by setting the [trace.TracerProvider] instance to
// an [grpc.ClientConn] built with [NewClient].
func WithClientTracerProvider(p trace.TracerProvider) ServerOption {
	return func(options *serverOptions) error {
		options.tracerProvider = p
		return nil
	}
}

// WithClientMeterProvider enables metrics by setting the [metric.MetricProvider] instance to
// an [grpc.ClientConn] built with [NewClient].
func WithClientMeterProvider(p metric.MeterProvider) ServerOption {
	return func(options *serverOptions) error {
		options.meterProvider = p
		return nil
	}
}

// WithClientOptions appends the given [grpc.DialOption]. [NewClient] will merge these options
// with others set by that routine.
func WithClientOptions(opts ...grpc.DialOption) ClientOpt {
	return func(co *clientOptions) {
		if co.extraOpts == nil {
			co.extraOpts = make([]grpc.DialOption, 0, len(opts))
		}
		co.extraOpts = append(co.extraOpts, opts...)
	}
}
