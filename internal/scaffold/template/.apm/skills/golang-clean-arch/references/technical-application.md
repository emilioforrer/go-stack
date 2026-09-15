# Technical Application Profile

Use this profile for gateways, proxies, brokers, protocol servers, databases,
compilers, runtimes, CLIs, infrastructure tools, schedulers, orchestrators,
agents, and similar systems.

The universal rules in `../SKILL.md` remain authoritative.

## Primary Rule

Organize around the technical capabilities that actually define the system.
Do not invent business features, entities, repositories, or use cases when they
are not part of the problem.

## Example Structure

A gateway might use:

```text
project/
├── cmd/
│   └── gateway/
├── internal/
│   ├── app/                    # composition root
│   ├── config/
│   ├── gateway/                # routing/dispatch semantics
│   ├── protocol/
│   │   ├── openai/
│   │   └── anthropic/
│   ├── provider/
│   │   ├── openai/
│   │   └── vendorx/
│   ├── middleware/
│   └── testutil/
└── <public extension packages if intentionally exposed>
```

A compiler, broker, or scheduler SHOULD use names native to its own technical
model rather than copying this tree.

## Core Behavior

Identify the semantics that define the product and keep them independent from
unnecessary delivery or infrastructure concerns.

Examples:

```text
Gateway   -> routing, capability resolution, dispatch
Compiler  -> AST, type rules, transformations
Broker    -> routing, acknowledgement, delivery semantics
Scheduler -> planning, ordering, execution semantics
Database  -> query/storage semantics
```

The core does not require a package named `core`.

## Technical Names Are Valid

Names such as these are legitimate when they represent real concepts:

```text
gateway
engine
runtime
protocol
transport
codec
scheduler
planner
executor
processor
```

Do not reject them merely because they are technical. Reject them when they are
vague dumping grounds.

## Dependency Direction

Document the intended technical flow explicitly.

Example:

```text
HTTP / CLI / configuration
          ↓
   protocol adapters
          ↓
     gateway core
          ↓
    provider contract
          ↑
    provider adapters
```

Intentional dependencies from higher-level technical capabilities to lower-level
stable capabilities are allowed.

Avoid:

- cycles;
- peer subsystems reaching into each other's internals;
- adapters owning core semantics;
- global service locators on runtime paths.

## Protocols and Transports

A protocol adapter owns protocol-specific behavior such as:

- request/response shapes;
- serialization;
- wire-level validation;
- status/error mapping;
- framing;
- streaming semantics;
- translation to another protocol when required.

Do not automatically normalize all protocols into a large universal object
model. Preserve raw/passthrough data when doing so is semantically correct and
important for performance or forward compatibility.

## Providers and External Systems

External providers/adapters own vendor-specific behavior such as:

- authentication mechanics;
- upstream URLs;
- wire details;
- SDK-specific types;
- provider-specific retries or translation.

The core SHOULD depend on the capability it consumes, not on concrete provider
packages.

Prefer capability-oriented interfaces:

```go
type Provider interface {
    RoundTrip(context.Context, *Request) (*http.Response, error)
}
```

rather than repository terminology when no persistence abstraction exists.

## Hot Paths

For performance-sensitive systems:

- construct and compose dependencies at startup;
- avoid repeated registry/DI lookups on the hot path;
- avoid mappings and allocations that provide no isolation benefit;
- preserve streaming and cancellation semantics;
- add abstraction only when it protects a real boundary.

Clean Architecture MUST NOT force unnecessary transformations that conflict with
the product's performance model.

## Configuration

Large technical applications commonly benefit from a central runtime loader plus
component-owned configuration semantics.

A dedicated child `config` package is optional. Use it when it prevents cycles
or forms a meaningful contract.

## Extension APIs

If third-party extensions are part of the product, their public API is an
intentional architectural boundary.

Keep extension contracts:

- small;
- stable;
- independent from internal DI/container types;
- independent from application config loaders;
- explicit about lifecycle and ownership.

Runtime extension discovery is a separate product decision and is not implied by
Clean Architecture.

## Review Checklist

- [ ] Packages use terminology native to the technical system.
- [ ] The core semantics are explicitly identified.
- [ ] Dependency direction is documented and acyclic.
- [ ] Protocol and vendor details are owned by their boundaries.
- [ ] No fake business/domain layers were created.
- [ ] No repository abstraction exists without persistence semantics that need it.
- [ ] Interfaces describe actual consumed capabilities.
- [ ] Hot paths do not resolve ordinary dependencies through DI/registries.
- [ ] Passthrough/streaming paths are not burdened by ceremonial mapping.
