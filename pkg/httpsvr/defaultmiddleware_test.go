package httpsvr

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLoggerMiddleware_Enabled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestLoggerMiddleware(true)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequestLoggerMiddleware_Disabled(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RequestLoggerMiddleware(false)
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRequestLoggerResponseWriter_WriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &requestLoggerResponseWriter{ResponseWriter: rec, status: http.StatusOK}
	rw.WriteHeader(http.StatusCreated)
	if rw.status != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rw.status)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected underlying writer 201, got %d", rec.Code)
	}
}

func TestSkipMiddlewaresForPaths(t *testing.T) {
	// Create a test middleware that adds a header
	testMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Middleware", "applied")
			next.ServeHTTP(w, r)
		})
	}

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	tests := []struct {
		name             string
		skipPatterns     []string
		requestPath      string
		expectMiddleware bool
		verify           func(t *testing.T, w *httptest.ResponseRecorder, expectMiddleware bool)
	}{
		{
			name:             "middleware applied for non-matching path",
			skipPatterns:     []string{"/skip"},
			requestPath:      "/test",
			expectMiddleware: true,
			verify: func(t *testing.T, w *httptest.ResponseRecorder, expectMiddleware bool) {
				header := w.Header().Get("X-Test-Middleware")
				if expectMiddleware && header != "applied" {
					t.Error("Expected middleware to be applied")
				}
				if !expectMiddleware && header == "applied" {
					t.Error("Expected middleware to be skipped")
				}
			},
		},
		{
			name:             "middleware skipped for matching path",
			skipPatterns:     []string{"/skip"},
			requestPath:      "/skip",
			expectMiddleware: false,
			verify: func(t *testing.T, w *httptest.ResponseRecorder, expectMiddleware bool) {
				header := w.Header().Get("X-Test-Middleware")
				if expectMiddleware && header != "applied" {
					t.Error("Expected middleware to be applied")
				}
				if !expectMiddleware && header == "applied" {
					t.Error("Expected middleware to be skipped")
				}
			},
		},
		{
			name:             "middleware skipped for regex pattern",
			skipPatterns:     []string{"/api/.*"},
			requestPath:      "/api/test",
			expectMiddleware: false,
			verify: func(t *testing.T, w *httptest.ResponseRecorder, expectMiddleware bool) {
				header := w.Header().Get("X-Test-Middleware")
				if expectMiddleware && header != "applied" {
					t.Error("Expected middleware to be applied")
				}
				if !expectMiddleware && header == "applied" {
					t.Error("Expected middleware to be skipped")
				}
			},
		},
		{
			name:             "middleware applied for non-matching regex",
			skipPatterns:     []string{"/api/.*"},
			requestPath:      "/web/test",
			expectMiddleware: true,
			verify: func(t *testing.T, w *httptest.ResponseRecorder, expectMiddleware bool) {
				header := w.Header().Get("X-Test-Middleware")
				if expectMiddleware && header != "applied" {
					t.Error("Expected middleware to be applied")
				}
				if !expectMiddleware && header == "applied" {
					t.Error("Expected middleware to be skipped")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap with skip middleware
			skipMiddleware := SkipMiddlewaresForPaths(tt.skipPatterns, testMiddleware)
			wrappedHandler := skipMiddleware(handler)

			// Create request and response recorder
			req := httptest.NewRequest("GET", tt.requestPath, nil)
			w := httptest.NewRecorder()

			// Execute request
			wrappedHandler.ServeHTTP(w, req)

			// Check status
			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Custom verification
			if tt.verify != nil {
				tt.verify(t, w, tt.expectMiddleware)
			}
		})
	}
}
