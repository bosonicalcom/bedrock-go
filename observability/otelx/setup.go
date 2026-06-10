package otelx

import (
	"cmp"
	"context"
	"errors"
	"time"

	otelruntime "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// Instruments holds the OpenTelemetry meter and tracer providers.
type Instruments struct {
	MeterProvider  metric.MeterProvider
	TracerProvider trace.TracerProvider
}

// NewInstrumentsNoop builds [Instruments] with no-operation instances.
//
// Safe to call at any time.
func NewInstrumentsNoop() *Instruments {
	return &Instruments{
		MeterProvider:  metricnoop.NewMeterProvider(),
		TracerProvider: tracenoop.NewTracerProvider(),
	}
}

// SetupSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupSDK(ctx context.Context, serviceName string, opts ...SetupSDKOpt) (*Instruments, func(context.Context) error, error) {
	options := &setupSDKOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var shutdownFuncs []func(context.Context) error
	var err error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// handleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, shutdown(ctx))
	}

	// Setup shared resource.

	sharedResource, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceNamespace(cmp.Or(options.platform, "unknown")),
			semconv.ServiceVersion(cmp.Or(options.version, "v0.1.0")),
			attribute.String("service.environment", cmp.Or(options.environment, "unknown")),
		),
	)
	if err != nil {
		handleErr(err)
		return nil, shutdown, err
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	// Set up trace provider.
	tracerProvider, err := newTracerProvider(ctx, sharedResource)
	if err != nil {
		handleErr(err)
		return nil, shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
	otel.SetTracerProvider(tracerProvider)

	// Set up meter provider.
	meterProvider, err := newMeterProvider(ctx, sharedResource)
	if err != nil {
		handleErr(err)
		return nil, shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
	otel.SetMeterProvider(meterProvider)

	if err = otelruntime.Start(otelruntime.WithMeterProvider(meterProvider)); err != nil {
		handleErr(err)
		return nil, shutdown, err
	}

	return &Instruments{
		MeterProvider:  meterProvider,
		TracerProvider: tracerProvider,
	}, shutdown, nil
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func newTracerProvider(ctx context.Context, sharedResource *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter,
			// Default is 5s.
			sdktrace.WithBatchTimeout(time.Second*5)),
		sdktrace.WithResource(sharedResource),
	)
	return tracerProvider, nil
}

func newMeterProvider(ctx context.Context, sharedResource *resource.Resource) (*sdkmetric.MeterProvider, error) {
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			// Default is 1m.
			sdkmetric.WithInterval(time.Minute))),
		sdkmetric.WithResource(sharedResource),
	)
	return meterProvider, nil
}

type setupSDKOptions struct {
	environment string
	platform    string
	version     string
}

type SetupSDKOpt func(*setupSDKOptions)

// WithEnvironment sets the service environment for the SDK.
func WithEnvironment(environment string) SetupSDKOpt {
	return func(opts *setupSDKOptions) {
		opts.environment = environment
	}
}

// WithPlatform sets the service platform (namespace) for the SDK.
func WithPlatform(platform string) SetupSDKOpt {
	return func(opts *setupSDKOptions) {
		opts.platform = platform
	}
}

// WithVersion sets the service version for the SDK.
func WithVersion(version string) SetupSDKOpt {
	return func(opts *setupSDKOptions) {
		opts.version = version
	}
}
