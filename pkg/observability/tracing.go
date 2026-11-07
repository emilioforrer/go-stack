package observability

import (
	"context"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var codesError = codes.Error

func tracing[T any](e *Executor, name string, cfg *runConfig) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		return func(ctx context.Context) (T, error) {
			attrs := collectAttributes(ctx, cfg)

			ctx, span := e.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
			defer span.End()

			res, err := next(ctx)

			if err != nil {
				span.RecordError(err)
				span.SetStatus(codesError, err.Error())
			}

			return res, err
		}
	}
}
