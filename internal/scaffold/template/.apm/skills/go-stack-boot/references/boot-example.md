# Thin Boot Composition Example

This example demonstrates the intended split:

```text
cmd/app/main.go          → configure + compose + supervise
internal/database/       → database behavior + thin boot provider
internal/worker/         → processor behavior + thin boot provider
internal/server/         → HTTP behavior + thin boot provider
```

Providers contain lifecycle wiring only. The actual server and worker logic are
ordinary package types.

## `cmd/app/main.go`

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

    "example.com/app/internal/config"
    "example.com/app/internal/database"
    "example.com/app/internal/server"
    "example.com/app/internal/supervisor"
    "example.com/app/internal/worker"
    "github.com/emilioforrer/go-stack/pkg/boot"
    "github.com/samber/do/v2"
)

const shutdownTimeout = 10 * time.Second

func main() {
    ctx, stop := signal.NotifyContext(
        context.Background(),
        os.Interrupt,
        syscall.SIGTERM,
    )
    defer stop()

    if err := run(ctx); err != nil {
        slog.Error("application stopped", slog.Any("error", err))
        os.Exit(1)
    }
}

func run(ctx context.Context) (err error) {
    injector := do.New()

    cfg := &config.Config{
        DatabaseDSN:   "memory://example",
        ServerAddr:    "127.0.0.1:8080",
        WorkerInterval: 30 * time.Second,
    }
    fatal := supervisor.New()

    // Top-level values do not need dedicated providers merely to exist in DI.
    do.ProvideValue(injector, cfg)
    do.ProvideValue(injector, fatal)

    bootstrapper := boot.NewDefaultBootstrapper(
        injector,
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
            err = errors.Join(err, fmt.Errorf("shutdown application: %w", shutdownErr))
        }
    }()

    if err := bootstrapper.Register(ctx); err != nil {
        return fmt.Errorf("register application: %w", err)
    }
    if err := bootstrapper.Boot(ctx); err != nil {
        return fmt.Errorf("boot application: %w", err)
    }

    select {
    case <-ctx.Done():
        return nil
    case err := <-fatal.Errors():
        return fmt.Errorf("critical runtime failure: %w", err)
    }
}
```

## `internal/supervisor/supervisor.go`

```go
package supervisor

import "sync"

type Supervisor struct {
    once sync.Once
    ch   chan error
}

func New() *Supervisor {
    return &Supervisor{ch: make(chan error, 1)}
}

func (s *Supervisor) Report(err error) {
    if err == nil {
        return
    }
    s.once.Do(func() { s.ch <- err })
}

func (s *Supervisor) Errors() <-chan error { return s.ch }
```

## `internal/database/provider.go`

The provider only creates/registers the client and delegates lifecycle methods.

```go
package database

import (
    "context"
    "fmt"

    "example.com/app/internal/config"
    "github.com/emilioforrer/go-stack/pkg/boot"
    "github.com/samber/do/v2"
)

type Provider struct {
    boot.DefaultProvider
    client *Client
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider { return &Provider{} }

func (p *Provider) Register(_ context.Context, c boot.Container) error {
    cfg, err := do.Invoke[*config.Config](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve config: %w", err)
    }

    p.client = NewClient(cfg.DatabaseDSN)
    do.ProvideValue(c.Injector(), p.client)
    return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
    if p.client == nil {
        return nil
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

`Client`, its connection behavior, and database-specific logic belong in normal
`database` files, not `provider.go`.

## `internal/worker/processor.go`

```go
package worker

import (
    "context"
    "fmt"
    "sync"
    "time"

    "example.com/app/internal/database"
)

type Processor struct {
    db       *database.Client
    interval time.Duration
    cancel   context.CancelFunc
    done     chan struct{}
    once     sync.Once
}

func New(db *database.Client, interval time.Duration) *Processor {
    return &Processor{db: db, interval: interval}
}

func (p *Processor) Start(ctx context.Context) error {
    if p.interval <= 0 {
        return fmt.Errorf("worker interval must be positive")
    }

    workerCtx, cancel := context.WithCancel(ctx)
    p.cancel = cancel
    p.done = make(chan struct{})

    go func() {
        defer close(p.done)
        ticker := time.NewTicker(p.interval)
        defer ticker.Stop()

        for {
            select {
            case <-workerCtx.Done():
                return
            case <-ticker.C:
                _ = p.db.Ping(workerCtx)
            }
        }
    }()

    return nil
}

func (p *Processor) Shutdown(ctx context.Context) error {
    if p.cancel == nil {
        return nil
    }
    p.once.Do(p.cancel)

    select {
    case <-p.done:
        return nil
    case <-ctx.Done():
        return fmt.Errorf("wait for worker: %w", ctx.Err())
    }
}
```

## `internal/worker/provider.go`

```go
package worker

import (
    "context"
    "fmt"

    "example.com/app/internal/config"
    "example.com/app/internal/database"
    "github.com/emilioforrer/go-stack/pkg/boot"
    "github.com/samber/do/v2"
)

type Provider struct {
    boot.DefaultProvider
    processor *Processor
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider { return &Provider{} }

func (p *Provider) Register(_ context.Context, c boot.Container) error {
    cfg, err := do.Invoke[*config.Config](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve config: %w", err)
    }
    db, err := do.Invoke[*database.Client](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve database: %w", err)
    }

    p.processor = New(db, cfg.WorkerInterval)
    do.ProvideValue(c.Injector(), p.processor)
    return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
    if p.processor == nil {
        return nil
    }
    return p.processor.Start(ctx)
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
    if p.processor == nil {
        return nil
    }
    return p.processor.Shutdown(ctx)
}
```

## `internal/server/server.go`

```go
package server

import (
    "context"
    "errors"
    "fmt"
    "net"
    "net/http"

    "example.com/app/internal/supervisor"
)

type Server struct {
    http  *http.Server
    fatal *supervisor.Supervisor
    done  chan error
}

func New(addr string, handler http.Handler, fatal *supervisor.Supervisor) *Server {
    return &Server{
        http:  &http.Server{Addr: addr, Handler: handler},
        fatal: fatal,
    }
}

func (s *Server) Start(ctx context.Context) error {
    listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", s.http.Addr)
    if err != nil {
        return fmt.Errorf("listen: %w", err)
    }

    s.done = make(chan error, 1)
    go func() {
        err := s.http.Serve(listener)
        if errors.Is(err, http.ErrServerClosed) {
            err = nil
        }
        s.done <- err
        close(s.done)
        if err != nil {
            s.fatal.Report(fmt.Errorf("http server: %w", err))
        }
    }()
    return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
    if s.done == nil {
        return nil
    }
    if err := s.http.Shutdown(ctx); err != nil {
        return err
    }

    select {
    case err := <-s.done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

HTTP handlers and route construction should live in separate ordinary files
such as `handler.go` / `routes.go`.

## `internal/server/provider.go`

```go
package server

import (
    "context"
    "fmt"
    "net/http"

    "example.com/app/internal/config"
    "example.com/app/internal/supervisor"
    "github.com/emilioforrer/go-stack/pkg/boot"
    "github.com/samber/do/v2"
)

type Provider struct {
    boot.DefaultProvider
    server *Server
}

var _ boot.Provider = (*Provider)(nil)

func NewProvider() boot.Provider { return &Provider{} }

func (p *Provider) Register(_ context.Context, c boot.Container) error {
    cfg, err := do.Invoke[*config.Config](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve config: %w", err)
    }
    handler, err := do.Invoke[http.Handler](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve http handler: %w", err)
    }
    fatal, err := do.Invoke[*supervisor.Supervisor](c.Injector())
    if err != nil {
        return fmt.Errorf("resolve supervisor: %w", err)
    }

    p.server = New(cfg.ServerAddr, handler, fatal)
    do.ProvideValue(c.Injector(), p.server)
    return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
    if p.server == nil {
        return nil
    }
    return p.server.Start(ctx)
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
    if p.server == nil {
        return nil
    }
    return p.server.Shutdown(ctx)
}
```

## Key Takeaway

Boot providers are **thin adapters around application packages/services**:

```text
DI + lifecycle coordination
          ↓
      Provider
          ↓
Real package/service implementation
```

The shared injector allows providers to compose any registered application
dependency, while normal runtime code stays explicit and independent from the
DI container.
