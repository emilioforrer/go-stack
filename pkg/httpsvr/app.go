package httpsvr

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	ErrServerNotRunning     = errors.New("server is not running")
	ErrServerForcedShutdown = errors.New("server forced shutdown")
)

type App interface {
	IsRunning() bool
	Running() <-chan bool
	Run() error
	Stop() error
}

type DefaultApp struct {
	server  HTTPServer
	Router  Router
	Config  DefaultConfig
	runChan chan bool
	running bool
	mu      sync.RWMutex
}

func NewApp() *DefaultApp {
	router := NewDefaultRouter()
	router.Use(RequestLoggerMiddleware(true))
	config := NewDefaultConfig(router)

	return &DefaultApp{
		Router:  router,
		Config:  config,
		running: false,
		runChan: make(chan bool, 1),
		server:  nil,
	}
}

func (a *DefaultApp) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

func (a *DefaultApp) Stop() error {
	a.mu.RLock()
	isRunning := a.running
	a.mu.RUnlock()

	if !isRunning {
		return ErrServerNotRunning
	}

	return a.server.Stop(context.Background())
}

func (a *DefaultApp) Running() <-chan bool {
	return a.runChan
}

func (a *DefaultApp) Run() error {
	config := a.Config
	if a.server == nil {
		a.server = NewDefaultHTTPServer(config)
	}
	server := a.server

	shutdownTimeout := config.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	done := make(chan os.Signal, 1)
	errorChan := make(chan error, 1)

	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	slog.Info("Server is running...")

	a.mu.Lock()
	a.running = true
	a.mu.Unlock()
	a.runChan <- true

	go func() {
		slog.Info("Starting server", "addr", config.Addr)
		if err := server.Start(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorChan <- err
		}
	}()

	return a.waitForShutdown(ctx, server, done, errorChan, shutdownTimeout)
}

func (a *DefaultApp) waitForShutdown(
	ctx context.Context,
	server HTTPServer,
	done chan os.Signal,
	errorChan chan error,
	shutdownTimeout time.Duration,
) error {
	select {
	case <-done:
		slog.Info("Server is shutting down...")
		return a.gracefulShutdown(ctx, server, shutdownTimeout)
	case err := <-errorChan:
		slog.Error("Server error", "error", err)
		return err
	}
}

func (a *DefaultApp) gracefulShutdown(
	ctx context.Context,
	server HTTPServer,
	shutdownTimeout time.Duration,
) error {
	shutdownComplete := make(chan struct{})
	go func() {
		if err := server.Stop(ctx); err != nil {
			slog.Error("Server shutdown error", "error", err)
		}
		close(shutdownComplete)
	}()

	select {
	case <-shutdownComplete:
		slog.Info("Server gracefully shutdown completed")
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		<-a.runChan
		a.runChan <- false
		slog.Info("Server stopped gracefully")
		return nil
	case <-time.After(shutdownTimeout):
		slog.Error("Shutdown timeout exceeded. Forcing exit.")
		return ErrServerForcedShutdown
	}
}
