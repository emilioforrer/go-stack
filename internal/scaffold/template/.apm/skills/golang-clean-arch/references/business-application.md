# Business Application Profile

Use this profile when the system is primarily organized around business
capabilities, policies, workflows, and domain concepts.

The universal rules in `../SKILL.md` remain authoritative.

## Default Structure

```text
project/
├── cmd/
│   └── api/
├── internal/
│   ├── appconfig/
│   ├── features/
│   │   ├── order/
│   │   │   ├── package.go
│   │   │   ├── config/                 # only when useful
│   │   │   ├── domain/
│   │   │   ├── application/
│   │   │   │   ├── port/
│   │   │   │   │   ├── incoming/
│   │   │   │   │   └── outgoing/
│   │   │   │   ├── usecase/
│   │   │   │   └── service/            # optional
│   │   │   └── adapter/
│   │   │       ├── incoming/
│   │   │       └── outgoing/
│   │   └── <feature>/
│   ├── workflows/
│   └── shared/
└── test/
    ├── integration/
    └── endtoend/
```

This is a reference shape, not a package-generation checklist. Create only
packages required by real behavior.

## Feature Ownership

A feature is a cohesive business capability or bounded context, such as:

```text
order
billing
customer
inventory
subscription
fulfillment
```

A feature normally owns its:

- domain model and invariants;
- application operations;
- feature-specific ports;
- incoming and outgoing adapters;
- configuration semantics;
- feature-specific tests;
- dependency registration when the application uses runtime DI.

A normal feature change SHOULD remain mostly inside the owning feature.

## Domain

Use `domain` when the product has a meaningful business model containing
concepts such as:

- aggregates;
- entities;
- value objects;
- business policies;
- domain services;
- events;
- invariants;
- business errors.

Domain code MUST remain independent from transports, persistence, SDKs,
configuration loading, and DI containers.

Do not create a domain layer for CRUD-shaped data with no meaningful rules merely
to satisfy this profile.

## Use Cases

A use case represents one cohesive business operation.

Use-case inputs and outputs SHOULD be transport-independent plain Go types when
that isolation has value.

Examples:

```text
placeorder
cancelorder
createinvoice
reserveinventory
renewsubscription
```

Do not combine unrelated operations into one handler.

A small feature MAY omit application-service or incoming-port layers when no
consumer benefits from them.

## Ports

### Incoming Ports

Incoming ports describe capabilities exposed by a feature. Use them when they
provide a useful stable boundary for:

- multiple incoming adapters;
- workflows;
- external composition;
- substitutable implementations.

Do not create them solely because Clean Architecture diagrams contain them.

### Outgoing Ports

Outgoing ports describe external capabilities required by application behavior.
They belong to the consuming application boundary.

Examples:

```go
type PaymentGateway interface { ... }
type OrderRepository interface { ... }
type EventPublisher interface { ... }
```

Do not create generic repositories that erase business intent.

## Adapters

Incoming adapters translate external requests into feature operations and map
results back to protocol responses.

Outgoing adapters implement capabilities required by the application using
persistence, SDKs, external APIs, queues, storage, or other infrastructure.

Incoming adapters MUST NOT bypass application behavior by calling outgoing
adapters directly.

## Feature Isolation

A business feature MUST NOT import another feature's domain, use-case
implementation, outgoing ports, repositories, adapters, or persistence models.

Cross-feature collaboration uses one of:

1. a workflow;
2. an event;
3. a genuinely shared contract;
4. executable/driver composition;
5. a consumer-owned projection.

Read `cross-feature-collaboration.md` for the detailed rules.

## Workflows

Use `internal/workflows/<name>` when one synchronous business operation
coordinates multiple feature capabilities and the coordination has business
meaning.

A workflow:

- depends on participating feature incoming ports;
- owns cross-feature coordination and errors;
- may own compensation/recovery behavior;
- may define outgoing ports for workflow-owned infrastructure;
- remains outside participating features.

Features MUST NOT depend on workflows.

## Shared Packages

Share business concepts only when they have the same meaning, rules, and
lifecycle for every consumer.

Good candidates may include stable concepts such as money, identity, clocks, or
technical infrastructure that is genuinely shared.

Do not share feature entities or repositories merely because their structures
look similar.

## Expected Dependency Direction

```text
incoming adapter
      ↓
incoming port / use-case
      ↓
application behavior
      ↓
domain + outgoing ports
      ↑
outgoing adapter
```

The exact layers may be collapsed when the feature is small, but infrastructure
must not become the owner of business behavior.

## Review Checklist

- [ ] Feature names use business terminology.
- [ ] Each business rule has one owning feature or workflow.
- [ ] Domain code is infrastructure-independent.
- [ ] No feature imports another feature's implementation.
- [ ] Workflows depend on feature capabilities, not feature internals.
- [ ] Ports exist only where real consumers benefit.
- [ ] Adapter-specific models stay at their boundaries.
- [ ] Shared concepts have identical semantics for all consumers.
