package httpsvr

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// Constants for HTTP headers and error messages
const (
	contentTypeHeader    = "Content-Type"
	applicationJSON      = "application/json"
	encodeErrorFormatStr = "failed to encode error response: %v"
)

type traceIDCtxKey struct{}

var traceIDKey traceIDCtxKey

// Helper functions

// func generateTraceID() string {
// 	bytes := make([]byte, 16) // TraceID is 16 bytes (128-bit) per spec
// 	rand.Read(bytes)
// 	return hex.EncodeToString(bytes)
// }

func generateTraceID() string {
	traceID := trace.NewSpanContext(trace.SpanContextConfig{}).TraceID()
	return traceID.String()
}

func getTraceID(ctx context.Context) string {
	val, ok := ctx.Value(traceIDKey).(string)
	if !ok {
		return generateTraceID()
	}

	return val
}

func isServerError(code int) bool {
	return code >= 500 && code <= 599
}

func isClientError(code int) bool {
	return code >= 400 && code <= 499 && code != http.StatusNotFound
}

// NotFound components
type notFoundResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *notFoundResponseWriter) Write(bytes []byte) (int, error) {
	if w.statusCode == http.StatusNotFound {
		w.written = true
		return len(bytes), nil // Suppress the default 404 message
	}

	return w.ResponseWriter.Write(bytes)
}

func (w *notFoundResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	if statusCode != http.StatusNotFound {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func NotFoundMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nfw := &notFoundResponseWriter{ResponseWriter: w}
			next.ServeHTTP(nfw, r)

			if nfw.statusCode == http.StatusNotFound {
				w.Header().Set(contentTypeHeader, applicationJSON)
				w.WriteHeader(http.StatusNotFound)

				err := json.NewEncoder(w).Encode(ErrorResponse{
					Errors: []ErrorObject{
						{
							ID:     getTraceID(r.Context()),
							Status: "404",
							Title:  "Route not found",
							Detail: "The requested route was not found",
							Source: &ErrorSource{
								Pointer: r.URL.Path,
							},
						},
					},
				})
				if err != nil {
					slog.Error("failed to encode error response", "error", err)
				}
			}
		})
	}
}

// Internal Server Error components
type internalServerResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (w *internalServerResponseWriter) Write(bytes []byte) (int, error) {
	if isServerError(w.statusCode) {
		w.written = true
		return len(bytes), nil
	}

	return w.ResponseWriter.Write(bytes)
}

func (w *internalServerResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	if !isServerError(statusCode) {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func InternalServerErrorMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			isw := &internalServerResponseWriter{ResponseWriter: w}
			next.ServeHTTP(isw, r)

			if isServerError(isw.statusCode) {
				w.Header().Set(contentTypeHeader, applicationJSON)
				w.WriteHeader(isw.statusCode)

				err := json.NewEncoder(w).Encode(ErrorResponse{
					Errors: []ErrorObject{
						{
							ID:     getTraceID(r.Context()),
							Status: strconv.Itoa(isw.statusCode),
							Title:  "Internal Server Error",
							Detail: http.StatusText(isw.statusCode),
							Source: &ErrorSource{
								Pointer: r.URL.Path,
							},
						},
					},
				})
				if err != nil {
					slog.Error("failed to encode error response", "error", err)
				}
			}
		})
	}
}

// Client Error components
type clientErrorResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	written    bool
}

func (w *clientErrorResponseWriter) Write(bytes []byte) (int, error) {
	if isClientError(w.statusCode) {
		w.written = true
		return w.body.Write(bytes)
	}

	return w.ResponseWriter.Write(bytes)
}

func (w *clientErrorResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	if !isClientError(statusCode) {
		w.ResponseWriter.WriteHeader(statusCode)
	}
}

func ClientErrorMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			crw := &clientErrorResponseWriter{
				ResponseWriter: w,
				body:           new(bytes.Buffer),
			}
			next.ServeHTTP(crw, r)

			if isClientError(crw.statusCode) {
				w.Header().Set(contentTypeHeader, applicationJSON)
				w.WriteHeader(crw.statusCode)

				err := json.NewEncoder(w).Encode(ErrorResponse{
					Errors: []ErrorObject{
						{
							ID:     getTraceID(r.Context()),
							Status: strconv.Itoa(crw.statusCode),
							Title:  http.StatusText(crw.statusCode),
							Detail: strings.TrimSpace(crw.body.String()),
							Source: &ErrorSource{
								Pointer: r.URL.Path,
							},
						},
					},
				})
				if err != nil {
					slog.Error("failed to encode error response", "error", err)
				}
			}
		})
	}
}

func TracingMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := generateTraceID()
			ctx := context.WithValue(r.Context(), traceIDKey, traceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RecoveryMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					slog.Error("Panic recovered", "error", err, "stack", string(debug.Stack()))
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
