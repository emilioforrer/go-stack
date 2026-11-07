package observability

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Observability holds the executor and a shutdown function for clean teardown.
type Observability struct {
	executor *Executor
	shutdown func(context.Context) error
}

// Executor returns the configured Executor for use with Run.
func (o *Observability) Executor() *Executor {
	return o.executor
}

// Shutdown flushes and shuts down all configured telemetry providers.
func (o *Observability) Shutdown(ctx context.Context) error {
	if o == nil || o.shutdown == nil {
		return nil
	}
	return o.shutdown(ctx)
}

// Setup initializes observability with the given configuration.
// When cfg.Enabled is false, it returns an Observability with a no-op executor.
func Setup(ctx context.Context, cfg Config) (*Observability, error) {
	cfg = cfg.WithDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid observability config: %w", err)
	}

	if !cfg.Enabled {
		return disabledObservability(cfg)
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	tracer, meter, shutdownFuncs, err := initInstruments(ctx, cfg, res)
	if err != nil {
		return nil, err
	}

	executor, err := NewExecutorFromInstruments(cfg.Enabled, cfg.TracingEnabled, cfg.MetricsEnabled, tracer, meter)
	if err != nil {
		return nil, fmt.Errorf("create executor: %w", err)
	}

	shutdown := func(ctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFuncs {
			if err := fn(ctx); err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}

	// Configure global text-map propagation for distributed tracing.
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return &Observability{
		executor: executor,
		shutdown: shutdown,
	}, nil
}

// disabledObservability returns an Observability with a no-op executor for a
// configuration that is completely disabled.
func disabledObservability(cfg Config) (*Observability, error) {
	tracer := noop.NewTracerProvider().Tracer(cfg.ServiceName)
	executor, err := NewExecutorFromInstruments(false, false, false, tracer, nil)
	if err != nil {
		return nil, fmt.Errorf("create no-op executor: %w", err)
	}

	return &Observability{
		executor: executor,
		shutdown: func(context.Context) error { return nil },
	}, nil
}

// initInstruments initializes the tracer and meter according to cfg, returning
// the instruments and the provider shutdown functions. A no-op tracer is
// returned when tracing is disabled, and a nil meter when metrics are disabled.
func initInstruments(
	ctx context.Context,
	cfg Config,
	res *resource.Resource,
) (trace.Tracer, otelmetric.Meter, []func(context.Context) error, error) {
	tracer := noop.NewTracerProvider().Tracer(cfg.ServiceName)

	var shutdownFuncs []func(context.Context) error

	if cfg.TracingEnabled {
		traceShutdown, t, err := initTracing(ctx, cfg, res)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("init tracing: %w", err)
		}
		shutdownFuncs = append(shutdownFuncs, traceShutdown)
		tracer = t
	}

	var meter otelmetric.Meter
	if cfg.MetricsEnabled {
		mp, metricShutdown, err := initMetrics(ctx, cfg, res)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("init metrics: %w", err)
		}
		shutdownFuncs = append(shutdownFuncs, metricShutdown)
		meter = mp.Meter("github.com/emilioforrer/go-stack/pkg/observability")
	}

	return tracer, meter, shutdownFuncs, nil
}

func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

func initTracing(ctx context.Context, cfg Config, res *resource.Resource) (func(context.Context) error, trace.Tracer, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithMaxQueueSize(2048),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(newSampler(cfg.SampleRatio)),
	)

	otel.SetTracerProvider(tp)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}

	return shutdown, tp.Tracer("github.com/emilioforrer/go-stack/pkg/observability"), nil
}

func initMetrics(ctx context.Context, cfg Config, res *resource.Resource) (*metric.MeterProvider, func(context.Context) error, error) {
	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("create OTLP metric exporter: %w", err)
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(exporter)),
		metric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	shutdown := func(ctx context.Context) error {
		return mp.Shutdown(ctx)
	}

	return mp, shutdown, nil
}

func newSampler(ratio float64) sdktrace.Sampler {
	switch {
	case ratio <= 0:
		return sdktrace.NeverSample()
	case ratio >= 1:
		return sdktrace.AlwaysSample()
	default:
		return sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(ratio),
		)
	}
}
