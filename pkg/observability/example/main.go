// Package main demonstrates the go-stack observability package with a small
// instrumented HTTP server.
//
// The server runs observability.Setup with configuration sourced from standard
// OTEL_ and OBSERVABILITY_ environment variables, then instruments every
// endpoint with RunWithRecovery so spans, duration histograms and error
// counters are exported to an OTLP endpoint.
//
// Endpoints:
//
//	GET /work    - simulated successful work (traced and measured).
//	GET /fail    - simulated failure (span error status + error counter).
//	GET /panic   - simulated panic, recovered by middleware: RunWithRecovery
//	               records the panic on the span and re-raises, then the
//	               middleware turns it into a 500.
//	GET /healthz - always succeeds.
//
// A middleware injects RequestID and UserID into each request context, and a
// ContextEnricher copies them onto every span recorded inside the request.
//
// Spans and metrics are exported through gRPC to an OTLP collector. Start one
// with `docker compose up -d` from this directory and open Grafana at
// http://localhost:3000 to see the traces and dashboards.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emilioforrer/go-stack/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
)

// main wires the server, the observability stack, and graceful shutdown.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	obs, err := setupObservability()
	if err != nil {
		slog.Error("setup observability", "error", err)
		os.Exit(1)
	}

	if err := runServer(ctx, obs.Executor()); err != nil {
		slog.Error("server", "error", err)
	}

	// Flush pending spans and metrics before exit. After a graceful shutdown
	// this normally succeeds; a warning here most often means the collector
	// is unreachable.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := obs.Shutdown(shutdownCtx); err != nil {
		slog.Warn("observability shutdown", "error", err)
	}

	slog.Info("application stopped")
}

// setupObservability builds the configuration from environment variables and
// runs observability.Setup. Empty values get sensible defaults so the demo
// works out of the box against the bundled docker-compose stack.
func setupObservability() (*observability.Observability, error) {
	cfg := observability.ConfigFromEnv().WithDefaults()

	if cfg.ServiceName == "" {
		cfg.ServiceName = "observability-example"
	}
	if cfg.OTLPEndpoint == "" {
		cfg.OTLPEndpoint = "localhost:4317"
	}
	// Allow insecure transport by default so the demo connects to the local
	// collector without certificates. An explicit OTEL_EXPORTER_OTLP_INSECURE
	// still wins.
	if os.Getenv("OTEL_EXPORTER_OTLP_INSECURE") == "" {
		cfg.Insecure = true
	}

	slog.Info("observability config",
		"tracing", cfg.TracingEnabled,
		"metrics", cfg.MetricsEnabled,
		"endpoint", cfg.OTLPEndpoint,
		"sample_ratio", cfg.SampleRatio,
	)

	return observability.Setup(context.WithoutCancel(context.Background()), cfg)
}

// runServer starts the HTTP server and blocks until the context is cancelled
// or the listener fails. The server is shut down gracefully before returning.
func runServer(ctx context.Context, exec *observability.Executor) error {
	s := &server{exec: exec}

	addr := "localhost:8080"
	s.http = &http.Server{
		Addr:              addr,
		Handler:           injectRequestContext(s.routes()),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errChan := make(chan error, 1)
	go func() {
		slog.Info("application ready", "addr", "http://"+addr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received")
	case runErr = <-errChan:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	return runErr
}

// server holds the executor shared by all instrumented handlers and the HTTP
// server instance used for graceful shutdown.
type server struct {
	exec *observability.Executor
	http *http.Server
}

// routes registers the demo endpoints.
func (s *server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.Handle("GET /work", s.instrumented("work", false, false))
	mux.Handle("GET /fail", s.instrumented("fail", true, false))
	mux.Handle("GET /panic", s.instrumented("panic", false, true))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok\n")
	})

	return mux
}

// instrumented returns an http.Handler that runs the named simulated operation
// through observability.RunWithRecovery. When fail is set, the operation
// returns an error so the span gets an error status and the error counter
// increments. When panic is set, the operation panics: RunWithRecovery records
// it on the span and re-raises, and the deferred recover turns it into a 500.
func (s *server) instrumented(name string, fail bool, panicSim bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// RunWithRecovery re-raises panics after recording them, so recover
		// here to keep the server process alive.
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("recovered panic", "operation", name, "panic", rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		result, err := observability.RunWithRecovery(s.exec, r.Context(), "example."+name,
			func(ctx context.Context) (string, error) {
				return simulate(name, fail, panicSim)
			},
			observability.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", r.URL.Path),
			),
			observability.WithEnrichers(
				observability.ContextEnricherFunc(func(ctx context.Context) []attribute.KeyValue {
					attrs := make([]attribute.KeyValue, 0, 2)

					if id, ok := observability.RequestID(ctx); ok {
						attrs = append(attrs, attribute.String("request.id", id))
					}
					if id, ok := observability.UserID(ctx); ok {
						attrs = append(attrs, attribute.String("user.id", id))
					}

					return attrs
				}),
			),
		)

		switch {
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			_, _ = fmt.Fprintln(w, result)
		}
	})
}

// simulate stands in for a real application operation. It always sleeps a few
// milliseconds so duration histograms have data, then either panics, fails, or
// succeeds depending on the flags.
func simulate(name string, fail bool, panicSim bool) (string, error) {
	if panicSim {
		panic(fmt.Sprintf("simulated panic in %s", name))
	}

	time.Sleep(time.Duration(rand.IntN(50)+10) * time.Millisecond)

	if fail {
		return "", errors.New("simulated failure")
	}

	return name + " ok", nil
}

// injectRequestContext injects a request ID and a user ID into every request
// context. In a real application these would come from headers (X-Request-ID)
// or an authentication layer; the enricher in instrumented copies them onto
// the spans.
func injectRequestContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := observability.WithRequestID(r.Context(), fmt.Sprintf("req-%03d", rand.IntN(1000)))
		ctx = observability.WithUserID(ctx, "user-7")

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
