package httpsvr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGenerateTraceID(t *testing.T) {
	id := generateTraceID()
	if id == "" {
		t.Fatal("expected non-empty trace ID")
	}
}

func TestGetTraceID_FromContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), traceIDKey, "test-trace-id")
	id := getTraceID(ctx)
	if id != "test-trace-id" {
		t.Fatalf("expected 'test-trace-id', got '%s'", id)
	}
}

func TestGetTraceID_GeneratesNew(t *testing.T) {
	id := getTraceID(context.Background())
	if id == "" {
		t.Fatal("expected non-empty trace ID")
	}
}

func TestIsServerError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{499, false},
		{500, true},
		{503, true},
		{599, true},
		{600, false},
	}
	for _, tt := range tests {
		if got := isServerError(tt.code); got != tt.expected {
			t.Errorf("isServerError(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestIsClientError(t *testing.T) {
	tests := []struct {
		code     int
		expected bool
	}{
		{399, false},
		{400, true},
		{403, true},
		{404, false},
		{499, true},
		{500, false},
	}
	for _, tt := range tests {
		if got := isClientError(tt.code); got != tt.expected {
			t.Errorf("isClientError(%d) = %v, want %v", tt.code, got, tt.expected)
		}
	}
}

func TestNotFoundResponseWriter_Write_404(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &notFoundResponseWriter{ResponseWriter: rec, statusCode: http.StatusNotFound}
	n, err := w.Write([]byte("not found"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len([]byte("not found")) {
		t.Fatalf("expected %d bytes written, got %d", len([]byte("not found")), n)
	}
	if !w.written {
		t.Fatal("expected written to be true")
	}
}

func TestNotFoundResponseWriter_Write_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &notFoundResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK}
	n, err := w.Write([]byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 bytes written, got %d", n)
	}
}

func TestNotFoundResponseWriter_WriteHeader_404(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &notFoundResponseWriter{ResponseWriter: rec}
	w.WriteHeader(http.StatusNotFound)
	if w.statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected underlying writer to keep default 200, got %d", rec.Code)
	}
}

func TestNotFoundResponseWriter_WriteHeader_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &notFoundResponseWriter{ResponseWriter: rec}
	w.WriteHeader(http.StatusOK)
	if w.statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected underlying writer to have status 200, got %d", rec.Code)
	}
}

func TestNotFoundMiddleware_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mw := NotFoundMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/missing", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Title != "Route not found" {
		t.Fatalf("expected 'Route not found', got '%s'", resp.Errors[0].Title)
	}
	if resp.Errors[0].Source.Pointer != "/missing" {
		t.Fatalf("expected '/missing', got '%s'", resp.Errors[0].Source.Pointer)
	}
}

func TestNotFoundMiddleware_Found(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := NotFoundMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/found", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("expected 'ok', got '%s'", w.Body.String())
	}
}

func TestInternalServerResponseWriter_Write_5xx(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &internalServerResponseWriter{ResponseWriter: rec, statusCode: http.StatusInternalServerError}
	n, err := w.Write([]byte("error"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len([]byte("error")) {
		t.Fatalf("expected %d, got %d", len([]byte("error")), n)
	}
	if !w.written {
		t.Fatal("expected written to be true")
	}
}

func TestInternalServerResponseWriter_Write_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &internalServerResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK}
	n, err := w.Write([]byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestInternalServerResponseWriter_WriteHeader_5xx(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &internalServerResponseWriter{ResponseWriter: rec}
	w.WriteHeader(http.StatusInternalServerError)
	if w.statusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected underlying writer to keep default 200, got %d", rec.Code)
	}
}

func TestInternalServerResponseWriter_WriteHeader_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &internalServerResponseWriter{ResponseWriter: rec}
	w.WriteHeader(http.StatusOK)
	if w.statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestInternalServerErrorMiddleware_5xx(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	mw := InternalServerErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/fail", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Title != "Internal Server Error" {
		t.Fatalf("expected 'Internal Server Error', got '%s'", resp.Errors[0].Title)
	}
}

func TestInternalServerErrorMiddleware_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := InternalServerErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestClientErrorResponseWriter_Write_4xx(t *testing.T) {
	rec := httptest.NewRecorder()
	buf := new(bytes.Buffer)
	w := &clientErrorResponseWriter{ResponseWriter: rec, statusCode: http.StatusBadRequest, body: buf}
	n, err := w.Write([]byte("bad"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
	if !w.written {
		t.Fatal("expected written to be true")
	}
}

func TestClientErrorResponseWriter_Write_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	buf := new(bytes.Buffer)
	w := &clientErrorResponseWriter{ResponseWriter: rec, statusCode: http.StatusOK, body: buf}
	n, err := w.Write([]byte("ok"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestClientErrorResponseWriter_WriteHeader_4xx(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &clientErrorResponseWriter{ResponseWriter: rec, body: new(bytes.Buffer)}
	w.WriteHeader(http.StatusBadRequest)
	if w.statusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected underlying writer to keep default 200, got %d", rec.Code)
	}
}

func TestClientErrorResponseWriter_WriteHeader_Other(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &clientErrorResponseWriter{ResponseWriter: rec, body: new(bytes.Buffer)}
	w.WriteHeader(http.StatusOK)
	if w.statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.statusCode)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestClientErrorMiddleware_4xx(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	mw := ClientErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/bad", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Title != http.StatusText(http.StatusBadRequest) {
		t.Fatalf("expected 'Bad Request', got '%s'", resp.Errors[0].Title)
	}
}

func TestClientErrorMiddleware_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := ClientErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ok", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTracingMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := getTraceID(r.Context())
		if id == "" {
			t.Fatal("expected non-empty trace ID in context")
		}
		w.WriteHeader(http.StatusOK)
	})

	mw := TracingMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_NoPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := RecoveryMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_WithPanic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	mw := RecoveryMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestClientErrorMiddleware_404NotClientError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mw := ClientErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/notfound", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestClientErrorMiddleware_WithBody(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("access denied"))
	})

	mw := ClientErrorMiddleware()
	wrapped := mw(handler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/forbidden", nil)
	wrapped.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(resp.Errors))
	}
	if resp.Errors[0].Detail != "access denied" {
		t.Fatalf("expected 'access denied', got '%s'", resp.Errors[0].Detail)
	}
}

type failingResponseWriter struct {
	http.ResponseWriter
	failOnWrite bool
}

func (f *failingResponseWriter) Write(b []byte) (int, error) {
	if f.failOnWrite {
		return 0, errors.New("write failed")
	}
	return f.ResponseWriter.Write(b)
}

func (f *failingResponseWriter) WriteHeader(code int) {
	f.ResponseWriter.WriteHeader(code)
}

func TestNotFoundMiddleware_EncodeError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	mw := NotFoundMiddleware()
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	w := &failingResponseWriter{ResponseWriter: rec, failOnWrite: true}
	req := httptest.NewRequest("GET", "/missing", nil)
	wrapped.ServeHTTP(w, req)
}

func TestInternalServerErrorMiddleware_EncodeError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	mw := InternalServerErrorMiddleware()
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	w := &failingResponseWriter{ResponseWriter: rec, failOnWrite: true}
	req := httptest.NewRequest("GET", "/fail", nil)
	wrapped.ServeHTTP(w, req)
}

func TestClientErrorMiddleware_EncodeError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	mw := ClientErrorMiddleware()
	wrapped := mw(handler)

	rec := httptest.NewRecorder()
	w := &failingResponseWriter{ResponseWriter: rec, failOnWrite: true}
	req := httptest.NewRequest("GET", "/bad", nil)
	wrapped.ServeHTTP(w, req)
}