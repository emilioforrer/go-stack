package provider

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/emilioforrer/go-stack/pkg/httpsvr"
	"github.com/samber/do/v2"
)

type mockContainer struct {
	injector do.Injector
}

func (m *mockContainer) Injector() do.Injector {
	return m.injector
}

func newMockContainer() boot.Container {
	return &mockContainer{
		injector: do.New(),
	}
}

type mockHTTPServer struct {
	startErr    error
	stopErr     error
	startCalled bool
	stopCalled  bool
}

func (m *mockHTTPServer) Start(_ context.Context) error {
	m.startCalled = true
	return m.startErr
}

func (m *mockHTTPServer) Stop(_ context.Context) error {
	m.stopCalled = true
	return m.stopErr
}

var _ httpsvr.HTTPServer = (*mockHTTPServer)(nil)

func TestNewServerProvider_WithDefaults(t *testing.T) {
	provider := NewServerProvider()

	if provider.options.Host != "0.0.0.0" {
		t.Errorf("expected Host to be 0.0.0.0, got %s", provider.options.Host)
	}

	if provider.options.Port != 8888 {
		t.Errorf("expected Port to be 8888, got %d", provider.options.Port)
	}

	if provider.server != nil {
		t.Error("expected server to be nil before Register is called")
	}
}

func TestNewServerProvider_WithCustomOptions(t *testing.T) {
	opts := ServerOptions{
		Debug: true,
		Host:  "127.0.0.1",
		Port:  9999,
	}

	provider := NewServerProvider(opts)

	if provider.options.Host != "127.0.0.1" {
		t.Errorf("expected Host to be 127.0.0.1, got %s", provider.options.Host)
	}

	if provider.options.Port != 9999 {
		t.Errorf("expected Port to be 9999, got %d", provider.options.Port)
	}

	if !provider.options.Debug {
		t.Error("expected Debug to be true")
	}
}

func TestNewServerProvider_WithEmptyHostFallback(t *testing.T) {
	opts := ServerOptions{
		Host: "",
		Port: 9999,
	}

	provider := NewServerProvider(opts)

	if provider.options.Host != "0.0.0.0" {
		t.Errorf("expected Host to fallback to 0.0.0.0, got %s", provider.options.Host)
	}
}

func TestNewServerProvider_WithZeroPortFallback(t *testing.T) {
	opts := ServerOptions{
		Host: "127.0.0.1",
		Port: 0,
	}

	provider := NewServerProvider(opts)

	if provider.options.Port != 8888 {
		t.Errorf("expected Port to fallback to 8888, got %d", provider.options.Port)
	}
}

func TestServerProvider_Register(t *testing.T) {
	provider := NewServerProvider(ServerOptions{
		Host: "localhost",
		Port: 8080,
	})

	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if provider.server == nil {
		t.Fatal("expected server to be initialized after Register")
	}

	if provider.addr != "localhost:8080" {
		t.Errorf("expected addr to be localhost:8080, got %s", provider.addr)
	}
}

func TestServerProvider_BootBeforeRegister(t *testing.T) {
	provider := NewServerProvider()
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Boot(ctx, container)
	if err == nil {
		t.Fatal("expected Boot to fail when called before Register")
	}

	if !errors.Is(err, ErrServerNotInitialized) {
		t.Errorf("expected ErrServerNotInitialized, got %v", err)
	}
}

func TestServerProvider_BootAndShutdown(t *testing.T) {
	provider := NewServerProvider(ServerOptions{
		Host: "127.0.0.1",
		Port: 48080,
	})

	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	err = provider.Boot(ctx, container)
	if err != nil {
		t.Fatalf("Boot failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + provider.addr + "/docs")
	if err == nil {
		resp.Body.Close()
	}

	err = provider.Shutdown(ctx, container)
	if err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_, err = http.Get("http://" + provider.addr + "/docs")
	if err == nil {
		t.Error("expected request to fail after shutdown, but it succeeded")
	}
}

func TestServerProvider_ShutdownWithoutServer(t *testing.T) {
	provider := NewServerProvider()
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Shutdown(ctx, container)
	if err != nil {
		t.Fatalf("expected Shutdown to handle nil server gracefully, got %v", err)
	}
}

func TestServerProvider_BootWithServerError(t *testing.T) {
	provider := NewServerProvider()
	container := newMockContainer()
	ctx := context.Background()

	mockServer := &mockHTTPServer{
		startErr: errors.New("listen error"),
	}
	provider.setServer(mockServer, "127.0.0.1:8080")

	err := provider.Boot(ctx, container)
	if err != nil {
		t.Fatalf("Boot failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !mockServer.startCalled {
		t.Error("expected Start to be called")
	}
}

func TestServerProvider_BootWithErrServerClosed(t *testing.T) {
	provider := NewServerProvider()
	container := newMockContainer()
	ctx := context.Background()

	mockServer := &mockHTTPServer{
		startErr: http.ErrServerClosed,
	}
	provider.setServer(mockServer, "127.0.0.1:8080")

	err := provider.Boot(ctx, container)
	if err != nil {
		t.Fatalf("Boot failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !mockServer.startCalled {
		t.Error("expected Start to be called")
	}
}

func TestServerProvider_ShutdownWithError(t *testing.T) {
	provider := NewServerProvider()
	container := newMockContainer()
	ctx := context.Background()

	mockServer := &mockHTTPServer{
		stopErr: errors.New("shutdown error"),
	}
	provider.setServer(mockServer, "127.0.0.1:8080")

	err := provider.Shutdown(ctx, container)
	if err == nil {
		t.Fatal("expected Shutdown to return an error")
	}

	if !errors.Is(err, errors.New("shutdown error")) && err.Error() != "server shutdown: shutdown error" {
		t.Errorf("expected wrapped shutdown error, got %v", err)
	}

	if !mockServer.stopCalled {
		t.Error("expected Stop to be called")
	}
}

func TestServerProvider_ImplementsProvider(t *testing.T) {
	var _ boot.Provider = (*ServerProvider)(nil)
}
