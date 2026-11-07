package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func metrics[T any](e *Executor, name string, cfg *runConfig) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		return func(ctx context.Context) (T, error) {
			start := time.Now()

			res, err := next(ctx)

			duration := time.Since(start).Seconds()

			attrs := append(
				[]attribute.KeyValue{attribute.String("operation.name", name)},
				collectAttributes(ctx, cfg)...,
			)

			if e.duration != nil {
				e.duration.Record(ctx, duration, metric.WithAttributes(attrs...))
			}

			if err != nil {
				e.errors.Add(ctx, 1, metric.WithAttributes(attrs...))
			}

			return res, err
		}
	}
}
