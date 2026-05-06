---
description: Architecture standards (short) — Clean Architecture + DDD + Vertical Slice with Hexagonal boundaries. For worked examples, code skeletons, and full rationale, see architecture-long.context.md.
applyTo: '**/*.go'
---

# Architecture Standards (Short Reference)

> **For the full specification, worked examples, and code skeletons** — or when adding/restructuring a feature, deciding unclear layer/boundary placement, writing or modifying repositories/gateways/OUT adapters, wiring shared integrations across features, or changing HTTP/gRPC handlers, presenters, or routes — see [architecture-long.context.md](architecture-long.context.md).


**Clean Architecture + DDD + Vertical Slice** with explicit **Hexagonal (Ports & Adapters)** boundaries.

Goal: keep business logic independent of frameworks, persistence, serialization, and external systems while each feature evolves as an independent DDD bounded context.

---

## 1. Dependency Rule

Dependencies point inward only:

```text
adapters/in   ->  application  ->  domain
adapters/out  ->  application  ->  domain
```

- `domain/` has **zero** dependencies on frameworks, persistence, transport, serialization, application ports, or adapters.
- `application/` depends on `domain/` only. **Never** import Bun, HTTP, gRPC, JSON encoders, SDK clients, or any concrete adapter.
- `adapters/in/*` and `adapters/out/*` depend on `application/` and `domain/`, but **never on each other**.
- Inner layers **MUST NEVER** import outer layers.

---

## 2. Feature Boundaries

Each `internal/features/<feature>/` is a vertical slice — a DDD bounded context with its own domain, use cases, and adapters.

A feature **may import**:
- Its own `domain`, `application`, `adapters` packages.
- `internal/shared/*` (shared kernel, cross-cutting utilities).

A feature **MUST NOT import** another feature's:
- `domain`, `application`, `adapters`, Bun models, repositories, gateways, request/view models, or domain types.

Cross-feature communication must use (in preference order):
1. A **dedicated owning feature** exposing IN ports for reads/writes. Composition/driver code outside feature packages wires them.
2. **Per-feature read models** (projections) populated by domain events or scheduled jobs.
3. **Domain events** with anti-corruption translation in the consuming feature.

**Never share Bun models, repositories, gateways, or domain types across features.** The consuming feature still must not import the owning feature.

---

## 3. Ports and Adapters (Hexagonal)

| Component | Meaning | Location |
| --- | --- | --- |
| **IN ports** (primary/driving) | What the application exposes to drivers | `application/ports/in/` |
| **OUT ports** (secondary/driven) | What the application needs from infrastructure | `application/ports/out/` |
| **IN adapters** | Drivers that call the app (HTTP, gRPC, CLI, consumers) | `adapters/in/` |
| **OUT adapters** | Driven infra called by the app (repos, gateways, messaging) | `adapters/out/` |

- IN adapters **call** IN ports.
- OUT adapters **implement** OUT ports.
- Interfaces live where they are **consumed**, not where they are implemented.

---

## 4. Repository Layout

```text
cmd/<binary>/main.go                         # flags, DI, Run() — no business logic
internal/
  features/<feature>/
    domain/
      <entity>.go                            # entities, aggregate roots
      <value_object>.go                      # value objects
      rules.go, errors.go, events.go         # invariants, domain errors, events
    application/
      service/service.go                     # implements IN ports, orchestrates use cases
      ports/in/<use_case>.go                 # IN port interface; uses use-case Input/Output types
      ports/out/<repository>.go, <gateway>.go # OUT port interfaces
      usecases/<use_case>/
        usecase.go                           # core logic — returns plain Output
        validator.go                         # use-case validation rules
        types.go                             # Input, Output — plain structs, NO tags
        presenter.go                         # optional documentation-only interface
    adapters/
      in/http/
        routes.go
        <use_case>_handler.go                # parse request, transport validate, call IN port, call presenter
        <use_case>_presenter.go              # Output -> ViewModel (json tags HERE)
        requests.go, view_models.go          # serialization tags allowed HERE
      in/grpc/, in/cli/                      # optional alternative drivers
      out/postgres/
        models/<entity>_model.go             # Bun model — bun tags ONLY here
        mapper.go                            # domain <-> Bun mapping
        <entity>_repo.go                     # implements OUT repository port
      out/external/<gateway>.go              # implements OUT gateway port
  shared/
    kernel/                                  # tiny shared domain primitives (Money, IDs, Time)
    observability/                           # slog, metrics, tracing
    adapters/in/<transport>/                 # reusable inbound building blocks only
    adapters/out/<technology>/               # reusable outbound building blocks only
    adapters/out/integrations/<system>/      # raw technical clients only (no business logic)
pkg/                                         # optional — exported reusable libraries only
```

**`internal/shared/` rules:**
- `shared/kernel/` — small set of value objects used across features. Add to it **conservatively**.
- `shared/adapters/in/<transport>/` — reusable inbound building blocks: middleware, interceptors, error mappers, response helpers, router/server bootstrap, CLI scaffolding. **No feature-specific handlers or routes. No business logic.**
- `shared/adapters/out/<technology>/` — reusable outbound building blocks: DB pool setup, transaction helpers, base repository scaffolding, cache/messaging client wiring. **No feature-specific repositories, Bun models, or gateways. No business logic.**
- `shared/adapters/out/integrations/<system>/` — raw technical clients: auth, retries, pagination, rate limiting, wire-format structs. **No business logic. No feature-specific naming.** (e.g., Stripe client does not know about "checkout" or "subscriptions").

---

## 5. Layer Responsibilities

| Layer | Owns | Must | Must Not |
| --- | --- | --- | --- |
| **Domain** | Entities, aggregates, value objects, invariants, domain services, errors, events | Enforce invariants through aggregate roots; stay framework-agnostic | Import application ports, adapters, Bun, HTTP, gRPC, serialization, SDK clients, or any infrastructure |
| **Application (Use Cases)** | Core business logic for one use case | Return plain Output structs + errors; call OUT ports; orchestrate domain | Know HTTP, JSON, DB, ORM, Presenters, ViewModels; return structs with serialization tags |
| **Application (Services)** | Implements IN ports; orchestrates 1+ Use Cases; manages transactions | Delegate business logic to Use Cases; return plain Outputs | Contain business logic; depend on Presenters; know transport concerns |
| **IN Adapters** | HTTP/gRPC/CLI handlers, consumers | Parse requests; transport validate; map to Inputs; call IN ports; invoke Presenters; encode responses/set status codes | Contain business logic; call OUT adapters or external APIs directly |
| **OUT Adapters** | Repositories, external gateways, messaging, cache, storage | Implement OUT port interfaces; map infra shapes <-> domain at boundary | Leak Bun models/API payloads/HTTP semantics into application or domain; be imported by IN adapters or other features |

**Only inbound adapters** may contain: transport request/response structs, serialization tags, status-code mapping, and presenter invocation.

---

## 6. Component Contracts

**Use Case:**
```go
func (uc *CreateOrderUseCase) Execute(ctx context.Context, input CreateOrderInput) (CreateOrderOutput, error)
```
- Contains core application logic for one operation.
- Orchestrates domain entities, value objects, domain services, and OUT ports.
- Calls use-case Validator before execution.
- Returns plain Output structs and/or domain/application errors.
- Must not know HTTP, JSON, DB, ORM, Presenters, ViewModels, or Application Services.

**Application Service:**
```go
func (s *OrderApplicationService) CreateOrder(ctx context.Context, input CreateOrderInput) (CreateOrderOutput, error)
```
- Implements IN port interface.
- Delegates to one or more Use Cases.
- Handles transactions and cross-use-case coordination.
- Must not call Presenters, know transport concerns, or contain core business logic.

**Presenter:**
```go
func (p *HTTPOrderPresenter) PresentCreateOrder(output CreateOrderOutput) OrderViewModel
func (p *HTTPOrderPresenter) PresentError(err error) ErrorViewModel
```
- Belongs to the **delivery layer** under `adapters/in/<transport>/`.
- Transforms use-case Output to transport-specific ViewModel.
- Formats, renames, omits, or adds presentation fields. Applies serialization tags.
- Called **only by inbound adapters**.
- Must not be passed to or invoked by Use Cases or Application Services.
- Must not contain business logic or access DBs/external services.

A presenter **interface** may be declared in `application/usecases/<uc>/presenter.go` for documentation purposes only. Application code must never invoke it.

---

## 7. Validation Layers

| Layer | Location | Responsibility |
| --- | --- | --- |
| **Domain invariants** | `domain/rules.go`, entity constructors, aggregate methods | Business rules that must always hold (e.g., "order has at least one item") |
| **Use-case validation** | `application/usecases/<uc>/validator.go` | Operation-specific rules (e.g., "email must be unique at registration", "password policy"). May call OUT ports for pre-checks. |
| **Transport validation** | `adapters/in/*/<handler>.go` | Well-formed payload (valid JSON, required fields present, basic format). Returns 400-class errors before reaching application. |

Failures at one layer never bubble untranslated into another. Domain/application errors are mapped to HTTP/gRPC status codes only by inbound adapters.

---

## 8. Call Flow

```text
HTTP/gRPC/CLI Request
  -> IN Adapter: parse request, transport validate, map to application Input
  -> Application Service: called through IN port
  -> Application Service: invokes one or more Use Cases
  -> Use Case: validates (use-case rules), orchestrates domain, calls OUT ports
  -> OUT Adapter: implements OUT port, maps infra <-> domain
  <- plain Output returns to IN Adapter
  -> IN Adapter: calls Presenter
  -> Presenter: Output -> ViewModel (with serialization tags)
  -> IN Adapter: encodes ViewModel, sets HTTP/gRPC status, writes response
```

---

## 9. Persistence and Serialization

**ORM (Bun) Policy:**
- Bun-tagged structs live **only** in `adapters/out/postgres/models/`.
- Bun models **never** leave `adapters/out/postgres/`.
- Repositories accept and return **domain entities** at their boundary.
- `mapper.go` is the only place that knows both domain and Bun shapes.
- **Never share Bun models across features.**

**Serialization Policy:**
- `json`, `xml`, `protobuf`, `bson`, `db`, `gorm`, and `bun` tags are **forbidden** in `domain/` and `application/`.
- Use-case Inputs and Outputs are **plain Go structs** — no serialization tags.
- Serialization tags belong **only** in inbound adapter request structs and ViewModels at `adapters/in/*/`.

---

## 10. External Integrations

When multiple features use the same external system (e.g., Stripe, Jira):

1. **Raw technical client** in `internal/shared/adapters/out/integrations/<system>/` — owns auth, retries, pagination, rate limiting, wire-format structs. No business meaning. No feature-specific naming.
2. **Feature-specific OUT port** in `internal/features/<feature>/application/ports/out/` — business abstraction (e.g., `PaymentGateway`, `BillingGateway`, `Notifier`).
3. **Feature-specific OUT adapter** in `internal/features/<feature>/adapters/out/external/` — wraps the shared raw client to implement the feature's OUT port.

Each feature owns its own abstraction over the shared raw client. Features still never import each other.

---

## 11. Testing by Layer

| Layer | Test Style |
| --- | --- |
| **Domain** | Pure unit tests. No mocks. Test invariants, value-object behavior, aggregate operations. |
| **Application (Use Cases/Services)** | Unit tests with mocked OUT ports. No real DB, HTTP, gRPC, or external services. |
| **IN Adapters** | Handler/driver tests. Mock IN ports. Assert request parsing, presenter invocation, error/status mapping. |
| **OUT Adapters** | Integration tests with real DB (testcontainers) or recorded HTTP fixtures for external systems. |
| **End-to-end** | Optional. Spin up wired app, exercise through real HTTP/gRPC/CLI surface. |

See [test-standards.context.md](test-standards.context.md) for detailed testing rules, table-driven patterns, and API-first TDD.

---

## 12. Forbidden Anti-Patterns

1. Sharing ORM/Bun models, repositories, gateways, or domain types across features.
2. Features importing other features (domain, application, or adapters).
3. Business logic in handlers, presenters, repositories, or raw integration clients.
4. Application layer importing Bun, HTTP, gRPC, JSON encoders, concrete adapters, or SDK clients.
5. Domain importing application ports, adapters, persistence, transport, or any infrastructure.
6. IN adapters calling external APIs directly instead of going through application + OUT ports.
7. OUT adapters knowing HTTP/gRPC semantics (status codes, headers, request bodies).
8. Use Cases or Application Services calling, depending on, or receiving Presenters.
9. Use-case Inputs/Outputs with `json`, `xml`, `protobuf`, or any serialization tags.
10. Domain entities with `bun`, `json`, `db`, `gorm`, protobuf, or infrastructure tags.
11. Repositories returning ORM/Bun models instead of domain entities.
12. God integration packages that mix multiple external systems or business meanings.
13. `utils/`, `helpers/`, `common/` dumping grounds. Use specific, intent-revealing package names.
14. Over-abstracting too early. Add interfaces only when there is a real testing need, a real second implementation, or a real architectural boundary.
15. Treating folders as architecture. Folder layout follows the dependency rules; it does not replace them.

---

## 13. Golden Rule

> **If changing a framework, database, message broker, or external API forces changes in domain or use-case code, the architecture is already broken.**

Adapters absorb infrastructure changes. Domain and application code stay stable.

---

*Short reference. For worked examples (users feature registration), detailed code skeletons, ORM modeling policy, and full explanations, see [architecture-long.context.md](architecture-long.context.md).*
