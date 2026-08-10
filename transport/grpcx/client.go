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

	clientOpts := make([]grpc.DialOption, 0, len(options.extraOpts)+4)
	if len(options.extraOpts) > 0 {
		clientOpts = append(clientOpts, options.extraOpts...)
	}
	if !options.disableSyserr {
		// Appended last so the conversion sits innermost, closest to the wire: interceptors
		// supplied through WithClientOptions then observe an already-converted *syserr.Error.
		unary, stream := SyserrClientInterceptor()
		clientOpts = append(clientOpts,
			grpc.WithChainUnaryInterceptor(unary),
			grpc.WithChainStreamInterceptor(stream),
		)
	}
	if options.insecure {
		clientOpts = append(clientOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if options.tracerProvider != nil || options.meterProvider != nil {
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
		if options.meterProvider != nil {
			otelOptions = append(otelOptions, otelgrpc.WithMeterProvider(options.meterProvider))
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
	disableSyserr  bool
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
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

// DisableClientSyserrInterceptor disables [SyserrClientInterceptor] on a [grpc.ClientConn]
// built by [NewClient], leaving raw gRPC status errors to the caller.
func DisableClientSyserrInterceptor() ClientOpt {
	return func(co *clientOptions) {
		co.disableSyserr = true
	}
}

// WithClientTracerProvider enables tracing by setting the [trace.TracerProvider] instance to
// an [grpc.ClientConn] built with [NewClient].
func WithClientTracerProvider(p trace.TracerProvider) ClientOpt {
	return func(options *clientOptions) {
		options.tracerProvider = p
	}
}

// WithClientMeterProvider enables metrics by setting the [metric.MetricProvider] instance to
// an [grpc.ClientConn] built with [NewClient].
func WithClientMeterProvider(p metric.MeterProvider) ClientOpt {
	return func(options *clientOptions) {
		options.meterProvider = p
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
