---
name: go-stack-boot
description: >
  Use go-stack's pkg/boot package to compose Go application startup, dependency
  registration, long-running services, and graceful shutdown with samber/do.
  Apply when adding or reviewing boot providers, wiring an executable's
  lifecycle, ordering runtime dependencies, or testing startup and teardown.
metadata:
  author: go-stack
  version: "1.1.0"
---

# Go Stack Boot

## Purpose

Use `github.com/emilioforrer/go-stack/pkg/boot` at an executable's composition
boundary when several application-scoped runtime components need a shared
`samber/do` injector and a deterministic lifecycle.

The lifecycle is:

1. `Register` every provider in declaration order.
2. `Boot` every provider in declaration order, after registration succeeds.
3. `Shutdown` every provider in reverse declaration order.

Use `pkg/boot` for lifecycle orchestration, not for business logic.

## Terminology

Do not confuse these two concepts:

- `boot.Provider` is an application lifecycle component with `Register`, `Boot`,
  and `Shutdown` methods.
- `do.Provider[T]` is a `samber/do` constructor function that creates a dependency.

A runtime package may use both, but they have different responsibilities.

## When to Use It

Create a `boot.Provider` for an application-scoped runtime component when it:

- registers dependencies in the application's `do.Injector`;
- must initialize or verify a resource before the application is ready;
- starts a server, consumer, scheduler, or other long-running process;
- owns a resource that must stop or close during graceful shutdown; or
- has startup or teardown ordering constraints relative to another component.

Typical providers manage configuration, databases, migrations, HTTP servers,
message consumers, caches, telemetry, or background workers.

Do not create a provider for:

- pure domain or use-case logic;
- stateless helper packages;
- request-scoped work;
- resources whose lifetime is confined to one operation.

Keep business logic and ordinary constructors independent from `boot.Container`
and `do.Injector`.

For a small command with one plainly owned resource, direct construction and a
local `defer` can be clearer than a bootstrapper. Use `pkg/boot` when coordinated
application lifecycle and dependency ordering provide real value.

## Non-Negotiable Lifecycle Rules

- Declare providers in dependency order.
- Keep lifecycle method calls sequential. Do not call `Register`, `Boot`, or
  `Shutdown` concurrently on the same bootstrapper.
- Call `Register` before `Boot`; otherwise `Boot` returns
  `boot.ErrBootBeforeRegister`.
- A successful `Register` or `Boot` call is idempotent for that bootstrapper.
- `Shutdown` is attempted at most once by `DefaultBootstrapper`, even when one or
  more provider shutdowns fail.
- After a failed `Register` or `Boot`, do not retry that phase on the same
  bootstrapper. Request `Shutdown` and create a fresh bootstrapper if startup
  must be attempted again.
- Arrange cleanup before calling `Register`, because registration or boot may
  partially initialize resources before returning an error.
- Every provider's `Shutdown` must tolerate partial initialization.

## Provider Placement and Ownership

Keep a provider with the runtime component whose lifecycle it owns. Prefer:

```text
internal/config/provider.go
internal/database/provider.go
internal/worker/provider.go
internal/server/provider.go
```

rather than a large generic package containing unrelated provider implementations.

Registration must remain owned by the runtime package. A provider may register
its dependencies directly in its package, or delegate to that package's existing
`Package` function when the project uses package-based `samber/do` registration.

Example delegation:

```go
func (p *Provider) Register(_ context.Context, container boot.Container) error {
    database.Package(p.config)(container.Injector())
    return nil
}
```

Do not move domain constructors or feature registrations into the executable just
because the executable owns provider ordering.

A resource must have one lifecycle owner. `DefaultBootstrapper.Shutdown` calls
`boot.Provider.Shutdown`; it does not automatically shut down the `samber/do`
injector. If a resource could also be shut down by `samber/do`, do not independently
close the same resource through both mechanisms.

## Implementing a Provider

Use a pointer receiver. Embed `boot.DefaultProvider` when only some lifecycle hooks
need custom behavior, and add a compile-time interface assertion:

```go
type Provider struct {
    boot.DefaultProvider
}

var _ boot.Provider = (*Provider)(nil)
```

Returning `boot.Provider` from a constructor is appropriate when callers only need
to compose and order the provider:

```go
func NewProvider() boot.Provider {
    return &Provider{}
}
```

### Register

Use `Register` to:

- validate configuration required for dependency assembly;
- construct or register application-scoped dependencies;
- expose values or package registrations through `container.Injector()`.

Keep registration deterministic. Do not start long-running goroutines in this
phase.

A provider may resolve a dependency during `Register` only when an earlier
provider has already registered it. Prefer lazy `samber/do` registrations when
construction can wait until `Boot` or first use.

### Boot

Use `Boot` for work that makes the component ready:

- resolve registered dependencies;
- verify connections;
- run required startup work such as migrations;
- start long-running processes.

Return startup failures so later providers do not boot.

Long-running work must not block `Boot`. Start it through an owned goroutine or an
API that returns after startup. The provider must arrange for that work to observe
the application context or stop it explicitly in `Shutdown`.

`DefaultBootstrapper` does not supervise failures that happen after `Boot`
returns. At minimum log unexpected background failures. If a background process
is critical to application correctness, propagate the failure to an application
supervisor that can cancel or terminate the application.

At the start of `Boot`, check state that `Register` must initialize and return a
descriptive error when it is missing. This keeps providers safe to test directly.

### Shutdown

Use `Shutdown` to stop and close only resources the provider owns.

It must:

- tolerate partial initialization and return `nil` when there is nothing to do;
- honor the supplied context for bounded cleanup;
- wait for owned goroutines when necessary;
- return cleanup errors instead of swallowing them.

The bootstrapper calls every provider's `Shutdown` in reverse declaration order
and joins returned errors. It does not automatically roll back a failed
`Register` or `Boot`; the composition root must request shutdown.

## Ordering Providers

Provider declaration order is lifecycle dependency order.

Place provider A before provider B when B:

- resolves A's registrations during `Register`;
- requires A to be ready before B's `Boot` can succeed; or
- must stop before A is closed.

For example:

```text
Config
  ↓
Database
  ↓
Worker
  ↓
HTTP Server
```

Reverse shutdown becomes:

```text
HTTP Server
  ↓
Worker
  ↓
Database
  ↓
Config
```

Do not rely on provider ordering to hide an undeclared business dependency.
Preserve feature boundaries and expose dependencies through package registrations
and application ports.

## Compose the Executable

The executable is the composition root. It owns:

- the `do.Injector`;
- provider construction and ordering;
- the OS-signal/application context;
- startup error propagation;
- the shutdown deadline.

Keep substantial provider implementations in their runtime packages. The
executable should primarily construct and order them:

```go
bootstrapper := boot.NewDefaultBootstrapper(
    do.New(),
    config.NewProvider(...),
    database.NewProvider(),
    worker.NewProvider(),
    server.NewProvider(),
)
```

Install shutdown before calling `Register` so partial startup is cleaned up.
Use a fresh bounded context for shutdown. When application context values should
be retained, detach cancellation first:

```go
shutdownCtx, cancel := context.WithTimeout(
    context.WithoutCancel(ctx),
    shutdownTimeout,
)
defer cancel()
```

Do not pass the already-canceled application context directly to graceful
shutdown; cleanup may abort immediately.

Preserve startup and cleanup failures when both matter, for example with
`errors.Join`.

See [`references/boot-example.md`](references/boot-example.md) for a complete
multi-package example.

## Testing

Test each provider directly with a real `boot.Container` backed by a fresh
injector:

```go
injector := do.New()
container := boot.NewContainer(injector)
provider := NewProvider()

err := provider.Register(ctx, container)
```

Cover behavior owned by the provider:

- registrations produced by `Register`;
- readiness work and failures in `Boot`;
- safe shutdown before registration or after partial initialization;
- resource closure and returned cleanup errors;
- context cancellation for background work;
- unexpected post-start failure reporting when the provider owns a long-running
  process.

Avoid real listeners or external infrastructure in unit tests. Use fakes and keep
integration tests separate.

## Verification

Before considering a boot change complete:

- run `gofmt` on changed Go files;
- compile the affected packages;
- run affected provider tests;
- run `go test ./...` when practical;
- verify new providers follow `Register -> Boot -> Shutdown`;
- verify shutdown after partial `Register` or `Boot` failure;
- verify provider declaration order matches dependency and reverse teardown needs;
- for long-running providers, verify `Boot` returns promptly and `Shutdown`
  terminates owned goroutines or resources;
- do not claim the change works if required commands were not actually run.

## Review Checklist

- `pkg/boot` is used only for application-scoped lifecycle concerns.
- The provider contains lifecycle orchestration, not domain/use-case logic.
- The provider lives with, or delegates registration to, the runtime package it
  owns.
- A resource has exactly one shutdown owner.
- Provider order matches registration, readiness, and reverse teardown needs.
- `Register` does not start long-running work.
- `Boot` returns promptly and reports immediate startup failures.
- Unexpected post-start failures are surfaced through logging or supervision.
- `Shutdown` is safe after partial startup and respects its context.
- Startup failure triggers cleanup when resources may have been acquired.
- Normal shutdown uses a fresh bounded context, not the canceled application
  context.
- Failed lifecycle phases are not retried on the same bootstrapper.
- Lifecycle errors are wrapped and preserved.
- Verification commands were actually executed when the environment permits it.
