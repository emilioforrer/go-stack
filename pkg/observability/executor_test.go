package observability

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
)

// newTestExecutor creates an Executor with a no-op tracer and real meter.
func newTestExecutor(enabled bool) *Executor {
	tracer := noop.NewTracerProvider().Tracer("test")
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	exec, err := NewExecutorFromInstruments(enabled, enabled, enabled, tracer, mp.Meter("test"))
	if err != nil {
		panic(err)
	}
	return exec
}

// newTestExecutorWithInstruments creates an Executor with real tracer and meter for inspection.
func newTestExecutorWithInstruments(enabled, tracing, metrics bool) (*Executor, *tracetest.SpanRecorder, *metric.ManualReader) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	tracer := tp.Tracer("test")
	if !tracing {
		tracer = noop.NewTracerProvider().Tracer("test")
	}

	var meter otelmetric.Meter
	if metrics {
		meter = mp.Meter("test")
	}

	exec, err := NewExecutorFromInstruments(enabled, tracing, metrics, tracer, meter)
	if err != nil {
		panic(err)
	}
	return exec, sr, reader
}

func TestExecutor_enabled(t *testing.T) {
	e := newTestExecutor(true)
	if !e.enabled.Load() {
		t.Fatal("expected enabled=true")
	}
	if !e.traceEnabled.Load() {
		t.Fatal("expected traceEnabled=true")
	}
	if !e.metricEnabled.Load() {
		t.Fatal("expected metricEnabled=true")
	}
}

func TestExecutor_disabled(t *testing.T) {
	e := newTestExecutor(false)
	if e.enabled.Load() {
		t.Fatal("expected enabled=false")
	}
	if e.traceEnabled.Load() {
		t.Fatal("expected traceEnabled=false")
	}
	if e.metricEnabled.Load() {
		t.Fatal("expected metricEnabled=false")
	}
}

func TestToggle_EnableDisable(t *testing.T) {
	e := newTestExecutor(false)

	e.Enable()
	if !e.enabled.Load() {
		t.Fatal("expected enabled after Enable()")
	}
	e.Disable()
	if e.enabled.Load() {
		t.Fatal("expected disabled after Disable()")
	}
}

func TestToggle_Tracing(t *testing.T) {
	e := newTestExecutor(false)

	e.EnableTracing()
	if !e.traceEnabled.Load() {
		t.Fatal("expected traceEnabled after EnableTracing()")
	}
	e.DisableTracing()
	if e.traceEnabled.Load() {
		t.Fatal("expected traceDisabled after DisableTracing()")
	}
}

func TestToggle_Metrics(t *testing.T) {
	e := newTestExecutor(false)

	e.EnableMetrics()
	if !e.metricEnabled.Load() {
		t.Fatal("expected metricEnabled after EnableMetrics()")
	}
	e.DisableMetrics()
	if e.metricEnabled.Load() {
		t.Fatal("expected metricDisabled after DisableMetrics()")
	}
}

func TestRun_disabled_fast_path(t *testing.T) {
	e := newTestExecutor(false)

	res, err := Run(e, context.Background(), "op", func(ctx context.Context) (string, error) {
		return "fast", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "fast" {
		t.Fatalf("got %q, want %q", res, "fast")
	}
}

func TestRun_all_enabled(t *testing.T) {
	e, _, reader := newTestExecutorWithInstruments(true, false, true)

	res, err := Run(e, context.Background(), "op.all", func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	var count uint64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if d, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, dp := range d.DataPoints {
					count += dp.Count
				}
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 histogram data point, got %d", count)
	}
}

func TestRun_all_enabled_with_error(t *testing.T) {
	e, _, reader := newTestExecutorWithInstruments(true, false, true)

	_, err := Run(e, context.Background(), "op.err", func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	var histCount uint64
	var errCount int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, dp := range d.DataPoints {
					histCount += dp.Count
				}
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					errCount += dp.Value
				}
			}
		}
	}
	if histCount != 1 {
		t.Fatalf("expected histogram count 1, got %d", histCount)
	}
	if errCount != 1 {
		t.Fatalf("expected error count 1, got %d", errCount)
	}
}

func TestRun_tracing_only(t *testing.T) {
	e, sr, _ := newTestExecutorWithInstruments(true, true, false)

	res, err := Run(e, context.Background(), "op.trace", func(ctx context.Context) (string, error) {
		return "traced", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "traced" {
		t.Fatalf("got %q, want %q", res, "traced")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestRun_metrics_only(t *testing.T) {
	e, _, reader := newTestExecutorWithInstruments(true, false, true)

	res, err := Run(e, context.Background(), "op.metric", func(ctx context.Context) (string, error) {
		return "metered", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "metered" {
		t.Fatalf("got %q, want %q", res, "metered")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	var count uint64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if d, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, dp := range d.DataPoints {
					count += dp.Count
				}
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 histogram data point, got %d", count)
	}
}

func TestRun_enabled_but_both_sub_toggles_off(t *testing.T) {
	e := newTestExecutor(true)
	e.DisableTracing()
	e.DisableMetrics()

	res, err := Run(e, context.Background(), "op.noop", func(ctx context.Context) (string, error) {
		return "noop", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "noop" {
		t.Fatalf("got %q, want %q", res, "noop")
	}
}

func TestRun_with_context_attributes(t *testing.T) {
	e := newTestExecutor(true)

	ctx := WithRequestID(context.Background(), "req-1")
	ctx = WithUserID(ctx, "user-1")

	res, err := Run(e, ctx, "op.ctx", func(ctx context.Context) (string, error) {
		return "enriched", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "enriched" {
		t.Fatalf("got %q, want %q", res, "enriched")
	}
}

func TestRun_tracing_with_error_and_context(t *testing.T) {
	e, sr, _ := newTestExecutorWithInstruments(true, true, false)

	ctx := WithRequestID(context.Background(), "req-err")
	ctx = WithUserID(ctx, "user-err")

	_, err := Run(e, ctx, "op.trace.err", func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestRun_metrics_with_error(t *testing.T) {
	e, _, reader := newTestExecutorWithInstruments(true, false, true)

	_, err := Run(e, context.Background(), "op.metric.err", func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("fail")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	var histCount uint64
	var errCount int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, dp := range d.DataPoints {
					histCount += dp.Count
				}
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					errCount += dp.Value
				}
			}
		}
	}
	if histCount != 1 {
		t.Fatalf("expected histogram count 1, got %d", histCount)
	}
	if errCount != 1 {
		t.Fatalf("expected error count 1, got %d", errCount)
	}
}

func TestRun_tracing_without_context_attributes(t *testing.T) {
	e, sr, _ := newTestExecutorWithInstruments(true, true, false)

	res, err := Run(e, context.Background(), "op.no_attrs", func(ctx context.Context) (string, error) {
		return "no_attrs", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "no_attrs" {
		t.Fatalf("got %q, want %q", res, "no_attrs")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
}

func TestRun_with_attributes_option(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, true, true, tp.Tracer("test"), mp.Meter("test"))
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	_, err = Run(exec, context.Background(), "op.with_attrs",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		},
		WithAttributes(attribute.String("custom.key", "custom.value")),
	)
	if err != nil {
		t.Fatal(err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	attrs := span.Attributes()
	found := false
	for _, a := range attrs {
		if string(a.Key) == "custom.key" && a.Value.AsString() == "custom.value" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom.key attribute on span, got %v", attrs)
	}
}

func TestRun_span_error_status(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, true, false, tp.Tracer("test"), nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	_, _ = Run(exec, context.Background(), "op.err_status",
		func(ctx context.Context) (string, error) {
			return "", errors.New("expected failure")
		},
	)

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Fatalf("expected span status Error, got %v", span.Status().Code)
	}
	if span.Status().Description != "expected failure" {
		t.Fatalf("expected status description 'expected failure', got %q", span.Status().Description)
	}
	if len(span.Events()) != 1 {
		t.Fatalf("expected 1 error event, got %d", len(span.Events()))
	}
}

func TestRun_with_otel_metrics(t *testing.T) {
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	defer mp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, false, true, noop.NewTracerProvider().Tracer("test"), mp.Meter("test"))
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	_, err = Run(exec, context.Background(), "op.metric_test",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Run(exec, context.Background(), "op.metric_test",
		func(ctx context.Context) (string, error) {
			return "", errors.New("fail")
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}

	var histCount uint64
	var errCount int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, dp := range d.DataPoints {
					histCount += dp.Count
				}
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					errCount += dp.Value
				}
			}
		}
	}
	if histCount != 2 {
		t.Fatalf("expected histogram count 2, got %d", histCount)
	}
	if errCount != 1 {
		t.Fatalf("expected error count 1, got %d", errCount)
	}
}

func TestRunWithRecovery_panic(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, true, false, tp.Tracer("test"), nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic")
			}
		}()
		_, _ = RunWithRecovery(exec, context.Background(), "op.panic",
			func(ctx context.Context) (string, error) {
				panic("boom")
			},
		)
	}()

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Fatalf("expected span status Error after panic, got %v", span.Status().Code)
	}
	if span.Status().Description != "panic" {
		t.Fatalf("expected status description 'panic', got %q", span.Status().Description)
	}
	events := span.Events()
	if len(events) == 0 {
		t.Fatalf("expected at least 1 event, got %d", len(events))
	}
	foundException := false
	for _, ev := range events {
		if ev.Name == "exception" {
			foundException = true
			break
		}
	}
	if !foundException {
		t.Fatalf("expected at least one exception event, got %v", events)
	}
}

func TestRunWithRecovery_success(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, true, false, tp.Tracer("test"), nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	res, err := RunWithRecovery(exec, context.Background(), "op.ok",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	if span.Status().Code != codes.Unset {
		t.Fatalf("expected span status Unset for success, got %v", span.Status().Code)
	}
}

func TestSetup_disabled(t *testing.T) {
	obs, err := Setup(context.Background(), Config{Enabled: false, ServiceName: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs == nil {
		t.Fatal("expected non-nil Observability")
	}
	if obs.Executor() == nil {
		t.Fatal("expected non-nil Executor")
	}
	if err := obs.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
}

func TestSetup_validation_error(t *testing.T) {
	_, err := Setup(context.Background(), Config{Enabled: true, ServiceName: "", OTLPEndpoint: "localhost:4317"})
	if err == nil {
		t.Fatal("expected error for missing service name")
	}

	_, err = Setup(context.Background(), Config{Enabled: true, ServiceName: "test", OTLPEndpoint: ""})
	if err == nil {
		t.Fatal("expected error for missing endpoint")
	}

	_, err = Setup(context.Background(), Config{Enabled: true, ServiceName: "test", OTLPEndpoint: "localhost:4317", SampleRatio: -0.1})
	if err == nil {
		t.Fatal("expected error for invalid sample ratio")
	}
}

func TestConfig_WithDefaults(t *testing.T) {
	cfg := Config{TracingEnabled: true}.WithDefaults()
	if cfg.SampleRatio != 1.0 {
		t.Fatalf("expected sample ratio 1.0, got %f", cfg.SampleRatio)
	}

	cfg2 := Config{TracingEnabled: false}.WithDefaults()
	if cfg2.SampleRatio != 0 {
		t.Fatalf("expected sample ratio 0, got %f", cfg2.SampleRatio)
	}
}

func TestConfig_Validate_disabled(t *testing.T) {
	if err := (Config{Enabled: false}).Validate(); err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
}

func TestRun_concurrent_toggles(t *testing.T) {
	e := newTestExecutor(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			e.DisableTracing()
			e.EnableTracing()
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_, _ = Run(e, ctx, "op.concurrent", func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	}

	<-done
	cancel()
}

func TestNewSampler(t *testing.T) {
	if newSampler(0) == nil {
		t.Fatal("expected non-nil sampler for ratio 0")
	}
	if newSampler(1) == nil {
		t.Fatal("expected non-nil sampler for ratio 1")
	}
	if newSampler(0.5) == nil {
		t.Fatal("expected non-nil sampler for ratio 0.5")
	}
}

func TestRun_enricher(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	defer tp.Shutdown(context.Background())

	exec, err := NewExecutorFromInstruments(true, true, false, tp.Tracer("test"), nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	enricher := ContextEnricherFunc(func(ctx context.Context) []attribute.KeyValue {
		return []attribute.KeyValue{attribute.String("tenant.id", "tenant-1")}
	})

	_, err = Run(exec, context.Background(), "op.enricher",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		},
		WithEnrichers(enricher),
	)
	if err != nil {
		t.Fatal(err)
	}

	spans := sr.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	span := spans[0]
	found := false
	for _, a := range span.Attributes() {
		if string(a.Key) == "tenant.id" && a.Value.AsString() == "tenant-1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected tenant.id attribute, got %v", span.Attributes())
	}
}

func TestRun_disabled_fast_path_preserves_context(t *testing.T) {
	e := newTestExecutor(false)

	ctx := WithRequestID(context.Background(), "req-preserve")
	passedCtx := ctx
	res, err := Run(e, ctx, "op.preserve", func(ctx context.Context) (string, error) {
		passedCtx = ctx
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}
	if _, ok := RequestID(passedCtx); !ok {
		t.Fatal("expected context to be preserved")
	}
}

func TestSetup_trace_and_metric_shutdown(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("trace shutdown error: %v", err)
	}

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	if err := mp.Shutdown(context.Background()); err != nil {
		t.Fatalf("metric shutdown error: %v", err)
	}
}

func BenchmarkRun_fast_path(b *testing.B) {
	e := newTestExecutor(false)
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Run(e, ctx, "op", fn)
	}
}

func BenchmarkRun_all_enabled(b *testing.B) {
	e := newTestExecutor(true)
	fn := func(ctx context.Context) (string, error) {
		return "ok", nil
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Run(e, ctx, "op", fn)
	}
}
