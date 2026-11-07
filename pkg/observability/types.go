package observability

import "context"

type Handler[T any] func(ctx context.Context) (T, error)

type Middleware[T any] func(next Handler[T]) Handler[T]
