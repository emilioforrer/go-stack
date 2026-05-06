---
description: Architecture standards — Clean Architecture + DDD + Vertical Slice with explicit Hexagonal (Ports & Adapters) boundaries
applyTo: '**/*.go'
---

# Architecture Standards

This repository follows **Clean Architecture + DDD + Vertical Slice** with explicit **Hexagonal Architecture** boundaries: **Inbound (IN) adapters** and **Outbound (OUT) adapters**.

The goal is to keep business logic independent from frameworks (HTTP, gRPC, CLI), persistence (Bun/Postgres), and external systems (Stripe, Jira, SonarQube, email, etc.), while still allowing features to be developed as self-contained vertical slices that can grow, be replaced, or be deleted independently.

This document is the **single source of truth** for how Go code is organized in this project. It synthesizes:

- Universal Go project layout (`cmd/`, `internal/`, `pkg/`).
- DDD bounded contexts (vertical slices grouped by business capability).
- Clean Architecture's Dependency Rule (dependencies point inward).
- Hexagonal Architecture (primary/inbound and secondary/outbound ports & adapters).
- Strict separation between **Use Cases**, **Application Services**, and **Presenters**.

---

## 1. Core Principles

### 1.1 The Dependency Rule

Dependencies always point **inward**:

```
adapters/in   →   application   →   domain
adapters/out  →   application   →   domain
```

- Inner layers **MUST NEVER** import outer layers.
- `domain/` has zero dependencies on frameworks, persistence, transport, or serialization.
- `application/` depends on `domain/` only — never on Bun, HTTP, gRPC, or any infrastructure package.
- `adapters/in/*` and `adapters/out/*` depend on `application/` and `domain/`, but never on each other.

### 1.2 Features Are Independent Vertical Slices

Each feature under `internal/features/<feature>/` is a "mini application" with its own domain model, use cases, and adapters. It maps to a **DDD bounded context**.

**Features MUST NOT import other features.** A feature may only import:

- Its own packages (`domain`, `application`, `adapters`).
- `internal/shared/*` (shared kernel, integrations, cross-cutting utilities).

A feature **MUST NOT** import:

- Another feature's `domain`, `application`, or `adapters` packages.
- Another feature's Bun models, repositories, or gateways.

If two features need to communicate, do it through:

1. A dedicated feature owning the lifecycle (e.g., a `projects` feature exposing IN ports).
2. Per-feature read models (projections) materialized via events or scheduled jobs.
3. Domain events with anti-corruption translation in the consuming feature.

### 1.3 Ports and Adapters (Hexagonal)

- **IN ports** (primary/driving) define what the application **exposes** (use-case-level API).
- **OUT ports** (secondary/driven) define what the application **needs** (DB, gateways, messaging).
- **IN adapters** (HTTP, gRPC, CLI, consumers) **call** IN ports.
- **OUT adapters** (Bun repositories, external clients) **implement** OUT ports.

> **Key distinction**
>
> - IN adapters **call the application**.
> - OUT adapters **are called by the application**.

Interfaces live where they are **consumed**, not where they are implemented:

- IN port interfaces live in `application/ports/in/` (the application defines what it offers to drivers).
- OUT port interfaces live in `application/ports/out/` (the application defines what it needs from infrastructure).

### 1.4 ORM (Bun) Isolation

Bun structs are **persistence models**, not domain models.

- Bun-tagged structs live **only** in `adapters/out/postgres/models/`.
- Domain entities and value objects contain **no ORM tags** (`bun`, `gorm`, `db`, etc.).
- The application layer **never** imports Bun.
- A `mapper.go` translates between domain and Bun models inside the OUT adapter.

### 1.5 Serialization Isolation

- `json`, `xml`, `protobuf`, `bson`, and any other serialization tags are **forbidden** in `domain/` and `application/`.
- Use-case Outputs returned by Use Cases are **plain Go structs**.
- Serialization tags exist **only** in `adapters/in/*/view_models.go` and Request structs.

---

## 2. Repository Structure

```
cmd/
  <binary>/                 # one subdirectory per main package — minimal logic
    main.go
internal/
  features/                 # vertical slices (one per bounded context)
    <feature>/
      domain/
      application/
      adapters/
  shared/                   # shared kernel + integrations + cross-cutting utilities
    domain/
    application/
    adapters/
      in/                     # reusable inbound adapter building blocks
      out/                    # reusable outbound adapter building blocks
        integrations/           # raw technical clients to external SaaS
          <external-system>/
pkg/                        # OPTIONAL — only if exporting reusable libraries
go.mod
go.sum
```

### 2.1 `cmd/`

- One subdirectory per binary (`api`, `worker`, `migrate`, `cli`, ...).
- `main.go` parses flags, builds the DI container, calls `Run()`. **No business logic.**

### 2.2 `internal/shared/`

```
internal/shared/
  kernel/                   # tiny domain primitives reusable across features (Money, IDs, Time)
  observability/            # slog setup, metrics, tracing
  adapters/
    in/                       # reusable inbound adapter building blocks
      http/                     # shared HTTP middleware, error mappers, response helpers
      grpc/                     # shared gRPC interceptors, status mappers
      cli/                      # shared CLI scaffolding (root command, version, global flags)
    out/                      # reusable outbound adapter building blocks
      postgres/                 # Bun connection/pool setup, Tx helpers, base repository
      cache/                    # shared cache client setup
      messaging/                # shared publisher/consumer plumbing
      integrations/             # raw technical clients to external SaaS
        stripe/
        jira/
        sonarqube/
        email/
```

`shared/adapters/in/<transport>/` contains **reusable inbound building blocks only** — middleware, interceptors, error-to-status mappers, router/server construction helpers, CLI bootstrap:

- **No feature-specific handlers or routes.** Those live under `internal/features/<feature>/adapters/in/`.
- **No business logic.**
- **No knowledge of any specific use case.**

`shared/adapters/out/<technology>/` contains **reusable outbound building blocks only** — DB connection/pool setup, transaction helpers, base repository scaffolding, shared cache/messaging client wiring:

- **No feature-specific repositories, Bun models, or gateways.** Those live under `internal/features/<feature>/adapters/out/`.
- **No business logic.**

`shared/adapters/out/integrations/<system>/` contains **raw technical clients only**:

- Authentication, retries, pagination, rate limiting.
- Wire-format structs for the external API.
- **No business logic.**
- **No feature-specific naming.** A Stripe client does not know about "checkout" or "subscriptions".

`internal/shared/kernel/` is the **shared kernel** in DDD terms: a deliberately small set of value objects used in many features (`Money`, `Email`, `UserID`). Add to it conservatively.

### 2.4 `internal/features/<feature>/`

```
internal/features/<feature>/
  domain/
    <entity>.go             # entities and aggregate roots
    <value_object>.go       # value objects
    rules.go                # invariants and domain services
    errors.go               # domain errors (sentinels)
    events.go               # domain events (if used)

  application/
    service/
      service.go            # ApplicationService — implements IN ports, orchestrates use cases
    ports/
      in/
        <use_case>.go       # IN port interface(s) + Input type
      out/
        <repository>.go     # OUT port interface (e.g., UserRepository)
        <gateway>.go        # OUT port interface (e.g., Notifier, PaymentGateway)
    usecases/
      <use_case>/
        usecase.go       # Use Case (core use-case logic) — returns plain result
        validator.go        # use-case validation rules
        types.go            # Input, Output — plain structs, NO json tags
        presenter.go        # OPTIONAL — presenter interface for documentation only

  adapters/
    in/
      http/
        routes.go
        <use_case>_handler.go    # parses request, calls IN port, calls presenter
        <use_case>_presenter.go  # use-case Output → ViewModel (json tags allowed HERE)
        view_models.go           # ViewModel structs with json tags
        requests.go              # Request structs with json tags
      grpc/                      # OPTIONAL alternative driver
      cli/                       # OPTIONAL alternative driver
    out/
      postgres/
        models/
          <entity>_model.go      # Bun model (persistence-only — bun tags HERE)
        mapper.go                # domain ↔ Bun mapping
        <entity>_repo.go         # implements OUT repository port
      external/
        <gateway>.go             # implements OUT gateway port (wraps shared/integrations/*)
```

---

## 3. Layer Responsibilities

### 3.1 Domain (`domain/`)

- Entities, aggregates, value objects, invariants, domain services, domain events.
- **MUST** be framework-agnostic.
- **MUST NOT** know about persistence, serialization, transport, or external systems.
- **MUST NOT** import `adapters/`, `application/ports/`, Bun, HTTP, or any third-party infrastructure.

All mutations to an aggregate go through the aggregate root. The root enforces invariants.

### 3.2 Application (`application/`)

The application layer contains four kinds of artifacts:

1. **Use Cases** (`application/usecases/<uc>/usecase.go`) — core business logic for one use case. Returns a plain `Output` struct.
2. **Application Services** (`application/service/service.go`) — implement IN ports, orchestrate one or more Use Cases, manage transactions and cross-use-case coordination.
3. **Ports** (`application/ports/in|out/`) — interfaces.
4. **Validators** (`application/usecases/<uc>/validator.go`) — use-case-level validation (distinct from domain invariants and transport validation).

The application layer **MUST**:

- Be framework-agnostic and transport-agnostic.
- Return plain Go structs.

The application layer **MUST NOT**:

- Import Bun, HTTP, gRPC, JSON encoders, or any concrete adapter.
- Depend on Presenters or ViewModels.
- Contain serialization tags.

### 3.3 Inbound Adapters (`adapters/in/`)

- HTTP / gRPC / CLI handlers, message consumers.
- Parse incoming requests; perform **transport-level validation** (well-formed JSON, required headers, etc.).
- Map requests to application Inputs.
- Call **Application Services** (IN ports).
- Invoke **Presenters** to convert use-case Outputs into ViewModels.
- Encode the response and set HTTP status codes / gRPC status codes.

This is the **only** layer where:

- Serialization tags (`json`, `xml`, `protobuf`) are allowed.
- HTTP status codes and gRPC status codes are decided.
- Presenters are invoked.

### 3.4 Outbound Adapters (`adapters/out/`)

- Bun repositories implementing repository OUT ports.
- External API gateways implementing gateway OUT ports (typically wrapping a `shared/integrations/<system>` raw client).
- Messaging publishers/consumers, cache clients, file storage clients.

OUT adapters **MUST**:

- Implement ports defined in `application/ports/out/`.
- Map between infrastructure shapes (Bun models, API payloads) and domain entities at the adapter boundary.

OUT adapters **MUST NOT**:

- Leak Bun models, API payloads, or HTTP semantics into the application or domain layers.
- Be imported directly by inbound adapters or by other features.

---

## 4. Component Responsibilities and Boundaries

### 4.1 Use Case

**Responsibilities**

- Contains the **core application logic** — the "what" of the business operation.
- Orchestrates domain entities, value objects, and domain services.
- Calls OUT ports (repositories, gateways) to interact with infrastructure.
- Returns a plain **use-case Output** struct and/or domain errors.

**Constraints**

- **MUST** be framework-agnostic and transport-agnostic.
- **MUST NOT** know about HTTP, JSON, databases, ORMs, presenters, or external services.
- **MUST NOT** return ViewModels or structs with `json` tags.
- **MUST NOT** depend on Presenters or Application Services.
- **MUST** return plain Go structs.

**Signature**

```go
func (uc *CreateOrderUseCase) Execute(ctx context.Context, input CreateOrderInput) (CreateOrderOutput, error)
```

### 4.2 Application Service

**Responsibilities**

- Implements **IN ports** (the public API of the feature).
- Orchestrates **one or more Use Cases** when a single request requires multiple use cases.
- Handles **transactions** and cross-cutting coordination across use cases.
- Returns the use-case Output to the inbound adapter.

**Constraints**

- **MUST** delegate business logic to Use Cases (or domain services).
- **MUST** return use-case Outputs (plain Go structs) plus `error`.
- **MUST NOT** depend on Presenters or any transport concern.
- **MUST NOT** contain business logic — that belongs to Use Cases or the Domain.

**Signature**

```go
func (s *OrderApplicationService) CreateOrder(ctx context.Context, input CreateOrderInput) (CreateOrderOutput, error)
```

### 4.3 Presenter

**Responsibilities**

- Transforms a use-case **Output** into a **ViewModel** for a specific delivery mechanism.
- Shapes data for presentation: formatting dates/currencies, renaming fields, aggregating, omitting fields, adding computed fields.
- Applies serialization tags (`json`, `xml`, `protobuf`).

**Constraints**

- **MUST** be called **only by inbound adapters** (HTTP handlers, gRPC handlers, CLI commands).
- **MUST NOT** be called by Use Cases or Application Services.
- **MUST NOT** contain business logic.
- **MUST NOT** access databases or external services.
- **IS** part of the **delivery layer**, not the application core.

**Signature**

```go
func (p *HTTPOrderPresenter) PresentCreateOrder(output CreateOrderOutput) OrderViewModel
func (p *HTTPOrderPresenter) PresentError(err error) ErrorViewModel
```

The presenter **interface** MAY be declared in `application/usecases/<uc>/presenter.go` for documentation, but the application layer never imports it for invocation. Use Cases and Application Services do not receive a Presenter.

### 4.4 Inbound Adapters

**Responsibilities**

- Handle transport and infrastructure concerns (HTTP routing, gRPC service registration, CLI flag parsing, message consumption).
- Parse requests into application **Inputs**.
- Invoke the **Application Service** (IN port).
- Invoke the **Presenter** to build the ViewModel.
- Write the response (status code + serialized ViewModel).

**Constraints**

- **ARE** the only place where `json` tags and HTTP/gRPC status codes are allowed.
- **MUST NOT** contain business logic.
- **MUST** delegate every business operation to an Application Service.

### 4.5 Outbound Adapters

**Responsibilities**

- Implement OUT ports defined by the application layer.
- Translate between infrastructure shapes (Bun models, third-party API payloads) and domain entities.
- Manage transactions when the port requires it (e.g., `Tx` helpers passed via context).

**Constraints**

- **MUST** implement interfaces from `application/ports/out/`.
- **MUST NOT** leak ORM models or third-party SDK types into the application or domain layers.
- **MUST NOT** depend on inbound adapters.

---

## 5. Validators

Validation is split across three layers — each with a clearly scoped responsibility:

| Layer | Location | Responsibility |
| --- | --- | --- |
| **Domain invariants** | `domain/rules.go`, entity constructors | Business rules that must always hold (e.g., "an order has at least one item"). Enforced by the aggregate root. |
| **Use-case validation** | `application/usecases/<uc>/validator.go` | Rules specific to a use case (e.g., "email must be unique at registration time", "password policy"). May call OUT ports for pre-checks. |
| **Transport validation** | `adapters/in/*/<handler>.go` | Well-formed payload (valid JSON, required fields present, basic format). Returns 400-class errors before reaching the application. |

A failure at one layer never bubbles untranslated into another. Domain errors are mapped to HTTP/gRPC status codes only by inbound adapters.

---

## 6. Call Flow

```
HTTP Request
    │
    ▼
┌─────────────────────────────────────────────────┐
│ IN Adapter (adapters/in/http)                   │
│  - parse request (json tags HERE)               │
│  - transport-level validation                   │
│  - map to application Input                     │
└──────────────────┬──────────────────────────────┘
                   │ (1) calls IN port
                   ▼
┌─────────────────────────────────────────────────┐
│ Application Service (application/service)       │
│  - implements IN port                           │
│  - coordinates transactions / multiple uc's     │
│  - MUST NOT call Presenters                     │
└──────────────────┬──────────────────────────────┘
                   │ (2) invokes
                   ▼
┌─────────────────────────────────────────────────┐
│ Use Case (application/usecases/<uc>)          │
│  - calls Validator                              │
│  - orchestrates domain entities                 │
│  - calls OUT ports                              │
│  - returns plain Output (NO json tags)          │
└──────────────────┬──────────────────────────────┘
                   │ (3) calls
                   ▼
┌─────────────────────────────────────────────────┐
│ OUT Ports (application/ports/out)               │
│   ▲ implemented by                              │
│   │                                             │
│ OUT Adapters (adapters/out/postgres, external)  │
│  - Bun repository / external API gateway        │
│  - maps Bun model / API payload ↔ domain entity │
└──────────────────┬──────────────────────────────┘
                   │ (4) result returns up
                   ▼
┌─────────────────────────────────────────────────┐
│ IN Adapter                                      │
│  - receives plain Output                        │
└──────────────────┬──────────────────────────────┘
                   │ (5) invokes
                   ▼
┌─────────────────────────────────────────────────┐
│ Presenter (adapters/in/http)                    │
│  - Output → ViewModel (json tags HERE)          │
│  - formats fields, adds computed fields         │
└──────────────────┬──────────────────────────────┘
                   │ (6) returns ViewModel
                   ▼
┌─────────────────────────────────────────────────┐
│ IN Adapter                                      │
│  - encodes ViewModel + sets status code         │
└──────────────────┬──────────────────────────────┘
                   ▼
HTTP Response
```

---

## 7. Worked Example — `users` feature: Register User + Send Welcome Email

A user-registration feature that, on success, persists the user and triggers a welcome notification (implemented as email) without leaking email concerns into the domain.

### 7.1 Folder structure

```
internal/features/users/
  domain/
    user.go                   # User aggregate root
    email.go                  # Email value object
    rules.go                  # invariants
    errors.go                 # ErrEmailTaken, ErrInvalidPassword

  application/
    service/
      service.go              # UserApplicationService implements IN ports
    ports/
      in/
        register_user.go      # RegisterUser IN port + Input type
      out/
        user_repository.go    # UserRepository OUT port
        notifier.go           # Notifier OUT port (feature-level abstraction)
        password_hasher.go    # PasswordHasher OUT port (optional)
    usecases/
      register_user/
        usecase.go            # core use case — returns RegisterUserOutput
        validator.go          # use-case validation
        types.go              # RegisterUserInput, RegisterUserOutput (plain)
        presenter.go          # OPTIONAL presenter interface (documentation only)

  adapters/
    in/
      http/
        routes.go
        register_user_handler.go     # parses request, calls service, calls presenter
        register_user_presenter.go   # Output → ViewModel (json tags)
        view_models.go               # ViewModel definitions
        requests.go                  # Request structs (json tags)
    out/
      postgres/
        models/
          user_model.go              # Bun model (bun tags ONLY here)
        mapper.go                    # domain ↔ Bun model
        user_repo.go                 # implements UserRepository
      external/
        email_notifier.go            # implements Notifier using shared/integrations/email
```

### 7.2 Use-case sequence

1. HTTP handler decodes the request and runs transport validation.
2. Handler maps the request to `RegisterUserInput` and calls `UserApplicationService.RegisterUser(ctx, input)`.
3. The Application Service delegates to `RegisterUserUseCase.Execute`.
4. The Use Case:
   - Runs the use-case `Validator` (uniqueness, password policy).
   - Hashes the password via the `PasswordHasher` OUT port (optional).
   - Persists the user via `UserRepository.Create`.
   - Triggers `Notifier.SendWelcomeEmail`.
   - Returns a plain `RegisterUserOutput`.
5. The handler invokes the Presenter to build a `UserViewModel` (with `json` tags).
6. The handler writes the response with status `201 Created`.

### 7.3 Code skeletons

**`application/usecases/register_user/types.go`**

```go
package register_user

import "time"

// Input — plain struct, no json tags.
type RegisterUserInput struct {
    Email    string
    Password string
    FullName string
}

// Output — plain struct, no json tags.
type RegisterUserOutput struct {
    UserID    string
    Email     string
    CreatedAt time.Time
}
```

**`application/usecases/register_user/usecase.go`**

```go
package register_user

import (
    "context"

    "myapp/internal/features/users/application/ports/out"
    "myapp/internal/features/users/domain"
)

type RegisterUserUseCase struct {
    users    out.UserRepository
    notifier out.Notifier
    hasher   out.PasswordHasher
}

func (uc *RegisterUserUseCase) Execute(ctx context.Context, input RegisterUserInput) (RegisterUserOutput, error) {
    if err := uc.validate(ctx, input); err != nil {
        return RegisterUserOutput{}, err
    }

    hash, err := uc.hasher.Hash(ctx, input.Password)
    if err != nil {
        return RegisterUserOutput{}, err
    }

    user, err := domain.NewUser(input.Email, input.FullName, hash)
    if err != nil {
        return RegisterUserOutput{}, err
    }

    id, err := uc.users.Create(ctx, user)
    if err != nil {
        return RegisterUserOutput{}, err
    }

    if err := uc.notifier.SendWelcomeEmail(ctx, user.Email(), domain.WelcomeInput{Name: user.FullName()}); err != nil {
        // Decide whether this is fatal or fire-and-forget per business rule.
        return RegisterUserOutput{}, err
    }

    return RegisterUserOutput{
        UserID:    id,
        Email:     user.Email().String(),
        CreatedAt: user.CreatedAt(),
    }, nil
}
```

**`application/service/service.go`**

```go
package service

import (
    "context"

    "myapp/internal/features/users/application/usecases/register_user"
)

type UserApplicationService struct {
    register *register_user.RegisterUserUseCase
}

// Implements IN port RegisterUser.
func (s *UserApplicationService) RegisterUser(ctx context.Context, input register_user.RegisterUserInput) (register_user.RegisterUserOutput, error) {
    return s.register.Execute(ctx, input)
}
```

**`adapters/in/http/register_user_handler.go`**

```go
package http

import (
    "encoding/json"
    "net/http"

    "myapp/internal/features/users/application/ports/in"
    "myapp/internal/features/users/application/usecases/register_user"
)

type RegisterUserHandler struct {
    svc       in.RegisterUser
    presenter *RegisterUserPresenter
}

func (h *RegisterUserHandler) Handle(w http.ResponseWriter, r *http.Request) {
    var req RegisterUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    input := register_user.RegisterUserInput{
        Email:    req.Email,
        Password: req.Password,
        FullName: req.FullName,
    }

    output, err := h.svc.RegisterUser(r.Context(), input)
    if err != nil {
        h.presenter.PresentError(w, err)
        return
    }

    vm := h.presenter.PresentRegisterUser(output)
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    _ = json.NewEncoder(w).Encode(vm)
}
```

**`adapters/in/http/register_user_presenter.go`**

```go
package http

import (
    "time"

    "myapp/internal/features/users/application/usecases/register_user"
)

type UserViewModel struct {
    ID        string `json:"id"`
    Email     string `json:"email"`
    CreatedAt string `json:"created_at"`
}

type RegisterUserPresenter struct{}

func (p *RegisterUserPresenter) PresentRegisterUser(output register_user.RegisterUserOutput) UserViewModel {
    return UserViewModel{
        ID:        output.UserID,
        Email:     output.Email,
        CreatedAt: output.CreatedAt.Format(time.RFC3339),
    }
}
```

**`adapters/out/external/email_notifier.go`**

```go
package external

import (
    "context"

    "myapp/internal/features/users/domain"
    "myapp/internal/shared/adapters/out/integrations/email"
)

type EmailNotifier struct {
    client *email.Client // raw technical client from shared/integrations
}

func (n *EmailNotifier) SendWelcomeEmail(ctx context.Context, to domain.Email, input domain.WelcomeInput) error {
    return n.client.Send(ctx, email.Message{
        To:      to.String(),
        Subject: "Welcome!",
        Body:    renderWelcomeBody(input),
    })
}
```

This keeps the **technical client** in `shared/adapters/out/integrations/email/` and the **business meaning** ("welcome email after registration") in the `users` feature.

---

## 8. ORM (Bun) Modeling Policy

```
adapters/out/postgres/
  models/
    <entity>_model.go      # Bun struct with `bun:"..."` tags
  mapper.go                # domain ↔ Bun mapping (both directions)
  <entity>_repo.go         # implements OUT repository port
```

- Bun models **never** leave `adapters/out/postgres/`.
- Repositories accept and return **domain entities** at their boundary.
- `mapper.go` is the only place that knows both shapes.
- **Never share Bun models across features.** If two features need the same data, expose it through an IN port on the owning feature, or build a per-feature read model (projection).

---

## 9. Sharing External Integrations Across Features

Multiple features may need the same external system (Stripe for `checkout` and `subscriptions`, Jira for several reporting features, etc.). Apply this layering:

1. **Raw technical client** lives in `internal/shared/adapters/out/integrations/<system>/`.
   - Authentication, retries, pagination, wire-format structs.
   - No business meaning, no feature-specific naming.
2. **Feature-specific OUT port** lives in `internal/features/<feature>/application/ports/out/`.
   - E.g., `PaymentGateway` for `checkout`, `BillingGateway` for `subscriptions`.
3. **Feature-specific OUT adapter** lives in `internal/features/<feature>/adapters/out/external/`.
   - Implements the feature's OUT port by wrapping the shared raw client.

This way the raw integration is shared, but each feature owns its own abstraction over it. **Features still never import each other.**

---

## 10. Sharing Cross-Feature Data (e.g., `Project`)

Preferred patterns, in order:

1. **Dedicated feature** that owns the entity's lifecycle (e.g., a `projects` feature) and exposes IN ports for reads/writes other features need.
2. **Per-feature read models** (projections) populated via events or scheduled jobs.
3. **Anti-corruption layer** when consuming external/legacy data.

**Never share Bun models, repositories, or domain types across features.**

---

## 11. Testing

| Layer | Test type | Notes |
| --- | --- | --- |
| **Domain** | Pure unit tests | No mocks. Test invariants, value-object behavior, aggregate operations. |
| **Application (Use Cases / Services)** | Unit tests with mocked OUT ports | Mock repositories and gateways. No real DB, no real HTTP. |
| **Adapters IN** | Handler tests | Test request parsing, presenter invocation, status-code mapping. Mock the IN port. |
| **Adapters OUT** | Integration tests | Real DB (testcontainers) or recorded HTTP fixtures for external systems. |
| **End-to-end** | Optional | Spin up the wired app and exercise it through its real HTTP/gRPC surface. |

Refer to [test-standards.context.md](test-standards.context.md) for table-driven test patterns and the API-first TDD philosophy.

---

## 12. Anti-Patterns (Forbidden)

1. **Sharing ORM/Bun models across features.**
2. **Business logic in adapters** (handlers or repositories).
3. **Application layer importing Bun, HTTP, JSON, or any concrete adapter.**
4. **IN adapters calling external APIs directly** instead of going through the application + OUT ports.
5. **OUT adapters knowing HTTP semantics** (status codes, headers, request bodies).
6. **God integration packages** that mix multiple external systems.
7. **Features importing other features** (domain, application, or adapters).
8. **`utils/`, `helpers/`, `common/` dumping grounds.** Use specific, intent-revealing package names.
9. **Over-abstracting too early.** Add interfaces only when there is a real second implementation or a real testing need.
10. **Treating folders as architecture.** The structure is a consequence of the dependency rules, not a substitute for them.
11. **Use Cases calling Presenters.** Use Cases return plain Outputs; Presenters are invoked only by IN adapters.
12. **Application Services depending on Presenters.** Application Services return plain Outputs; they do not know about ViewModels or serialization.
13. **Use-case Inputs / Outputs with `json` tags.** Plain structs only. Serialization tags belong in `adapters/in/*/view_models.go` (and Request structs).
14. **Passing Presenters to Use Cases or Application Services.** Violates dependency inversion and couples the application core to the delivery mechanism.
15. **Domain entities with `bun`, `json`, or other infrastructure tags.**
16. **Repositories returning ORM models** instead of domain entities.

---

## 13. Golden Rule

> **If changing a framework, database, or external API forces changes in domain or use-case code, the architecture is already broken.**

The domain and the application layer must be replaceable HTTP frameworks, ORMs, message brokers, and SaaS providers — without modification. Adapters absorb that change.
