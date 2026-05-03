# go-stack

**go-stack** is a scaffold template for Go projects designed to help you kickstart production-ready applications with confidence. It follows Go best practices, enforces clean architecture, and provides a well-organized structure that is easy to extend and maintain.

This template is built with the following principles in mind:
- **Best Practices** — idiomatic Go, standard project layout, and consistent conventions.
- **Clean Architecture** — clear separation of concerns with modular, testable layers.
- **AI Agentic Ready** — structured, documented, and predictable code that automated agents and LLMs can understand and extend with ease.
- **Easy to Extend** — pluggable packages, dependency injection, and minimal coupling so new features fit in naturally.

## Installation

Install the CLI globally using `go install`:

```bash
go install github.com/emilioforrer/go-stack/cmd/go-stack@latest
```

## CLI

The `go-stack` CLI helps you scaffold new projects from this template in seconds.

### Commands

| Command | Description | Example |
|---|---|---|
| `new <path>` | Create a new project from the template at the given path. | `go-stack new ./my-app` |

### Flags for `new`

| Flag | Description | Default |
|---|---|---|
| `--name` | Project name (used for Sonar properties). | inferred from path |
| `--module` | Go module name for the generated project. | inferred from path |

## Project Structure

Running `go-stack new ./my-app` generates a scaffold with a purpose-built layout optimized for the `boot` + `httpsvr` ecosystem:

```
my-app/
├── cmd/
│   └── app/                  # CLI entry point (Cobra commands)
│       ├── main.go          # Application entry point
│       ├── root.go          # Root command definition
│       ├── serve.go         # HTTP server command
│       ├── version.go       # Version command
│       ├── completion.go    # Shell completion command
│       └── exit_code.go     # Exit code utilities
├── boot/
│   └── provider/            # Boot providers (server, DB, cache, queue, etc.)
│       └── server.go        # HTTP server provider (includes Huma + OpenAPI)
├── devops/
│   └── security/            # Security scan results, SBOM, Docker Compose, etc.
├── .agents/
│   └── skills/              # Codex skills (45+ Go best-practice files)
├── .agents/
│   └── skills/              # Codex skills (45+ Go best-practice files)
├── .claude/
│   └── skills/              # Claude Code skills (45+ Go best-practice files)
├── .github/
│   └── skills/              # GitHub Copilot skills (45+ Go best-practice files)
├── .opencode/
│   └── skills/              # OpenCode agent skills (45+ Go best-practice files)
├── .air.toml                # Live reload configuration
├── .golangci.yml            # Linter configuration
├── .goreleaser.yml          # Release automation
├── .mise.toml               # Development environment tooling
├── Taskfile.yml             # Task runner configuration
├── apm.yml                  # APM (Agent Package Manager) configuration
├── apm.lock.yaml            # APM lock file
└── go.mod                   # Module definition
```

### AI Agent Package Manager (APM)

This template is **AI agentic ready**. It ships with contextual skill files for three major agent platforms:

| Directory | Agent |
|---|---|
| `.agents/skills/` | AI Agents (Codex) |
| `.claude/skills/` | Claude Code |
| `.github/skills/` | GitHub Copilot |
| `.opencode/skills/` | OpenCode |

Each directory contains **45+ skill files** covering the entire Go ecosystem — concurrency, error handling, testing, security, database patterns, design patterns, Cobra, Viper, samber/do, samber/lo, gRPC, GraphQL, and more. When an AI agent works inside the repository, it automatically loads these skills, ensuring it follows the project's best practices and generates idiomatic, well-structured code.

## Packages

A collection of reusable Go packages for building well-structured applications.

Each package lives under `pkg/` as an independent Go module that can be imported individually.

| Package | Description | Install |
|---|---|---|
| [boot](pkg/boot/README.md) | Application bootstrapping and lifecycle management built on top of [samber/do](https://github.com/samber/do) dependency injection. | `go get github.com/emilioforrer/go-stack/pkg/boot` |
| [httpsvr](pkg/httpsvr/README.md) | HTTP server package built on `net/http` with structured router, configurable server, middleware chaining, and JSON error responses. | `go get github.com/emilioforrer/go-stack/pkg/httpsvr` |

### boot — Lifecycle & Dependency Injection

`boot` is the heart of the template. It manages application startup and teardown through a strict, deterministic lifecycle powered by [samber/do](https://github.com/samber/do).

#### Core Lifecycle

1. **Register** — each provider binds its services into the shared DI container.
2. **Boot** — each provider resolves dependencies and initializes state. **Boot should NOT be a long blocking operation.** If a provider needs to do long-running work (e.g., start a background worker), launch a goroutine inside `Boot` and use a channel or `sync.WaitGroup` to signal readiness.
3. **Shutdown** — each provider tears down gracefully in **reverse** registration order.

All phases are **idempotent**; calling them multiple times is a safe no-op.

#### Key Types

| Type | Role |
|---|---|
| `Container` | Wraps `do.Injector`; the shared service locator. |
| `Provider` | Interface with `Register`, `Boot`, and `Shutdown` hooks. |
| `DefaultProvider` | Embeddable no-op base — override only what you need. |
| `DefaultBootstrapper` | Orchestrates the full lifecycle across all providers. |

#### Creating a Custom Provider

Implement the `Provider` interface (or embed `DefaultProvider`) and register services via `do.Provide`:

```go
type DatabaseProvider struct {
    boot.DefaultProvider
}

func (p *DatabaseProvider) Register(_ context.Context, c boot.Container) error {
    do.Provide(c.Injector(), func(i do.Injector) (*sql.DB, error) {
        return sql.Open("postgres", dsn)
    })
    return nil
}

func (p *DatabaseProvider) Boot(_ context.Context, c boot.Container) error {
    db, err := do.Invoke[*sql.DB](c.Injector())
    if err != nil {
        return err
    }
    return db.Ping()
}

func (p *DatabaseProvider) Shutdown(_ context.Context, c boot.Container) error {
    db, _ := do.Invoke[*sql.DB](c.Injector())
    return db.Close()
}
```

Then wire it into the bootstrapper in `main`:

```go
b := boot.NewDefaultBootstrapper(do.New(), &DatabaseProvider{}, &httpsvr.Provider{})
_ = b.Register(ctx)
_ = b.Boot(ctx)
// ... application runs ...
_ = b.Shutdown(ctx)
```

Because providers are registered in order, you can safely depend on earlier providers during `Boot`. Because `Shutdown` runs in reverse, services that depend on the database will be stopped before the database itself is closed.

### httpsvr — Production-Ready HTTP Server

`httpsvr` provides a thin, opinionated layer over `net/http` that includes routing, middleware chaining, graceful shutdown, and standardized JSON error responses.

#### Key Types

| Type | Role |
|---|---|
| `DefaultRouter` | `http.ServeMux`-backed router with per-route middleware. |
| `DefaultHTTPServer` | Standard `net/http.Server` with configurable timeouts. |
| `DefaultApp` | Full lifecycle app with OS signal handling and graceful shutdown. |
| `Middleware` | `func(http.Handler) http.Handler` — composable by design. |

#### Built-in Middleware

| Middleware | Purpose |
|---|---|
| `RequestLoggerMiddleware` | Rails-style request logging with status and duration. |
| `RecoveryMiddleware` | Recovers from panics and returns 500. |
| `TracingMiddleware` | Injects a trace ID into the request context. |
| `NotFoundMiddleware` | JSON 404 responses for unmatched routes. |
| `InternalServerErrorMiddleware` | JSON 5xx responses. |
| `ClientErrorMiddleware` | JSON 4xx responses (excluding 404). |

You can mount the router as a `boot.Provider` so the server starts and stops automatically with the rest of your application lifecycle.

### Optional Huma Integration

The scaffold comes with [Huma](https://huma.rocks/) wired into the HTTP server provider (`boot/provider/server.go`). Huma provides automatic OpenAPI 3.x generation, request/response validation, and structured JSON error handling — all on top of the standard library `net/http`.

> **Huma is completely optional.** If you prefer a different API framework or plain `net/http`, you can remove the Huma adapter and its dependency from `go.mod` without affecting the rest of the `boot` or `httpsvr` packages.

## Extending the Project

To add a new feature (e.g., a cache, a message queue, or a third-party client):

1. **Create a provider** in a new package or inside `internal/`.
2. **Implement `Register`** to bind the service into the `do.Injector`.
3. **Implement `Boot`** to validate configuration or warm up connections.
4. **Implement `Shutdown`** to release resources cleanly.
5. **Add the provider** to `NewDefaultBootstrapper(...)` in `cmd/<app>/main.go`.

Because every provider shares the same `Container`, cross-service dependencies are resolved automatically by `samber/do`. This keeps `main` small, makes unit testing trivial, and guarantees a deterministic startup/shutdown sequence.

## DevOps, Security & Code Quality

The template is **DevOps-ready** out of the box. It ships with local infrastructure and security scanning baked into the scaffold:

| Capability | What it does |
|---|---|
| Dockerized SonarQube | A ready-to-run `docker-compose.yml` under `devops/security/` spins up a local SonarQube server for continuous code quality inspection. |
| SonarQube Scanner | Configured to scan the project against the local (or remote) SonarQube instance and publish reports. |
| Trivy | Lightweight vulnerability scanner for container images, filesystem, and repositories. |
| Grype | Anchore-powered vulnerability scanner focused on SBOM (Software Bill of Materials) analysis. |
| Snyk | Developer-first security scanner that finds and fixes vulnerabilities in dependencies and containers. |
| SonarQube Issue Extraction CLI | A dedicated CLI tool included in `devops/security/` to fetch and export SonarQube issues for reporting or further processing. |

## Tools

| Tool | Description |
|---|---|
| [golangci-lint](https://github.com/golangci/golangci-lint) | Fast Go linters runner. |
| [Task](https://taskfile.dev) | Task runner used to manage all project commands via `Taskfile.yml`. |

### Available Task Commands

The template uses [Task](https://taskfile.dev) as its command runner. After scaffolding, run any of the following from the project root:

```bash
task test              # Run tests
task test:coverage     # Run tests with coverage report
task test:report       # Run tests and output JSON report
task linter            # Run golangci-lint
task govet             # Run go vet
task ci                # Run the full CI pipeline (linter, vet, test, coverage, release)
task run:local:serve   # Run the application locally with the serve command
task release           # Build and release with GoReleaser
task release:snapshot  # Build a snapshot release (no publish)
task sonarqube:start   # Start the local SonarQube server
task sonarqube:stop    # Stop the local SonarQube server
task sonarqube:scan    # Run the SonarQube scanner
task security:trivy     # Scan with Trivy
task security:grype     # Scan with Grype
task security:snyk      # Scan with Snyk
task security:scan      # Run all security scans (Trivy, Grype, Snyk)
task mod:rename         # Rename the Go module
task mod:replace:add    # Add local replace directives for go-stack packages
task mod:replace:drop   # Remove local replace directives
```

## Roadmap

- [ ] **Bun ORM integration** — Add [Bun](https://bun.uptrace.dev/) as an optional `boot` provider and standalone `pkg`, including a CLI command to run migrations. Provide a `testcontainers` helper for bootstrapping integration tests with a real database.
- [ ] **`pkg/health` package** — Introduce a health-check provider and HTTP endpoints (`/health`, `/ready`) that aggregate the status of all registered providers.
- [ ] **Reusable middleware package** — Add new middlewares into a dedicated `pkg/middlewares` library so they can be reused with any `net/http` compatible router.
- [ ] **Docker & container registry integration** — Add a `Dockerfile`, `docker-compose.yml`, and Task commands to build, tag, and push the application image to a container registry.
- [ ] **Documentation improvements** — Expand documentation with practical guides on:
  - How to leverage the dependency injection system (samber/do) for wiring services, testing, and mocking.
  - How to structure features using **clean architecture vertical slices** — organizing code by feature rather than by layer, keeping handlers, services, and repositories colocated.
  - A proposed reference architecture for implementing new features (e.g., user management, payments) that demonstrates the full lifecycle from provider registration to HTTP endpoint.
