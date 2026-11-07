# observability example

A runnable demo of the [`pkg/observability`](../README.md) package: a small HTTP
server whose endpoints are instrumented with `RunWithRecovery`, producing
**traces** and **metrics** on a local Grafana stack.

## What it demonstrates

| Feature | Where |
|---|---|
| `Setup` with env-based config (`ConfigFromEnv`) | `setupObservability` |
| Spans + duration histogram + error counter | `server.instrumented` |
| Panic recording with `RunWithRecovery` | `GET /panic` |
| Error status and error counter | `GET /fail` |
| Request ID / user ID in context | `injectRequestContext` |
| Context enrichers on spans | `ContextEnricherFunc` |
| Graceful shutdown with telemetry flush | `main` |

## Run

```bash
# 1. Start the Grafana LGTM stack (OTLP collector + Tempo + Prometheus + Grafana).
docker compose up -d

# 2. Run the example (from this directory).
go run .

# 3. Generate traffic.
curl localhost:8080/healthz
curl localhost:8080/work
curl localhost:8080/fail
curl localhost:8080/panic

# 4. Explore the data in Grafana: http://localhost:3000 (admin / admin).
#    - Traces: Explore -> Tempo, service "observability-example"
#    - Metrics: Explore -> Prometheus (application_operation_duration_seconds,
#      application_operation_errors_total)

# 5. Tear down.
docker compose down
```

## Configuration

Configuration is read from standard `OTEL_` and `OBSERVABILITY_` environment
variables (with demo-friendly defaults):

| Variable | Default | Description |
|---|---|---|
| `OBSERVABILITY_ENABLED` | `true` | Master switch (disabling keeps a no-op executor) |
| `OBSERVABILITY_TRACING_ENABLED` | `true` | Export spans to the OTLP endpoint |
| `OBSERVABILITY_METRICS_ENABLED` | `true` | Export metrics to the OTLP endpoint |
| `OTEL_SERVICE_NAME` | `observability-example` | Service name on all telemetry |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Insecure gRPC for the local collector |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | Trace sampling ratio |

The example runs fine with no environment variables set. It also tolerates the
collector being down: exporters connect lazily, and a failed flush at shutdown
is logged as a warning.

## Notes

- The example module is intentionally separate from
  `github.com/emilioforrer/go-stack/pkg/observability`; the workspace
  (`go.work` at the repository root) resolves the dependency locally.
- `RunWithRecovery` **re-raises** panics after recording them; the demo recovers
  them at the middleware boundary so the server process stays alive.
