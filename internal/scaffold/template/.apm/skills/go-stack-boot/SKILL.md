---
name: go-stack-boot
description: >
  Use go-stack's pkg/boot package as the thin application composition and
  lifecycle entry point for configuring and registering packages, resolving
  application dependencies with samber/do, starting long-running services,
  supervising critical runtime failures, and performing graceful shutdown.
  Apply when adding or reviewing boot providers, executable wiring, runtime
  services, startup ordering, or teardown behavior.
metadata:
  author: go-stack
  version: "1.2.0"
---

# Go Stack Boot

## Purpose

Use `github.com/emilioforrer/go-stack/pkg/boot` at an executable's composition
boundary when application-scoped packages and services need shared dependency
injection and a deterministic lifecycle.

`pkg/boot` is the application's **thin entry point for composition and
lifecycle**, not a place for business or technical behavior.

Typical flow:

```text
Configure
   ↓
Register packages/dependencies
   ↓
Boot runtime services
   ↓
Run + supervise
   ↓
Shutdown in reverse order
```

Lifecycle:

1. `Register` providers in declaration order.
2. `Boot` providers in declaration order after registration succeeds.
3. `Shutdown` providers in reverse declaration order.

## Scope

Use boot for executable applications such as APIs, gateways, workers, CLIs with
long-running services, consumers, schedulers, and technical daemons.

Do not expose `boot.Container`, `boot.Provider`, or `do.Injector` as part of a
library/SDK public API. Libraries normally use constructors and explicit
resource ownership instead.

## Terminology

- `boot.Provider`: thin application lifecycle adapter with `Register`, `Boot`,
  and `Shutdown`.
- `do.Provider[T]`: `samber/do` constructor for a dependency.
- package/service: the real implementation containing behavior.

A `boot.Provider` orchestrates a package/service; it should not become the
implementation itself.

## Thin Provider Rule

Providers SHOULD contain only composition/lifecycle logic:

```text
Register → resolve/configure/register dependencies
Boot     → activate/start/verify readiness
Shutdown → stop/wait/close owned resources
```

Keep HTTP handlers, processing loops, business rules, protocol logic, storage
logic, retries, and other real behavior in their owning packages/services.

Prefer:

```go
func (p *Provider) Register(_ context.Context, c boot.Container) error {
    cfg := do.MustInvoke[*config.Config](c.Injector())
    db := do.MustInvoke[*database.Client](c.Injector())

    p.processor = worker.New(db, cfg.WorkerInterval)
    do.ProvideValue(c.Injector(), p.processor)
    return nil
}

func (p *Provider) Boot(ctx context.Context, _ boot.Container) error {
    return p.processor.Start(ctx)
}

func (p *Provider) Shutdown(ctx context.Context, _ boot.Container) error {
    return p.processor.Shutdown(ctx)
}
```

Do not put the worker loop itself in the provider.

## Dependency Injection

The boot layer is allowed to access the application's dependency graph.
Providers may resolve any dependency already registered in the shared
`do.Injector` when composing their package/service.

This makes boot the integration point between packages without requiring
constructors to manually thread every dependency through `main.go`.

However, do not turn DI into a runtime service locator:

- providers/package registration may use `boot.Container` / `do.Injector`;
- constructors receive concrete dependencies;
- normal handlers, services, domain/application code, and background loops
  should not retain or query the injector during normal execution.

Use DI for **composition**, not hidden runtime dependency lookup.

## When to Create a Provider

Create a provider when an application-scoped package/service:

- contributes dependencies to the application injector;
- requires application configuration during assembly;
- must initialize or verify a resource before readiness;
- starts an HTTP server, consumer, scheduler, worker, or other long-running
  service;
- owns a resource that must be stopped or closed;
- has startup/shutdown ordering constraints.

Do not create a provider for:

- pure domain/application logic;
- stateless helpers;
- request-scoped work;
- short-lived operation-scoped resources;
- a library that does not participate in the executable lifecycle.

For simple static values, direct DI registration at the composition root is
fine; not every dependency needs its own provider.

## Lifecycle Model

```text
Constructor → construct
Register    → wire
Boot        → activate + establish readiness
Running     → supervise critical runtime failures
Shutdown    → deactivate + release
```

### Register

Use `Register` to:

- validate configuration required for assembly;
- resolve dependencies registered by earlier providers;
- construct thin package/service objects;
- register application-scoped dependencies.

`Register` MUST NOT start long-running goroutines, listeners, consumers, or
other runtime activity.

DI constructors should construct objects, not secretly start services or
acquire long-lived runtime resources. Activation belongs in `Boot`.

### Boot

Use `Boot` to:

- establish connections;
- verify readiness;
- run required startup work;
- start long-running services.

`Boot` receives the **application lifetime context**. Do not pass a short-lived
startup timeout as the parent context for long-running services.

Use operation-specific child contexts for bounded startup checks:

```go
checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()

if err := client.Ping(checkCtx); err != nil {
    return err
}
```

Long-running work must not block `Boot`; `Boot` should return once the component
is ready.

### Runtime Supervision

`DefaultBootstrapper` orchestrates startup/shutdown; it does not automatically
supervise failures occurring after `Boot` returns.

Classify runtime services:

```text
Non-critical failure → log/recover according to service policy
Critical failure     → report to application supervisor
                       → cancel/terminate application
                       → graceful Shutdown
```

A critical HTTP listener, consumer, gateway loop, or required worker must not
silently die while the process remains "healthy".

### Shutdown

`Shutdown` stops/closes only resources owned by that provider/service.

It MUST:

- tolerate partial initialization;
- return `nil` when there is nothing to clean up;
- honor the supplied context;
- wait for owned goroutines/resources when necessary;
- return cleanup errors rather than swallowing them.

Use exactly one lifecycle owner per resource. Do not independently close the
same resource through both boot and `samber/do` shutdown mechanisms.

## Non-Negotiable Rules

- Declare providers in lifecycle dependency order.
- Keep `Register`, `Boot`, and `Shutdown` sequential for one bootstrapper.
- Call `Register` before `Boot`.
- Arrange cleanup before startup because registration/boot may partially
  initialize resources before returning an error.
- `Shutdown` must be safe after partial `Register` or `Boot`.
- After failed `Register` or `Boot`, shut down and create a fresh bootstrapper;
  do not retry the failed phase on the same instance.
- Preserve startup and shutdown failures when both matter, e.g. `errors.Join`.

## Provider Placement

Keep providers close to the package/service they compose:

```text
internal/database/provider.go
internal/worker/provider.go
internal/server/provider.go
```

Keep actual behavior beside them in ordinary files:

```text
internal/worker/processor.go
internal/server/server.go
internal/server/handler.go
```

Package registration remains owned by the package. A provider may delegate to
an existing `Package(...)` registration function.

## Ordering

Provider order is lifecycle dependency order.

Put A before B when B:

- resolves A during registration;
- requires A to be ready before B boots; or
- must stop before A closes.

Example:

```text
Configuration
    ↓
Database
    ↓
Background Processor
    ↓
HTTP Server
```

Shutdown is reversed.

Do not use ordering to hide undeclared architectural dependencies. Preserve the
dependency direction defined by the project's architecture profile.

## Compose the Executable

The executable/composition root owns:

- the `do.Injector`;
- top-level configuration values;
- provider construction and order;
- the application lifetime context / OS signals;
- fatal runtime supervision;
- startup error propagation;
- shutdown deadline.

Keep it thin:

```go
injector := do.New()

do.ProvideValue(injector, cfg)
do.ProvideValue(injector, supervisor)

bootstrapper := boot.NewDefaultBootstrapper(
    injector,
    database.NewProvider(),
    worker.NewProvider(),
    server.NewProvider(),
)
```

The executable should primarily configure, register, boot, supervise, and shut
down packages/services—not implement their behavior.

Install shutdown handling before `Register` so partial startup is cleaned up.

Use a fresh bounded context for shutdown:

```go
shutdownCtx, cancel := context.WithTimeout(
    context.WithoutCancel(ctx),
    shutdownTimeout,
)
defer cancel()
```

Do not use the already-canceled application context directly for graceful
cleanup.

See [`references/boot-example.md`](references/boot-example.md).

## Testing

Test providers as thin lifecycle adapters with a fresh injector/container.

Verify:

- expected dependencies are registered;
- dependency resolution errors are surfaced;
- `Boot` delegates startup and reports readiness failures;
- `Shutdown` delegates cleanup and tolerates partial startup;
- context cancellation reaches long-running services;
- critical post-start failures are reported to supervision;
- provider order and reverse shutdown are correct.

Test real package/service behavior in that package's own tests. Avoid duplicating
business/technical behavior tests through the provider.

## Verification

Before considering a boot change complete:

- run `gofmt` on changed Go files;
- compile affected packages;
- run affected tests;
- run `go test ./...` when practical;
- verify `Register → Boot → Shutdown` behavior;
- verify cleanup after partial startup failure;
- verify long-running `Boot` methods return promptly;
- verify critical runtime failures reach supervision;
- verify shutdown uses a fresh bounded context;
- do not claim commands pass unless they were actually run.

## Review Checklist

- Boot is used only at the executable/application composition boundary.
- Providers are thin lifecycle/composition adapters.
- Real behavior lives in packages/services, not providers.
- Packages can resolve app dependencies through DI during composition without
  retaining the injector as a runtime service locator.
- Constructors construct; `Register` wires; `Boot` activates.
- The application lifetime context is used for long-running services.
- Critical post-start failures are supervised.
- Every resource has exactly one lifecycle owner.
- Provider order matches dependency/readiness/reverse teardown needs.
- `Register` does not start long-running work.
- `Boot` returns after readiness rather than blocking for service lifetime.
- `Shutdown` is safe after partial startup and respects its context.
- Startup failure triggers cleanup.
- Shutdown uses a fresh bounded context.
- Failed phases are not retried on the same bootstrapper.
- Lifecycle errors are preserved.
