`cmd/app/main.go`

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/boot-example/internal/config"
	"example.com/boot-example/internal/database"
	"example.com/boot-example/internal/server"
	"example.com/boot-example/internal/worker"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

const (
	databaseDSN     = "memory://example"
	serverAddr      = "127.0.0.1:18080"
	workerInterval  = 30 * time.Second
	shutdownTimeout = 10 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("application stopped with error", slog.Any("error", err))
		os.Exit(1)
	}
}

func run(ctx context.Context) (err error) {
	bootstrapper := boot.NewDefaultBootstrapper(
		do.New(),
		config.NewProvider(config.Config{
			DatabaseDSN:    databaseDSN,
			ServerAddr:     serverAddr,
			WorkerInterval: workerInterval,
		}),
		database.NewProvider(),
		worker.NewProvider(),
		server.NewProvider(),
	)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.WithoutCancel(ctx),
			shutdownTimeout,
		)
		defer cancel()

		if shutdownErr := bootstrapper.Shutdown(shutdownCtx); shutdownErr != nil {
			err = errors.Join(
				err,
				fmt.Errorf("shutdown application: %w", shutdownErr),
			)
		}
	}()

	if err := bootstrapper.Register(ctx); err != nil {
		return fmt.Errorf("register application: %w", err)
	}

	if err := bootstrapper.Boot(ctx); err != nil {
		return fmt.Errorf("boot application: %w", err)
	}

	<-ctx.Done()
	return nil
}
```

`internal/config/provider.go`

```go
package config

import (
	"context"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

type Config struct {
	DatabaseDSN    string
	ServerAddr     string
	WorkerInterval time.Duration
}

type Provider struct {
	boot.DefaultProvider
	config Config
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider(config Config) boot.Provider {
	return &Provider{config: config}
}

func (p *Provider) Register(_ context.Context, container boot.Container) error {
	do.ProvideValue(container.Injector(), &p.config)
	return nil
}
```

`internal/database/provider.go`

```go
package database

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"example.com/boot-example/internal/config"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

var (
	errDSNRequired   = errors.New("database provider: dsn is required")
	errNotConnected  = errors.New("database: not connected")
	errNotRegistered = errors.New("database provider: register must run before boot")
)

type Client struct {
	mu        sync.RWMutex
	connected bool
}

func (c *Client) Connect(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("connect database: %w", err)
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.connected {
		return errNotConnected
	}
	return nil
}

func (c *Client) Close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return nil
}

type Provider struct {
	boot.DefaultProvider
	client *Client
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider {
	return &Provider{}
}

func (p *Provider) Register(_ context.Context, container boot.Container) error {
	cfg, err := do.Invoke[*config.Config](container.Injector())
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}
	if cfg.DatabaseDSN == "" {
		return errDSNRequired
	}

	p.client = &Client{}
	do.ProvideValue(container.Injector(), p.client)
	return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
	if p.client == nil {
		return errNotRegistered
	}
	return p.client.Connect(ctx)
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
	if p.client == nil {
		return nil
	}
	return p.client.Close(ctx)
}
```

`internal/worker/provider.go`

```go
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"example.com/boot-example/internal/config"
	"example.com/boot-example/internal/database"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

var (
	errIntervalRequired = errors.New("worker provider: interval must be positive")
	errNotRegistered    = errors.New("worker provider: register must run before boot")
)

type service struct {
	database *database.Client
	interval time.Duration
}

func (s *service) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.database.Ping(ctx); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				slog.Error("worker database ping failed", slog.Any("error", err))
				continue
			}
			slog.Info("worker tick")
		}
	}
}

type Provider struct {
	boot.DefaultProvider
	service *service
	cancel  context.CancelFunc
	done    chan struct{}
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider {
	return &Provider{}
}

func (p *Provider) Register(_ context.Context, container boot.Container) error {
	cfg, err := do.Invoke[*config.Config](container.Injector())
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}
	if cfg.WorkerInterval <= 0 {
		return errIntervalRequired
	}

	db, err := do.Invoke[*database.Client](container.Injector())
	if err != nil {
		return fmt.Errorf("resolve database: %w", err)
	}

	p.service = &service{database: db, interval: cfg.WorkerInterval}
	return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
	if p.service == nil {
		return errNotRegistered
	}

	workerCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})

	go func() {
		defer close(p.done)
		p.service.run(workerCtx)
	}()
	return nil
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
	if p.cancel == nil {
		return nil
	}

	p.cancel()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for worker: %w", ctx.Err())
	}
}
```

`internal/server/provider.go`

```go
package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"time"

	"example.com/boot-example/internal/config"
	"example.com/boot-example/internal/database"
	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
)

var (
	errAddrRequired  = errors.New("server provider: address is required")
	errNotRegistered = errors.New("server provider: register must run before boot")
)

type Provider struct {
	boot.DefaultProvider
	server   *http.Server
	listener net.Listener
	done     chan error
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider {
	return &Provider{}
}

func (p *Provider) Register(_ context.Context, container boot.Container) error {
	cfg, err := do.Invoke[*config.Config](container.Injector())
	if err != nil {
		return fmt.Errorf("resolve config: %w", err)
	}
	if cfg.ServerAddr == "" {
		return errAddrRequired
	}

	db, err := do.Invoke[*database.Client](container.Injector())
	if err != nil {
		return fmt.Errorf("resolve database: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthHandler(db))

	p.server = &http.Server{
		Addr:              cfg.ServerAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
	if p.server == nil {
		return errNotRegistered
	}

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", p.server.Addr)
	if err != nil {
		return fmt.Errorf("listen for http server: %w", err)
	}

	p.listener = listener
	p.done = make(chan error, 1)

	go func() {
		err := p.server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		if err != nil {
			slog.Error("http server stopped unexpectedly", slog.Any("error", err))
		}
		p.done <- err
		close(p.done)
	}()
	return nil
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
	if p.server == nil || p.listener == nil {
		return nil
	}

	if err := p.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	select {
	case err := <-p.done:
		if err != nil {
			return fmt.Errorf("serve http: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("wait for http server: %w", ctx.Err())
	}
}

func healthHandler(db *database.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if _, err := io.WriteString(w, "ok\n"); err != nil {
			slog.Warn("write health response failed", slog.Any("error", err))
		}
	})
}
```
