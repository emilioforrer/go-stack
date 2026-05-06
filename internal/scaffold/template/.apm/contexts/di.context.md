---
description: Dependency injection standards using samber/do v2
applyTo: '**/*.go'
---

# Dependency Injection Standards

## Framework

The project uses [`github.com/samber/do/v2`](https://do.samber.dev/) as its sole dependency injection framework. No other DI framework is permitted.


## Reference

- samber/do v2 docs: https://do.samber.dev/
- Package loading: https://do.samber.dev/docs/service-registration/package-loading
- Scoping: https://do.samber.dev/docs/container/scope

## Package Loading Pattern

Every new package that provides services to the DI container **MUST** contain a `package.go` file at the package root. This file is the single place where all services from that package are registered, using `do.Package()`.

This pattern keeps service registration centralized and makes it trivial for the boot process to wire an entire package into the container.

### Package `package.go` Example

```go
package api

import "github.com/samber/do/v2"

import (
    "github.com/example/app/internal/user"
    "github.com/example/app/internal/order"
)

// Package registers all services provided by this package.
var Package = do.Package(
    // Nested feature packages
    user.Package,
    order.Package,

    // Lazy services (created on first invocation)
    do.Lazy(NewRouter),
    do.Lazy(NewMiddleware),

    // Eager service (created immediately on boot)
    do.Eager(NewConfig),
)
```

### Boot Provider Register Example

Inside a boot provider's `Register` method, use `container.Injector()` to pass the container to the package's `do.Package` variable.

```go
package provider

import (
    "context"

    "github.com/emilioforrer/go-stack/internal/user"
    "github.com/emilioforrer/go-stack/pkg/boot"
)

type UserProvider struct{}

func (p *UserProvider) Register(_ context.Context, container boot.Container) error {
    user.Package(container.Injector())
    return nil
}

func (p *UserProvider) Boot(_ context.Context, _ boot.Container) error {
    return nil
}

func (p *UserProvider) Shutdown(_ context.Context, _ boot.Container) error {
    return nil
}
```

### Rules

1. **One `package.go` per package.** All `do.Lazy`, `do.Eager`, `do.Transient`, `do.Bind`, etc., calls for a package live in that file.
2. **No ad-hoc registration outside `package.go`.** Do not call `do.Provide` or similar functions scattered across other files in the same package.
3. **Export the variable as `Package`.** Use `var Package = do.Package(...)` so the boot process can reference it consistently.


