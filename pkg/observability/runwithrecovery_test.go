package observability

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestRunWithRecovery_disabled_fast_path(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec, err := NewExecutorFromInstruments(false, true, true, tracer, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	called := false
	res, err := RunWithRecovery(exec, context.Background(), "op.disabled",
		func(ctx context.Context) (string, error) {
			called = true
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("RunWithRecovery() error = %v", err)
	}
	if !called {
		t.Fatal("expected handler to run")
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}
}

func TestRunWithRecovery_both_toggles_off(t *testing.T) {
	tracer := noop.NewTracerProvider().Tracer("test")
	exec, err := NewExecutorFromInstruments(true, false, false, tracer, nil)
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	called := false
	res, err := RunWithRecovery(exec, context.Background(), "op.no_instruments",
		func(ctx context.Context) (string, error) {
			called = true
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("RunWithRecovery() error = %v", err)
	}
	if !called {
		t.Fatal("expected handler to run")
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}
}

func TestRunWithRecovery_metrics_records(t *testing.T) {
	tp := sdktrace.NewTracerProvider()

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	exec, err := NewExecutorFromInstruments(true, false, true, tp.Tracer("test"), mp.Meter("test"))
	if err != nil {
		t.Fatalf("create executor: %v", err)
	}

	if _, err := RunWithRecovery(exec, context.Background(), "op.metrics_success",
		func(ctx context.Context) (string, error) { return "ok", nil }); err != nil {
		t.Fatalf("RunWithRecovery() error = %v", err)
	}
	if _, err := RunWithRecovery(exec, context.Background(), "op.metrics_error",
		func(ctx context.Context) (string, error) { return "", errors.New("boom") }); err == nil {
		t.Fatal("expected error from handler")
	}

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	var durationCount uint64
	hasDuration := false
	var errorCount int64
	hasErrors := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			switch d := m.Data.(type) {
			case metricdata.Histogram[float64]:
				for _, dp := range d.DataPoints {
					if v, ok := dp.Attributes.Value("operation.name"); !ok || v.AsString() != "op.metrics_success" {
						continue
					}
					durationCount += dp.Count
					hasDuration = true
				}
			case metricdata.Sum[int64]:
				for _, dp := range d.DataPoints {
					if v, ok := dp.Attributes.Value("operation.name"); !ok || v.AsString() != "op.metrics_error" {
						continue
					}
					errorCount += dp.Value
					hasErrors = true
				}
			}
		}
	}

	if !hasDuration {
		t.Fatal("expected duration histogram data point with operation.name=op.metrics_success")
	}
	if durationCount != 1 {
		t.Fatalf("expected duration count 1, got %d", durationCount)
	}
	if !hasErrors {
		t.Fatal("expected error counter data point with operation.name=op.metrics_error")
	}
	if errorCount != 1 {
		t.Fatalf("expected error count 1, got %d", errorCount)
	}
}
