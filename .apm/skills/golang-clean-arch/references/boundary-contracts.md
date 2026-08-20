# Boundary Contracts

Read this reference when changing an incoming adapter, presenter, repository,
external gateway, validation path, transaction boundary, or layer-specific
tests.

The canonical package names and dependency rules come from `../SKILL.md`.
Examples here are illustrative; do not create packages or abstractions that the
behavior does not require.

## End-to-End Flow

```text
transport request
  -> incoming adapter parses and validates the protocol shape
  -> incoming adapter maps request to a plain application input
  -> incoming port or use-case handler executes business behavior
  -> outgoing port describes required infrastructure capability
  -> outgoing adapter maps domain/application types to infrastructure shapes
  <- plain application output or transport-independent error
  -> incoming adapter maps output or error to a protocol response
```

An application service may provide a stable feature facade or coordinate
several use cases. It is optional. A small incoming adapter may call a use-case
handler directly when the main skill permits it.

## Transport and Presentation Boundary

Incoming adapters own:

* HTTP, gRPC, CLI, message-consumer, or other protocol request types;
* parsing and protocol-level validation;
* mapping requests to application inputs;
* mapping application outputs and errors to protocol responses;
* serialization tags, headers, and status codes.

Application inputs and outputs are plain Go types without serialization or
persistence tags. This keeps use cases reusable across transports and prevents
protocol changes from propagating inward.

```go
// application/usecase/createorder/types.go
type Input struct {
	CustomerID string
	Items      []ItemInput
}

type Output struct {
	OrderID string
	Total   int64
}

// adapter/incoming/httpapi/view_models.go
type createOrderResponse struct {
	OrderID string `json:"order_id"`
	Total   int64  `json:"total_minor_units"`
}
```

### Presenters

A presenter is an optional incoming-adapter collaborator. Use one when response
mapping is substantial, shared by several handlers in the same transport, or
worth testing independently. Keep simple mapping in the handler when that is
clearer.

A presenter:

* belongs under the incoming adapter;
* maps application output or errors to transport-specific view models;
* may format, rename, omit, or add presentation fields;
* does not contain business logic or access repositories or external services;
* is never passed into or invoked by domain, use-case, or application-service
  code.

Do not create documentation-only presenter interfaces. Define an interface only
when a real consumer needs substitution.

## Validation Ownership

Validation is distributed by meaning, not centralized by mechanism.

### Domain invariants

Enforce conditions required for every valid domain object through constructors,
aggregate methods, value objects, or domain policies. Invalid domain state
should be difficult or impossible to construct.

Examples include an order requiring at least one item or a money value requiring
a valid currency.

### Use-case validation

Enforce operation-specific business preconditions in the use-case slice. These
rules may depend on outgoing ports, such as checking whether a registration
email is already in use.

A `validator.go` file or validator type is optional. Prefer the smallest form
that keeps the rule visible and testable.

### Transport validation

Incoming adapters reject malformed requests before calling application code:
invalid JSON, missing protocol fields, malformed headers, or invalid wire
formats. They also translate domain and application errors into HTTP, gRPC, CLI,
or messaging semantics.

Do not make domain or application errors carry HTTP status codes, gRPC codes, or
transport response bodies.

## Persistence Boundary

Persistence models and ORM concerns stay inside their outgoing adapter.

```text
application/domain type
        ⇅ mapper inside outgoing adapter
persistence model with ORM tags
        ⇅ ORM/database
```

Repositories accept and return domain entities or application-owned result
types at their application-facing boundary. They do not return ORM models.

For Bun, keep Bun-tagged models inside the PostgreSQL adapter, for example:

```text
adapter/outgoing/persistence/postgres/
├── models/
│   └── order_model.go
├── mapper.go
└── order_repository.go
```

The exact filenames are optional. The invariant is that mapping is performed
inside the adapter and no Bun model or tag leaks into domain or application
packages. Apply the same rule to `db`, `bun`, BSON, SDK, and generated wire
types.

## Transaction Boundaries

The application layer decides which business operation must be atomic, but it
must not import database or ORM transaction types.

When a use case spans several repository operations, expose an
application-owned Unit of Work or transaction capability through an outgoing
port. An outgoing persistence adapter implements it using the concrete database
transaction.

An application service may coordinate a transaction across multiple use cases
when it is a genuine feature-level operation. Do not add an application service
solely to hide a database transaction, and do not let transactions cross feature
boundaries implicitly. Cross-feature workflows need explicit consistency,
compensation, or recovery semantics.

Use the specialized database skill for isolation levels, locking, retries, and
driver-specific transaction mechanics.

## Testing by Boundary

Match test style to the boundary under test:

| Layer | Default test style |
| --- | --- |
| Domain | Pure unit tests without mocks; exercise invariants, value objects, and aggregate behavior |
| Use cases and application services | Unit tests using fakes or mocks for outgoing ports; no real transport or database |
| Incoming adapters | Handler or driver tests using a fake incoming port or use-case handler; assert parsing, mapping, presentation, and protocol errors |
| Outgoing adapters | Integration or contract tests against the real technology or a faithful controlled substitute |
| Wired executable | Selective end-to-end tests through a real incoming surface |

These are architectural test boundaries, not complete testing standards. Use
the specialized testing skill for table-driven tests, fixtures, testcontainers,
race testing, fuzzing, and detailed test mechanics.

## Boundary Review Checklist

* [ ] Application inputs and outputs are plain, transport-independent types.
* [ ] Serialization and persistence tags exist only in their owning adapters.
* [ ] Incoming adapters call application boundaries, not outgoing adapters.
* [ ] Presenters, when used, remain transport-specific and contain no business logic.
* [ ] Domain invariants cannot be bypassed through adapter mapping.
* [ ] Repositories expose domain or application-owned types, not ORM models.
* [ ] Transaction semantics are expressed through an application-owned boundary.
* [ ] Tests replace dependencies at ports rather than reaching across layers.
