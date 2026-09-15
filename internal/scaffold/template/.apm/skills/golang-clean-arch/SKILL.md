---
name: golang-clean-arch
description: >
  Authoritative capability-oriented Clean Architecture guidance for Go business
  applications, technical applications, libraries, and SDKs. Defines dependency
  direction, package ownership, boundary contracts, configuration ownership,
  composition, extension points, and archetype-specific structures while
  minimizing architectural ceremony.
user-invocable: true
license: MIT
compatibility: >
  Designed for Claude Code or similar AI coding agents, and for projects using
  Golang.
metadata:
  author: company
  version: "2.0.0"
  openclaw:
    emoji: "🏗️"
    requires:
      bins:
        - go
    install: []
---

# Capability-Oriented Clean Architecture for Go

## Goal

Build Go software that is easy to navigate, maintain, test, and extend without
forcing business-application layers onto technical systems or libraries.

Optimize for:

- explicit dependency direction;
- clear ownership;
- small change surfaces;
- stable boundaries;
- infrastructure isolation where it provides value;
- minimal architectural ceremony;
- idiomatic Go.

Prefer the simplest architecture that preserves the required boundaries.

## Authority

Apply instructions in this order:

1. repository-specific instructions and ADRs;
2. this skill;
3. the archetype reference selected by this skill;
4. specialized Go skills;
5. upstream architecture references.

Use specialized skills for detailed error handling, context propagation,
testing mechanics, concurrency, security, database practices, and general Go
style. Do not duplicate those standards here.

## Architecture Selection

Before designing packages, classify the project or subsystem by its dominant
responsibility.

| Archetype | Use when | Reference |
| --- | --- | --- |
| Business Application | Behavior is organized around business capabilities, workflows, policies, and domain concepts | [`references/business-application.md`](references/business-application.md) |
| Technical Application | The system is primarily a gateway, proxy, broker, compiler, database, CLI, agent runtime, orchestrator, protocol server, or infrastructure tool | [`references/technical-application.md`](references/technical-application.md) |
| Library / SDK | The primary product is a reusable Go package consumed by other applications | [`references/library-sdk.md`](references/library-sdk.md) |

A repository may contain more than one archetype. Classify each major subsystem
independently when necessary. Do not force the whole repository into one shape.

## Universal Rules

### 1. Organize Around Owned Concepts

Packages SHOULD represent cohesive concepts or capabilities that have clear
ownership.

Use business terminology for business software and technical terminology for
technical software.

Good names depend on the project:

```text
Business: order, billing, inventory
Technical: gateway, protocol, transport, runtime, compiler, scheduler
Library: client, codec, retry, transport
```

Avoid generic names such as `utils`, `helpers`, `common`, or `misc` when a more
precise concept exists.

Names such as `core`, `engine`, `processor`, or `runtime` are acceptable only
when they describe a real technical concept rather than hiding unclear
ownership.

### 2. Dependency Direction Matters More Than Folder Shape

Folder placement alone does not establish Clean Architecture.

Dependencies SHOULD point from volatile delivery/infrastructure concerns toward
stable behavior and contracts.

```text
external surface / infrastructure
             ↓
        application behavior
             ↓
       stable core concepts
```

Technical systems may use a different concrete flow, for example:

```text
protocol adapter -> gateway core -> provider contract <- provider adapter
```

Inspect imports, construction, and runtime calls when validating architecture.

### 3. Identify the Core Before Creating Layers

The core is the behavior whose semantics define the product.

Examples:

```text
Business app  -> domain policies and application operations
Gateway       -> routing, provider resolution, dispatch
Compiler      -> parsing semantics, AST transformations, type rules
Library       -> public behavior and algorithms
```

The core does not require a package literally named `core` or `domain`.

Do not create `domain`, `application`, `repository`, `service`, or `usecase`
packages unless the behavior actually needs those boundaries.

### 4. Ports and Interfaces Are Tools, Not Mandatory Layers

Create an interface when a real consumer benefits from substitution, isolation,
or a stable contract.

Prefer consumer-owned interfaces for internal boundaries.

```go
type ProviderResolver interface {
    Resolve(name string) (Provider, error)
}
```

Do not wrap a library automatically when the library already exposes the right
contract.

Do not create generic repository abstractions that erase intent.

Public extension APIs are an exception: their contracts belong to the public
package that intentionally owns them.

### 5. Keep Boundary-Specific Types at Their Boundary

Transport, persistence, SDK, generated-wire, and protocol-specific types SHOULD
remain with the adapter or subsystem that owns them when translating them inward
would protect the core from external change.

Do not map merely for ceremony. Passthrough-oriented technical systems MAY keep
raw wire data when preserving it is part of the required behavior or performance
model.

Read [`references/boundary-contracts.md`](references/boundary-contracts.md) when
designing transport, persistence, protocol, or external-service boundaries.

### 6. Configuration Is Owned, But Does Not Require a Package Per Component

A component owns the meaning and validation of its configuration.

Use a dedicated `config` package when it:

- prevents dependency cycles;
- forms a reusable boundary;
- aggregates configuration from children; or
- materially improves clarity.

For small libraries or cohesive components, a local `Config` or `Options` type
is preferred over an unnecessary subpackage.

Runtime loading, files, environment variables, flags, and secrets belong at the
application/composition boundary unless the project is itself a configuration
library.

### 7. Dependency Injection Belongs at Composition Boundaries

Constructors MUST remain independent from DI containers.

```go
func NewService(client Client, cfg Config) *Service
```

not:

```go
func NewService(i do.Injector) *Service
```

Applications MAY use a DI container at composition boundaries. Libraries SHOULD
normally use constructors and functional options rather than expose or require a
DI framework.

When using `github.com/samber/do/v2`, read
[`references/samber-do-composition.md`](references/samber-do-composition.md).

### 8. Composition Owns Construction and Lifecycle

An executable composition root owns:

- runtime configuration loading;
- dependency assembly;
- adapter selection;
- startup;
- lifecycle;
- shutdown.

It MUST NOT contain product behavior that belongs to another package.

Runtime code SHOULD operate on already-composed Go values rather than resolving
normal dependencies repeatedly from a container or registry on hot paths.

### 9. Share Concepts Deliberately

Share a package only when its consumers agree on its meaning and lifecycle.

Prefer limited duplication over a misleading shared abstraction.

Avoid dumping grounds:

```text
shared/common
shared/utils
shared/helpers
shared/models
```

Technical primitives that truly have the same semantics for multiple consumers
may be shared.

### 10. Preserve Replacement Boundaries

Use replacement cost as a diagnostic.

Changing a database, framework, external API, transport, protocol library, or
vendor SHOULD primarily affect its owning boundary and composition rather than
unrelated core behavior.

This is a diagnostic, not a mandate to abstract every dependency.

## Cross-Capability Collaboration

Business features require stronger isolation than technical subsystems.

For business feature collaboration, read
[`references/cross-feature-collaboration.md`](references/cross-feature-collaboration.md).

For technical systems, intentional dependencies between capabilities are
allowed when they follow the documented direction. Avoid cycles and sideways
coupling between peer subsystems.

## Implementation Workflow

When creating or restructuring architecture:

1. Classify the project/subsystem archetype.
2. Identify the core behavior.
3. Identify external boundaries and volatile dependencies.
4. Define package ownership using project-native terminology.
5. Document the intended dependency direction.
6. Add interfaces only where a consumer needs a boundary.
7. Keep boundary-specific details at their owner.
8. Add configuration and DI abstractions only when justified.
9. Compose runtime dependencies at the executable boundary.
10. Inspect imports and runtime calls for violations.
11. Validate the expected change surface and replacement cost.

## Architecture Validation

Before completing architecture-related work, verify:

- [ ] The correct archetype was selected for each major subsystem.
- [ ] Packages represent real owned concepts or capabilities.
- [ ] New behavior has one clear owner.
- [ ] Dependency direction is explicit and acyclic.
- [ ] Core behavior does not depend unnecessarily on delivery or infrastructure details.
- [ ] Interfaces exist because a real consumer or public contract needs them.
- [ ] External/wire-specific types do not leak across boundaries without a deliberate reason.
- [ ] Configuration ownership is clear and does not create cycles.
- [ ] Constructors are independent from DI containers.
- [ ] Composition owns construction, startup, and shutdown.
- [ ] Shared packages have stable meaning for every consumer.
- [ ] No empty layers or placeholder abstractions were created.
- [ ] The change remains inside the smallest reasonable ownership boundary.
- [ ] Replacing a volatile dependency does not unnecessarily rewrite core behavior.

## Principle

> Apply Clean Architecture through ownership and dependency direction, not
> mandatory folders or layers.
