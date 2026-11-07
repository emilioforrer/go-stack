package observability

import (
	"context"
	"testing"
	"time"
)

// enabledTestConfig returns a config pointing at an unreachable local endpoint.
// OTLP gRPC exporters are created lazily, so building them never dials;
// an endpoint outside the process keeps the tests side-effect free.
func enabledTestConfig() Config {
	return Config{
		Enabled:        true,
		TracingEnabled: true,
		MetricsEnabled: true,
		ServiceName:    "observability-test",
		OTLPEndpoint:   "127.0.0.1:1",
		Insecure:       true,
		SampleRatio:    0.5,
	}
}

func TestObservability_Shutdown_nil(t *testing.T) {
	var nilObs *Observability
	if err := nilObs.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error on nil receiver: %v", err)
	}

	obs := &Observability{}
	if err := obs.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error on nil shutdown func: %v", err)
	}
}

func TestSetup_tracing_only(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.MetricsEnabled = false

	obs, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if obs.Executor() == nil {
		t.Fatal("expected non-nil Executor")
	}

	res, err := Run(obs.Executor(), context.Background(), "op.trace_only",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res != "ok" {
		t.Fatalf("got %q, want %q", res, "ok")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		t.Logf("shutdown against unreachable endpoint failed (expected): %v", err)
	}
}

func TestSetup_metrics_only(t *testing.T) {
	cfg := enabledTestConfig()
	cfg.TracingEnabled = false

	obs, err := Setup(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	// Run an operation that exercises the real metric instruments.
	if _, err := Run(obs.Executor(), context.Background(), "op.metrics_only",
		func(ctx context.Context) (string, error) {
			return "ok", nil
		}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		t.Logf("shutdown against unreachable endpoint failed (expected): %v", err)
	}
}

func TestSetup_tracing_and_metrics(t *testing.T) {
	obs, err := Setup(context.Background(), enabledTestConfig())
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	// RunWithRecovery exercises both span recording and metric instruments
	// (histogram + error counter).
	if _, err := RunWithRecovery(obs.Executor(), context.Background(), "op.full",
		func(ctx context.Context) (string, error) {
			return "", context.Canceled
		}); err != nil && err != context.Canceled {
		t.Fatalf("RunWithRecovery() error = %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := obs.Shutdown(shutdownCtx); err != nil {
		t.Logf("shutdown against unreachable endpoint failed (expected): %v", err)
	}
}

func TestBuildResource(t *testing.T) {
	cfg := Config{
		ServiceName:    "my-service",
		ServiceVersion: "1.2.3",
		Environment:    "prod",
	}

	res, err := buildResource(context.Background(), cfg)
	if err != nil {
		t.Fatalf("buildResource() error = %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil resource")
	}

	if name, ok := res.Set().Value("service.name"); !ok || name.AsString() != "my-service" {
		t.Fatalf("got service.name = %v (ok=%v), want my-service", name, ok)
	}
	if version, ok := res.Set().Value("service.version"); !ok || version.AsString() != "1.2.3" {
		t.Fatalf("got service.version = %v (ok=%v), want 1.2.3", version, ok)
	}
	if environment, ok := res.Set().Value("deployment.environment"); !ok || environment.AsString() != "prod" {
		t.Fatalf("got deployment.environment = %v (ok=%v), want prod", environment, ok)
	}
}

func TestInitTracing(t *testing.T) {
	cfg := enabledTestConfig()

	shutdown, tracer, err := initTracing(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("initTracing() error = %v", err)
	}
	if tracer == nil {
		t.Fatal("expected non-nil tracer")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Logf("shutdown against unreachable endpoint failed (expected): %v", err)
	}
}

func TestInitMetrics(t *testing.T) {
	cfg := enabledTestConfig()

	mp, shutdown, err := initMetrics(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("initMetrics() error = %v", err)
	}
	if mp == nil {
		t.Fatal("expected non-nil meter provider")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := shutdown(shutdownCtx); err != nil {
		t.Logf("shutdown against unreachable endpoint failed (expected): %v", err)
	}
}
