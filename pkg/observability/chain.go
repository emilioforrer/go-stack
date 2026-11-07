package observability

func Chain[T any](middlewares ...Middleware[T]) Middleware[T] {
	return func(final Handler[T]) Handler[T] {
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}
