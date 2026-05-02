package httpsvr

import (
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// SkipMiddlewaresForPaths creates a middleware that will skip all provided middlewares
// if the request URL path matches any of the patterns in skipURLPatterns.
func SkipMiddlewaresForPaths(skipURLPatterns []string, middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		// First, apply all middlewares in reverse order to build the middleware chain
		handler := next
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}

		// Then return a handler that either skips all middlewares or applies them
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if the current path matches any of the skip patterns
			for _, pattern := range skipURLPatterns {
				matched, err := regexp.MatchString(pattern, r.URL.Path)
				if err == nil && matched {
					// If matched, skip all middlewares and go straight to the final handler
					next.ServeHTTP(w, r)
					return
				}
			}

			// If no match, use the middleware chain
			handler.ServeHTTP(w, r)
		})
	}
}

// RequestLoggerMiddleware logs the start and completion of each request.
func RequestLoggerMiddleware(enabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !enabled {
				next.ServeHTTP(w, r)
				return
			}

			startLocal := time.Now()
			startUTC := startLocal.UTC()

			rw := &requestLoggerResponseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(startUTC)

			slog.Info("Started request",
				"method", strconv.Quote(r.Method),
				"path", strconv.Quote(r.URL.Path),
				"remote_addr", strconv.Quote(r.RemoteAddr),
				"started_at", startUTC.Format("2006-01-02 15:04:05 UTC"),
				"started_at_local", startLocal.Format("2006-01-02 15:04:05 MST"))

			slog.Info("Completed request",
				"status", rw.status,
				"status_text", http.StatusText(rw.status),
				"duration", duration)
		})
	}
}

// requestLoggerResponseWriter wraps http.ResponseWriter to capture status code
type requestLoggerResponseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *requestLoggerResponseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}
