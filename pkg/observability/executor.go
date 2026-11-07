package observability

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Executor controls whether tracing and metrics are enabled and holds the
// OpenTelemetry instruments used by Run.
type Executor struct {
	enabled       atomic.Bool
	traceEnabled  atomic.Bool
	metricEnabled atomic.Bool

	tracer trace.Tracer

	// OTel instruments
	duration metric.Float64Histogram
	errors   metric.Int64Counter
}

// NewExecutorFromInstruments creates an Executor from OpenTelemetry metric instruments.
func NewExecutorFromInstruments(
	enabled bool,
	tracingEnabled bool,
	metricsEnabled bool,
	tracer trace.Tracer,
	meter metric.Meter,
) (*Executor, error) {
	e := &Executor{
		tracer: tracer,
	}

	e.enabled.Store(enabled)
	e.traceEnabled.Store(tracingEnabled)
	e.metricEnabled.Store(metricsEnabled)

	if meter != nil {
		duration, err := meter.Float64Histogram(
			"application.operation.duration",
			metric.WithUnit("s"),
			metric.WithDescription("Duration of instrumented application operations"),
		)
		if err != nil {
			return nil, fmt.Errorf("create duration histogram: %w", err)
		}
		e.duration = duration

		errorsCounter, err := meter.Int64Counter(
			"application.operation.errors",
			metric.WithDescription("Number of failed application operations"),
		)
		if err != nil {
			return nil, fmt.Errorf("create error counter: %w", err)
		}
		e.errors = errorsCounter
	}

	return e, nil
}

func (e *Executor) Enable()         { e.enabled.Store(true) }
func (e *Executor) Disable()        { e.enabled.Store(false) }
func (e *Executor) EnableTracing()  { e.traceEnabled.Store(true) }
func (e *Executor) DisableTracing() { e.traceEnabled.Store(false) }
func (e *Executor) EnableMetrics()  { e.metricEnabled.Store(true) }
func (e *Executor) DisableMetrics() { e.metricEnabled.Store(false) }

// Run executes fn with optional tracing and metrics.
func Run[T any](
	e *Executor,
	ctx context.Context,
	name string,
	fn Handler[T],
	options ...RunOption,
) (T, error) {
	if !e.enabled.Load() {
		return fn(ctx)
	}

	traceEnabled := e.traceEnabled.Load()
	metricEnabled := e.metricEnabled.Load()

	if !traceEnabled && !metricEnabled {
		return fn(ctx)
	}

	cfg := &runConfig{}
	for _, option := range options {
		option(cfg)
	}

	var m []Middleware[T]

	if traceEnabled {
		m = append(m, tracing[T](e, name, cfg))
	}

	if metricEnabled {
		m = append(m, metrics[T](e, name, cfg))
	}

	if len(m) == 0 {
		return fn(ctx)
	}

	return Chain(m...)(fn)(ctx)
}

// panicError wraps a recovered panic value so it can be recorded on a span
// without losing the original value or message format.
type panicError struct {
	value any
}

func (e panicError) Error() string {
	return fmt.Sprintf("panic: %v", e.value)
}

// RunWithRecovery executes fn with tracing, metrics, and panic recording.
// The panic is re-raised after recording it on the span.
func RunWithRecovery[T any](
	e *Executor,
	ctx context.Context,
	name string,
	fn Handler[T],
	options ...RunOption,
) (result T, err error) {
	if !e.enabled.Load() {
		return fn(ctx)
	}

	traceEnabled := e.traceEnabled.Load()
	metricEnabled := e.metricEnabled.Load()

	if !traceEnabled && !metricEnabled {
		return fn(ctx)
	}

	cfg := &runConfig{}
	for _, option := range options {
		option(cfg)
	}

	var span trace.Span
	if traceEnabled {
		attrs := collectAttributes(ctx, cfg)
		ctx, span = e.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
		defer span.End()
	}

	start := time.Now()

	if traceEnabled {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			span.RecordError(panicError{value: recovered})
			span.SetStatus(codesError, "panic")
			panic(recovered)
		}()
	}

	result, err = fn(ctx)

	if traceEnabled && err != nil {
		span.RecordError(err)
		span.SetStatus(codesError, err.Error())
	}

	if metricEnabled {
		e.recordRunMetrics(ctx, name, start, cfg, err)
	}

	return result, err
}

// recordRunMetrics records the operation duration and, when err is non-nil,
// an error count for a completed instrumented run.
func (e *Executor) recordRunMetrics(
	ctx context.Context,
	name string,
	start time.Time,
	cfg *runConfig,
	err error,
) {
	duration := time.Since(start).Seconds()

	attrs := make([]attribute.KeyValue, 0, 1)
	attrs = append(attrs, attribute.String("operation.name", name))
	attrs = append(attrs, collectAttributes(ctx, cfg)...)

	if e.duration != nil {
		e.duration.Record(ctx, duration, metric.WithAttributes(attrs...))
	}

	if err != nil {
		e.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}
