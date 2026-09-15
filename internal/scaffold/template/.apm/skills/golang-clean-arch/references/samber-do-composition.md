# samber/do Composition

Apply this reference only when an application uses `github.com/samber/do/v2`.
Libraries and SDKs SHOULD normally remain container-independent.

The universal rule is that DI belongs at composition boundaries. Constructors
and product behavior MUST NOT depend on `do.Injector`.

## Package Registration

A runtime package MAY expose dependency registrations through `package.go` when
that convention improves composition.

Configurable package example:

```go
func Package(cfg Config) func(do.Injector) {
    return do.Package(
        do.Eager(cfg),
        do.Lazy(provideService),
    )
}

func provideService(i do.Injector) (*Service, error) {
    return NewService(
        do.MustInvoke[Client](i),
        do.MustInvoke[Config](i),
    ), nil
}

func NewService(client Client, cfg Config) *Service {
    return &Service{client: client, cfg: cfg}
}
```

The normal constructor remains reusable without the container.

## Rules

- Use `do.Eager` for externally constructed values such as validated config.
- Prefer `do.Lazy` for runtime services unless eager startup is intentional.
- Use `do.Bind` when exposing a concrete implementation through a consumed interface.
- Providers may resolve dependencies from the injector at composition time.
- Constructors and normal runtime methods MUST NOT receive `do.Injector`.
- Package registration MUST preserve the dependency direction selected for the project archetype.
- Do not use the container as a global service locator.
- Do not resolve ordinary dependencies from the container on request/hot paths.

## Business Applications

A business feature root MAY compose registrations exported by its children:

```go
func Package(cfg featureconfig.Config) func(do.Injector) {
    return do.Package(
        postgres.Package(cfg.Postgres),
        usecase.Package,
        httpapi.Package(cfg.HTTP),
    )
}
```

This composition MUST NOT bypass feature-isolation rules.

## Technical Applications

Technical applications may compose packages by capability rather than business
feature:

```text
app
├── gateway
├── protocol
├── provider
└── server
```

DI composition may know all concrete implementations; the runtime capability
packages should not.

## Libraries

Reusable libraries SHOULD NOT require `samber/do` from their callers. If a
repository contains both a reusable library and an application, keep container
integration in the application or in an optional adapter package outside the
library's core API.
