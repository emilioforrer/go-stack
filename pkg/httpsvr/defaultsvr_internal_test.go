package httpsvr

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewDefaultConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	config := NewDefaultConfig(handler)
	if config.Addr != ":4000" {
		t.Fatalf("expected :4000, got %s", config.Addr)
	}
	if config.ReadTimeout != 30*time.Second {
		t.Fatalf("expected 30s read timeout, got %v", config.ReadTimeout)
	}
	if config.WriteTimeout != 30*time.Second {
		t.Fatalf("expected 30s write timeout, got %v", config.WriteTimeout)
	}
	if config.IdleTimeout != 30*time.Second {
		t.Fatalf("expected 30s idle timeout, got %v", config.IdleTimeout)
	}
	if config.ReadHeaderTimeout != 30*time.Second {
		t.Fatalf("expected 30s read header timeout, got %v", config.ReadHeaderTimeout)
	}
	if config.MaxHeaderBytes != 1<<20 {
		t.Fatalf("expected %d max header bytes, got %d", 1<<20, config.MaxHeaderBytes)
	}
	if config.Handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestDefaultHTTPServer_StartAndStop(t *testing.T) {
	router := NewDefaultRouter()
	router.GET("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	config := NewDefaultConfig(router)
	config.Addr = ":0"

	server := NewDefaultHTTPServer(config)

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(context.Background())
	}()

	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		t.Fatalf("unexpected error on stop: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			t.Fatalf("unexpected error from Start: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start to return")
	}
}