# Boundary Contracts

Read this reference when changing a transport, protocol adapter, presenter,
persistence adapter, external gateway, validation path, or layer-specific test.

The universal rules and selected archetype in `../SKILL.md` remain authoritative.

## General Boundary Flow

A transformation boundary commonly looks like:

```text
external representation
       ↓ parse / validate / map
owned behavior or contract
       ↓
external capability contract
       ↑ implemented by
external adapter
```

Not every boundary requires mapping. Technical passthrough systems MAY preserve
raw data when that is part of their semantics or performance requirements.

## Transport / Protocol Ownership

Transport or protocol boundaries may own:

- HTTP, gRPC, CLI, message, or wire request/response types;
- serialization tags;
- headers and status codes;
- protocol-level validation;
- streaming/framing semantics;
- mapping between external and owned concepts.

Business domain/application errors SHOULD remain transport-independent and be
mapped at the incoming boundary.

Technical systems may intentionally preserve protocol errors or raw responses
when transparent proxying is part of the contract.

## Validation Ownership

Place validation with the layer that owns the rule.

| Owner | Responsibility |
| --- | --- |
| Core/domain | Invariants intrinsic to the product concept |
| Application/capability | Preconditions for an operation |
| Protocol/transport | Parsing, required wire fields, framing, protocol format |
| Configuration owner | Configuration semantics and valid combinations |

Do not introduce a dedicated validator abstraction when a constructor or method
expresses the rule more clearly.

## Persistence Boundary

Persistence-specific models, ORM tags, queries, and drivers stay in the
persistence boundary when the application benefits from isolation.

```text
owned model / application result
          ⇅ mapper
persistence representation
          ⇅
database / ORM
```

Business repositories SHOULD expose business/application-owned concepts rather
than ORM records.

Technical projects MAY expose lower-level storage concepts when storage is part
of the product's core abstraction, such as a database or storage engine.

## External SDK / API Boundary

Keep vendor-specific SDK and wire concerns with the adapter that owns the
integration.

Do not create an adapter merely to hide a stable library that already provides
the exact capability needed. Introduce a boundary when it protects meaningful
semantics or replacement cost.

## Presenters

A presenter is optional. Use one when response mapping is substantial, reused,
or worth testing independently.

A presenter belongs to the incoming boundary and MUST NOT become a backchannel
from core behavior into transport concerns.

## Transactions

Business applications decide atomicity at the application/business operation
boundary without importing concrete ORM transaction types into domain behavior.

Expose a transaction or unit-of-work capability only when an operation truly
requires it.

Technical systems whose product semantics are transactional may define those
contracts closer to their core.

## Testing by Boundary

Match test style to the boundary:

| Boundary | Default style |
| --- | --- |
| Pure core behavior | Unit tests without infrastructure |
| Capability using external contracts | Unit tests with focused fakes/mocks where useful |
| Protocol/transport adapter | Handler/driver/contract tests |
| External/persistence adapter | Integration or contract tests against a faithful substitute or real dependency |
| Wired executable | Selective end-to-end tests |

## Review Checklist

- [ ] The boundary owns its external representation.
- [ ] Mapping exists only when it protects useful semantics or compatibility.
- [ ] Protocol/persistence/vendor details do not leak inward accidentally.
- [ ] Validation lives with the owner of the rule.
- [ ] Errors are translated at the appropriate boundary.
- [ ] Tests replace dependencies at explicit boundaries rather than reaching through layers.
