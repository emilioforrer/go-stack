# Library / SDK Profile

Use this profile when the main deliverable is a reusable Go package consumed by
other applications.

The universal rules in `../SKILL.md` remain authoritative.

## Primary Rule

The public Go API is the most important architectural boundary.

Optimize for:

- small public surface;
- stable semantics;
- idiomatic construction;
- compatibility;
- minimal dependencies;
- hidden implementation details.

Do not reproduce application architecture inside a library without a concrete
need.

## Typical Structure

```text
mylib/
├── client.go
├── options.go
├── errors.go
├── transport.go              # only if public
├── internal/
│   ├── protocol/
│   ├── retry/
│   └── transport/
└── examples/
```

Package structure SHOULD follow cohesive concepts, not mandatory layers.

## Public API

Export only what consumers need.

Prefer constructors and functional options:

```go
client, err := sdk.New(
    sdk.WithEndpoint(endpoint),
    sdk.WithHTTPClient(httpClient),
)
```

Avoid requiring callers to understand internal configuration packages,
registries, service locators, or DI containers.

## Dependency Injection

Constructors are the normal dependency-injection mechanism for libraries.

Prefer:

```go
func New(client *http.Client, opts ...Option) (*Client, error)
```

Do not expose `samber/do`, Wire, Fx, Dig, or another DI container as part of the
library contract unless integrating with that container is the library's actual
purpose.

## Interfaces

Do not export interfaces merely to make the library appear extensible.

Prefer:

- concrete exported types for primary library objects;
- consumer-owned interfaces in downstream applications;
- small public interfaces only when implementing them is an intentional extension
  point.

Accept standard-library abstractions where useful, such as `io.Reader`,
`io.Writer`, `http.RoundTripper`, `fs.FS`, or `context.Context`.

## Configuration

Small library configuration belongs next to the package that owns it:

```go
type Options struct {
    Timeout time.Duration
}
```

Do not create a separate `config` package unless it is independently useful or
prevents a real dependency problem.

Do not load environment variables or config files implicitly unless that is the
library's explicit purpose. Let applications own runtime configuration loading.

## Internal Packages

Use `internal/` to protect implementation details that callers should not depend
on.

Do not move types to public packages solely to make internal implementation
convenient.

## External Dependencies

Avoid wrapping dependencies solely for Clean Architecture ceremony.

Wrap or isolate a dependency when doing so protects:

- public API stability;
- portability;
- testing of meaningful behavior;
- replacement of a volatile implementation;
- ownership of protocol mapping.

## Errors

Public errors are part of the compatibility contract. Keep them intentional and
stable. Do not expose dependency-specific error types unless that is a deliberate
part of the API.

## Compatibility

Treat changes to exported identifiers, interfaces, behavior, and error semantics
as API design decisions.

Prefer additive evolution where practical. Keep internal refactors invisible to
consumers.

## Review Checklist

- [ ] Every exported identifier is useful to consumers.
- [ ] Construction is idiomatic and does not require an application framework.
- [ ] No application-only layers were created without a concrete need.
- [ ] Configuration loading remains the caller's responsibility unless explicitly in scope.
- [ ] Internal dependencies do not leak into the public API accidentally.
- [ ] Exported interfaces are intentional extension contracts.
- [ ] Public errors and behavior are treated as compatibility concerns.
