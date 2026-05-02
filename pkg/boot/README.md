# boot

An application bootstrapping and lifecycle management package built on top of [samber/do](https://github.com/samber/do) dependency injection.

It provides a structured way to **register**, **boot**, and **shutdown** service providers in a deterministic order, with a shared DI container.

## Install

```bash
go get github.com/emilioforrer/go-stack/pkg/boot
```

## Key Concepts

| Type | Role |
|---|---|
| `Container` | Wraps a `do.Injector` and acts as the shared service locator |
| `Provider` | Defines `Register`, `Boot`, and `Shutdown` hooks for a unit of functionality |
| `DefaultProvider` | Embeddable no-op base — override only the hooks you need |
| `DefaultBootstrapper` | Orchestrates the full lifecycle across all registered providers |

### Lifecycle order

1. **Register** — called once per provider (in order) to bind services into the container.
2. **Boot** — called once per provider (in order) after all registrations are complete.
3. **Shutdown** — called once per provider (in **reverse** order) for graceful teardown.

All three phases are idempotent: calling them more than once is a safe no-op.

## Usage

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

// GreeterService is a simple service we want to manage.
type GreeterService struct {
	Greeting string
}

// GreeterProvider registers and boots the GreeterService.
type GreeterProvider struct {
	boot.DefaultProvider // embed the no-op base
}

func (p *GreeterProvider) Register(_ context.Context, c boot.Container) error {
	// Provide the service into the DI container.
	do.Provide(c.Injector(), func(i do.Injector) (*GreeterService, error) {
		return &GreeterService{Greeting: "Hello from go-stack!"}, nil
	})
	return nil
}

func (p *GreeterProvider) Boot(_ context.Context, c boot.Container) error {
	// Resolve and use the service during boot.
	svc, err := do.Invoke[*GreeterService](c.Injector())
	if err != nil {
		return err
	}
	fmt.Println(svc.Greeting)
	return nil
}

func (p *GreeterProvider) Shutdown(_ context.Context, _ boot.Container) error {
	fmt.Println("GreeterProvider: shutting down")
	return nil
}

func main() {
	ctx := context.Background()

	// Create the bootstrapper with one or more providers.
	b := boot.NewDefaultBootstrapper(
		do.New(),
		&GreeterProvider{},
	)

	if err := b.Register(ctx); err != nil {
		log.Fatal(err)
	}
	if err := b.Boot(ctx); err != nil {
		log.Fatal(err)
	}

	// ... application runs ...

	if err := b.Shutdown(ctx); err != nil {
		log.Fatal(err)
	}
}
```

**Output:**

```
Hello from go-stack!
GreeterProvider: shutting down
```
