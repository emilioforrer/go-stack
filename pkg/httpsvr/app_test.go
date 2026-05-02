package httpsvr

import (
	"context"
	"errors"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("expected non-nil app")
	}
	if app.IsRunning() {
		t.Fatal("expected app to not be running initially")
	}
	if app.Router == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestDefaultApp_StopNotRunning(t *testing.T) {
	app := NewApp()
	err := app.Stop()
	if !errors.Is(err, ErrServerNotRunning) {
		t.Fatalf("expected ErrServerNotRunning, got %v", err)
	}
}

func TestDefaultApp_Running(t *testing.T) {
	app := NewApp()
	ch := app.Running()
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestDefaultApp_IsRunning(t *testing.T) {
	app := NewApp()
	if app.IsRunning() {
		t.Fatal("expected app to not be running")
	}
}

func TestDefaultApp_Stop_WithMockServer(t *testing.T) {
	app := NewApp()
	app.running = true
	app.server = &mockServerForApp{}
	err := app.Stop()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaultApp_Run_GracefulShutdown(t *testing.T) {
	router := NewDefaultRouter()
	router.GET("/", func(w http.ResponseWriter, r *http.Request) {})
	config := NewDefaultConfig(router)
	config.Addr = ":0"

	app := &DefaultApp{
		Router:  router,
		Config:  config,
		running: false,
		runChan: make(chan bool, 1),
		server:  nil,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	waitForRunning(t, app, 2*time.Second)

	if !app.IsRunning() {
		t.Fatal("expected app to be running")
	}

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error from Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to return after SIGTERM")
	}
}

func TestDefaultApp_Run_ServerError(t *testing.T) {
	app := &DefaultApp{
		Router:  NewDefaultRouter(),
		Config:  DefaultConfig{Addr: "invalid-addr:99999", Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})},
		running: false,
		runChan: make(chan bool, 1),
		server:  nil,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	waitForRunning(t, app, 2*time.Second)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Run")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to return with error")
	}
}

func TestDefaultApp_Run_ForcedShutdown(t *testing.T) {
	config := DefaultConfig{
		Addr:            ":0",
		Handler:         http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ShutdownTimeout: 50 * time.Millisecond,
	}

	app := &DefaultApp{
		Router:  NewDefaultRouter(),
		Config:  config,
		running: false,
		runChan: make(chan bool, 1),
		server:  &blockingMockServer{unblock: make(chan struct{})},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	waitForRunning(t, app, 2*time.Second)

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrServerForcedShutdown) {
			t.Fatalf("expected ErrServerForcedShutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for forced shutdown")
	}
}

func TestDefaultApp_Run_ServerStopError(t *testing.T) {
	router := NewDefaultRouter()
	router.GET("/", func(w http.ResponseWriter, r *http.Request) {})
	config := NewDefaultConfig(router)
	config.Addr = ":0"

	app := &DefaultApp{
		Router:  router,
		Config:  config,
		running: false,
		runChan: make(chan bool, 1),
		server:  &errorStopMockServer{},
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run()
	}()

	waitForRunning(t, app, 2*time.Second)

	p, _ := os.FindProcess(os.Getpid())
	_ = p.Signal(syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("unexpected error from Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Run to return")
	}
}

func waitForRunning(t *testing.T, app *DefaultApp, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if app.IsRunning() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for app to be running")
}

type mockServerForApp struct{}

func (m *mockServerForApp) Start(ctx context.Context) error { return nil }
func (m *mockServerForApp) Stop(ctx context.Context) error  { return nil }

type blockingMockServer struct {
	unblock chan struct{}
}

func (m *blockingMockServer) Start(ctx context.Context) error { return nil }
func (m *blockingMockServer) Stop(ctx context.Context) error {
	<-m.unblock
	return nil
}

type errorStopMockServer struct{}

func (m *errorStopMockServer) Start(ctx context.Context) error { return nil }
func (m *errorStopMockServer) Stop(ctx context.Context) error {
	return errors.New("shutdown error")
}
