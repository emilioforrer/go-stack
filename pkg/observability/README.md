# pkg/observability

A high-level, ergonomic wrapper around OpenTelemetry for Go services. It provides a unified API for **tracing** and **metrics** with minimal boilerplate, runtime toggling, and graceful shutdown support.

## Features

- **Unified setup** – One `Config` and one `Setup()` call initialize both tracing and metrics.
- **Ergonomic instrumentation** – `Run` and `RunWithRecovery` wrap any `func(context.Context) (T, error)` with spans and histograms.
- **Runtime toggling** – Enable or disable tracing and metrics independently via `Executor` without restarting.
- **Context enrichment** – Attach request IDs, user IDs, and custom attributes dynamically via `ContextEnricher`.
- **Environment-first configuration** – `ConfigFromEnv()` reads standard OTEL variables plus app-specific overrides.
- **Zero-overhead fast path** – When disabled, `Run` bypasses all instrumentation and calls the handler directly.
- **Panic recovery** – `RunWithRecovery` records panics on the active span before re-raising.
- **OTLP/gRPC exporters** – Ships traces and metrics to any OTLP-compatible backend (Jaeger, Grafana Tempo, Prometheus via OpenTelemetry Collector, etc.).

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/emilioforrer/go-stack/pkg/observability"
    "go.opentelemetry.io/otel/attribute"
)

func main() {
    ctx := context.Background()

    // 1. Configure from environment and overrides
    cfg := observability.ConfigFromEnv()
    cfg.ServiceName    = "my-service"
    cfg.ServiceVersion = "1.0.0"
    cfg.Environment    = "production"
    cfg.OTLPEndpoint   = "otel-collector:4317"

    // 2. Initialize observability
    obs, err := observability.Setup(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer func() {
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        _ = obs.Shutdown(shutdownCtx)
    }()

    exec := obs.Executor()

    // 3. Instrument a business operation
    result, err := observability.RunWithRecovery(exec, ctx, "order.create",
        func(ctx context.Context) (string, error) {
            // Your business logic here
            return "order-123", nil
        },
        observability.WithAttributes(
            attribute.String("customer.id", "cust-456"),
        ),
    )
    fmt.Println(result, err)
}
```

## Core Concepts

### Config

`Config` controls the entire observability stack. Required fields are validated by `Setup()`.

| Field | Description |
|-------|-------------|
| `Enabled` | Master switch. When `false`, everything is no-op. |
| `TracingEnabled` | Enable distributed tracing. |
| `MetricsEnabled` | Enable histograms and counters. |
| `ServiceName` | Required when enabled. Used as the tracer scope and resource attribute. |
| `ServiceVersion` | Resource attribute for `service.version`. |
| `Environment` | Resource attribute for `deployment.environment`. |
| `OTLPEndpoint` | Required when enabled. Host:port of the OTLP/gRPC collector. |
| `Insecure` | Use gRPC without TLS (useful for local development). |
| `SampleRatio` | Trace sampling ratio (`0` = never, `1` = always). Defaults to `1.0` when tracing is enabled. |

### Setup

```go
obs, err := observability.Setup(ctx, cfg)
```

`Setup` builds the OpenTelemetry `TracerProvider` and `MeterProvider`, configures global text-map propagation (`TraceContext` + `Baggage`), and returns an `*Observability` handle. Call `obs.Shutdown(ctx)` before exit to flush pending spans and metrics.

When `cfg.Enabled == false`, `Setup` returns a no-op `Observability` whose `Executor` bypasses all instrumentation.

### Executor

The `Executor` is the runtime engine used by `Run` and `RunWithRecovery`.

```go
exec := obs.Executor()
```

Toggle behavior at any time:

```go
exec.Disable()         // fully disable
exec.Enable()          // re-enable
exec.DisableTracing()  // keep metrics, drop spans
exec.EnableMetrics()   // turn metrics back on
```

### Run and RunWithRecovery

Both functions execute a `Handler[T]` (a `func(context.Context) (T, error)`) and optionally wrap it with tracing and metrics middleware.

- **`Run`** – Lightweight; uses composed middleware.
- **`RunWithRecovery`** – Also catches panics, records them on the span, and re-raises.

```go
result, err := observability.RunWithRecovery(exec, ctx, "operation.name", handler,
    observability.WithAttributes(attribute.String("key", "value")),
    observability.WithEnrichers(myEnricher),
)
```

### Context Enrichment

The package reads two well-known context values and attaches them automatically to every span and metric:

| Context Value | OTel Attribute | How to set |
|---------------|----------------|------------|
| Request ID | `request.id` | `observability.WithRequestID(ctx, "req-abc")` |
| User ID | `enduser.id` | `observability.WithUserID(ctx, "user-123")` |

For dynamic attributes, implement `ContextEnricher` or use the `ContextEnricherFunc` adapter:

```go
tenant := observability.ContextEnricherFunc(func(ctx context.Context) []attribute.KeyValue {
    return []attribute.KeyValue{attribute.String("tenant.id", "acme")}
})

observability.Run(exec, ctx, "op", handler, observability.WithEnrichers(tenant))
```

## Configuration from Environment

`ConfigFromEnv()` reads the following variables:

| Variable | Default | Maps to |
|----------|---------|---------|
| `OBSERVABILITY_ENABLED` | `true` | `Enabled` |
| `OBSERVABILITY_TRACING_ENABLED` | `true` | `TracingEnabled` |
| `OBSERVABILITY_METRICS_ENABLED` | `true` | `MetricsEnabled` |
| `OTEL_SERVICE_NAME` | – | `ServiceName` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | – | `OTLPEndpoint` |
| `OTEL_EXPORTER_OTLP_INSECURE` | `false` | `Insecure` |
| `OTEL_TRACES_SAMPLER_ARG` | `0` | `SampleRatio` |
| `OTEL_TRACES_SAMPLER` | – | Adjusts `SampleRatio` for `always_off` |

## Metrics

When `MetricsEnabled` is true, the package instruments every `Run` call with:

- **`application.operation.duration`** – `Float64Histogram` in seconds (`s`).
- **`application.operation.errors`** – `Int64Counter` incremented when the handler returns a non-nil error.

Both metrics are tagged with:
- `operation.name`
- Static attributes from `WithAttributes`
- Attributes produced by `ContextEnricher`s
- `request.id` and `enduser.id` when present in context

## Traces

When `TracingEnabled` is true, every `Run` call creates a span named with the operation name. Spans automatically receive:

- An **Error** status when the handler returns an error.
- A **panic** status when `RunWithRecovery` catches a panic.
- All collected attributes (see Context Enrichment above).

Nested operations create child spans naturally because the parent span is stored in the `context.Context` passed to the handler.

```go
// Parent span: "order.create"
observability.RunWithRecovery(exec, ctx, "order.create", func(ctx context.Context) (string, error) {
    // Child span: "payment.charge"
    return observability.RunWithRecovery(exec, ctx, "payment.charge", paymentHandler)
})
```

## Advanced Usage

### Manual Middleware Chain

For custom pipelines, the `Chain` helper composes `Middleware[T]`:

```go
m := observability.Chain(mw1, mw2, mw3)
handler := m(finalHandler)
result, err := handler(ctx)
```

For custom pipelines, the `Chain` helper composes `Middleware[T]`:

```go
m := observability.Chain(mw1, mw2, mw3)
handler := m(finalHandler)
result, err := handler(ctx)
```

## Testing

The package includes unit tests for middleware composition, context utilities, and executor edge cases:

```bash
go test ./pkg/observability/...
```

| Test File | Coverage |
|-----------|----------|
| `context_test.go` | `WithRequestID`, `WithUserID`, `RequestID`, `UserID` |
| `chain_test.go` | `Chain` ordering and short-circuit behavior |
| `executor_test.go` | `Run`, `RunWithRecovery`, toggle states, error propagation |

## Dependencies

- `go.opentelemetry.io/otel`
- `go.opentelemetry.io/otel/sdk`
- `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
- `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc`

## License

This package is part of the `go-stack` project. See the repository root for license details.
