package observability

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

// RunOption allows callers to customize the behavior of Run per invocation.
type RunOption func(*runConfig)

type runConfig struct {
	attributes []attribute.KeyValue
	enrichers  []ContextEnricher
}

// WithAttributes attaches operation-specific attributes to the span and metrics.
//
// Avoid adding high-cardinality or sensitive values indiscriminately.
// Be careful with: email addresses, access tokens, full URLs with query parameters,
// session IDs, PII, arbitrary request bodies, and unbounded user-generated values.
func WithAttributes(attrs ...attribute.KeyValue) RunOption {
	return func(cfg *runConfig) {
		cfg.attributes = append(cfg.attributes, attrs...)
	}
}

// WithEnrichers attaches custom context enrichers to include dynamic attributes.
func WithEnrichers(enrichers ...ContextEnricher) RunOption {
	return func(cfg *runConfig) {
		cfg.enrichers = append(cfg.enrichers, enrichers...)
	}
}

// ContextEnricher extracts attributes from a context.
type ContextEnricher interface {
	Attributes(context.Context) []attribute.KeyValue
}

// ContextEnricherFunc is a function adapter for ContextEnricher.
type ContextEnricherFunc func(context.Context) []attribute.KeyValue

// Attributes implements ContextEnricher.
func (f ContextEnricherFunc) Attributes(ctx context.Context) []attribute.KeyValue {
	return f(ctx)
}

func collectAttributes(ctx context.Context, cfg *runConfig) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, len(cfg.attributes))
	attrs = append(attrs, cfg.attributes...)

	for _, enricher := range cfg.enrichers {
		attrs = append(attrs, enricher.Attributes(ctx)...)
	}

	if reqID, ok := RequestID(ctx); ok {
		attrs = append(attrs, attribute.String("request.id", reqID))
	}
	if userID, ok := UserID(ctx); ok {
		attrs = append(attrs, attribute.String("enduser.id", userID))
	}

	return attrs
}
