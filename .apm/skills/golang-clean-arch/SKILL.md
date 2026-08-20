---
name: golang-clean-arch
description: >
  Authoritative architecture for Golang business applications using
  feature-first Clean Architecture and vertical use-case slices. Defines
  business-oriented package boundaries, feature isolation, incoming and
  outgoing ports, adapters, cross-feature workflows, shared packages, acyclic
  configuration ownership, and samber/do package registration. Apply when
  creating, extending, reviewing, or restructuring a Go business application.
user-invocable: true
license: MIT
compatibility: >
  Designed for Claude Code or similar AI coding agents, and for projects using
  Golang.
metadata:
  author: company
  version: "1.0.0"
  openclaw:
    emoji: "🏗️"
    homepage: https://github.com/samber/cc-skills-golang
    requires:
      bins:
        - go
    install: []
---

# Feature-First Clean Architecture for Go

## Goal

Build business applications that are easy to navigate, maintain, and extend by
humans and AI coding agents.

Optimize for:

* isolated business features;
* business-oriented package organization;
* explicit dependency direction;
* narrow and predictable change surfaces;
* independent business logic;
* multiple incoming and outgoing adapters;
* explicit cross-feature workflows;
* minimal architectural ceremony.

Prefer local clarity over generic abstractions.

## Authority

This skill supersedes the upstream `golang-design-patterns` skill and its
Clean Architecture reference when installed under the same skill name.

Apply instructions in this order:

1. repository-specific instructions;
2. this skill;
3. specialized Go skills;
4. upstream architecture references.

This skill defines application architecture only.

Use specialized skills for:

* error handling;
* context propagation;
* interface and struct design;
* dependency-injection mechanics beyond package registration;
* testing;
* concurrency;
* security;
* database practices;
* refactoring;
* general Go style.

Do not duplicate those rules here.

## Example Reference

For a complete illustrative project structure, read:

* [`references/blog-with-auth.md`](references/blog-with-auth.md)

The example demonstrates:

* an `auth` feature;
* a `profile` feature;
* a `post` feature;
* a `comment` feature;
* authentication shared with other incoming adapters;
* cross-feature workflows;
* package-owned configuration;
* `samber/do` package composition.

The reference is an example, not an additional source of architecture rules.

Do not load it for every task. Read it when:

* creating a new project structure;
* deciding where a package belongs;
* implementing an authentication feature;
* designing a cross-feature workflow;
* the user requests a complete concrete example.

When the example conflicts with this skill or repository-specific instructions,
this skill and the repository instructions take precedence.

## Canonical Structure

Organize the application by business feature first.

```text
project/
├── cmd/
│   ├── api/
│   ├── worker/
│   └── migrate/
│
├── internal/
│   ├── appconfig/
│   │
│   ├── features/
│   │   ├── order/
│   │   │   ├── package.go
│   │   │   ├── config/
│   │   │   ├── domain/
│   │   │   ├── application/
│   │   │   │   ├── port/
│   │   │   │   │   ├── incoming/
│   │   │   │   │   └── outgoing/
│   │   │   │   ├── usecase/
│   │   │   │   │   ├── placeorder/
│   │   │   │   │   ├── cancelorder/
│   │   │   │   │   └── getorder/
│   │   │   │   └── service/
│   │   │   └── adapter/
│   │   │       ├── incoming/
│   │   │       └── outgoing/
│   │   │
│   │   └── <other-feature>/
│   │
│   ├── workflows/
│   │   └── <cross-feature-workflow>/
│   │
│   └── shared/
│       ├── domain/
│       ├── application/
│       └── infrastructure/
│
└── test/
    ├── integration/
    └── endtoend/
```

This is a reference shape.

Create only packages required by real behavior. Do not create empty layers or
placeholder packages.

## Business-Oriented Organization

A feature represents a cohesive business capability or bounded context.

Examples:

```text
order
billing
customer
inventory
subscription
fulfillment
```

Feature and workflow names MUST use terminology from the business domain.

Use-case package names MUST describe business actions:

```text
placeorder
cancelorder
createinvoice
reserveinventory
renewsubscription
```

Do not use generic names as business boundaries:

```text
manager
processor
engine
core
module
businesslogic
```

Do not organize the repository primarily through global technical layers:

```text
internal/domain
internal/service
internal/repository
internal/handler
internal/adapter
```

Technical layers exist inside the business feature that owns them.

## Feature Ownership

Each feature owns its:

* domain model;
* business invariants;
* use cases;
* incoming ports;
* outgoing ports;
* incoming adapters;
* outgoing adapters;
* configuration;
* dependency registration;
* feature-specific tests.

A normal feature change SHOULD remain inside the owning feature.

Avoid modifying unrelated features.

## Package Responsibilities

| Package                                  | Responsibility                                              |
| ---------------------------------------- | ----------------------------------------------------------- |
| `<feature>/domain`                       | Business concepts, rules, invariants, events, and errors    |
| `<feature>/application/usecase/<action>` | One business operation                                      |
| `<feature>/application/port/incoming`    | Capabilities exposed by the feature                         |
| `<feature>/application/port/outgoing`    | External capabilities required by the feature               |
| `<feature>/application/service`          | Stable feature entry point when one is useful               |
| `<feature>/adapter/incoming`             | Translate external requests into feature operations         |
| `<feature>/adapter/outgoing`             | Implement external capabilities required by the feature     |
| `<feature>/config`                       | Aggregate configuration owned by feature children           |
| `<feature>/package.go`                   | Compose feature dependencies                                |
| `workflows/<name>`                       | Coordinate multiple features through incoming ports         |
| `shared/<concept>`                       | A genuinely shared concept independent of concrete features |
| `cmd/<name>`                             | Select packages and compose an executable                   |

## Dependency Direction

Dependencies point toward business behavior.

```text
incoming adapter
      │
      ▼
incoming port
      │
      ▼
application service
      │
      ▼
use-case handler
      │
      ├──────────────► domain
      │
      ▼
outgoing port
      ▲
      │
outgoing adapter
```

The application service is optional when it does not provide a useful feature
boundary.

An incoming adapter MAY call a use-case handler directly when the feature is
small and no stable service facade is required.

## Allowed Dependencies

```text
domain
  └──► approved shared domain concepts

use case
  ├──► feature domain
  ├──► outgoing ports
  └──► approved shared concepts

application service
  ├──► incoming ports
  └──► use-case handlers

incoming adapter
  ├──► incoming ports or use-case handlers
  ├──► application input and output types
  └──► its own config

outgoing adapter
  ├──► outgoing ports
  ├──► feature domain
  ├──► external libraries
  └──► its own config

feature config
  └──► child config packages

feature package
  ├──► feature config
  ├──► adapters
  └──► application registrations

workflow
  └──► incoming ports from participating features

cmd
  ├──► features
  ├──► workflows
  ├──► appconfig
  └──► shared runtime packages
```

## Forbidden Dependencies

```text
feature A             ─X► feature B
feature               ─X► workflow

domain                ─X► application
domain                ─X► adapters
domain                ─X► configuration
domain                ─X► dependency injection
domain                ─X► external systems

application           ─X► incoming adapters
application           ─X► outgoing implementations

workflow              ─X► feature domain
workflow              ─X► feature use-case implementations
workflow              ─X► feature outgoing ports
workflow              ─X► feature repositories
workflow              ─X► feature adapters

child implementation  ─X► parent feature config
child implementation  ─X► parent feature package
child config          ─X► child implementation

shared                ─X► concrete features
shared                ─X► concrete workflows
business logic        ─X► dependency-injection container
```

A workflow MAY define and use its own outgoing ports and repositories for
workflow-owned state or infrastructure. It MUST NOT use another feature's
outgoing ports or repositories.

## Domain

Use `domain` as the package for the business model.

The domain may contain:

* aggregates;
* entities;
* value objects;
* business policies;
* domain services;
* domain events;
* business errors;
* invariants.

The domain MUST remain independent from:

* transports;
* persistence;
* SDKs;
* dependency injection;
* configuration;
* adapters.

Domain types MUST NOT contain transport or persistence concerns.

## Use Cases

Each use case represents one cohesive business operation.

A use case owns its transport-independent:

* input;
* output;
* handler;
* application errors;
* tests.

A use case coordinates domain behavior and outgoing ports.

Use cases MUST NOT depend on:

* incoming adapters;
* outgoing adapter implementations;
* persistence models;
* transport models;
* dependency-injection containers.

Do not combine unrelated business operations into one handler.

## Ports

Use `incoming` and `outgoing` in package paths.

### Incoming Ports

Incoming ports describe capabilities exposed by a feature.

Use them when they provide a useful stable boundary for:

* multiple incoming adapters;
* workflows;
* external composition;
* substitutable feature implementations.

Do not create an incoming interface when no consumer benefits from the
abstraction.

### Outgoing Ports

Outgoing ports describe external capabilities required by a use case.

They belong to the consuming application boundary.

Create an outgoing port when the application requires isolation from an
external implementation.

Do not wrap a library automatically when it already provides an appropriate
extension interface.

Do not create generic repository abstractions that erase business intent.

## Adapters

Incoming adapters translate external requests into feature operations and
translate results back into protocol-specific responses.

Outgoing adapters implement external capabilities required by the application.

Adapter-specific models remain inside their adapters.

Examples of adapter organization:

```text
adapter/incoming/httpapi
adapter/incoming/grpcapi
adapter/incoming/consumer

adapter/outgoing/persistence/postgres
adapter/outgoing/payment/stripe
adapter/outgoing/messaging/nats
adapter/outgoing/storage/s3
```

Business rules MUST NOT live in adapters.

## Feature Isolation

A feature MUST NOT import another feature.

Cross-feature collaboration uses one of:

1. a workflow;
2. an event;
3. a genuinely shared contract;
4. composition at the executable boundary.

Do not access another feature through its domain, repository, adapter, outgoing
port, or use-case implementation.

## Workflows

Use `internal/workflows/<name>` for synchronous business operations that
coordinate multiple features.

Examples:

```text
workflows/checkout
workflows/accountonboarding
workflows/subscriptionrenewal
```

A workflow:

* depends on feature incoming ports;
* owns cross-feature coordination;
* owns workflow-specific input, output, state, and errors;
* may define compensation or recovery behavior;
* remains outside all participating features;
* may define its own outgoing ports for workflow-owned infrastructure.

A workflow MUST NOT import:

* feature domain packages;
* feature use-case implementations;
* feature outgoing ports;
* feature repositories;
* feature adapters.

Features MUST NOT know about workflows.

Concrete workflows MUST NOT be placed under `shared`.

## Shared Packages

Share concepts, not merely similar code.

A package belongs in `shared` only when:

1. multiple consumers require it;
2. it has the same meaning for every consumer;
3. its rules and lifecycle are the same;
4. it does not depend on a concrete feature or workflow.

Examples:

```text
shared/domain/money
shared/domain/identity
shared/application/clock
shared/infrastructure/postgres
shared/infrastructure/observability
```

Avoid generic shared dumping grounds:

```text
shared/common
shared/utils
shared/helpers
shared/models
shared/services
shared/repositories
```

Prefer limited duplication over a misleading shared abstraction.

## Configuration Ownership

Every configurable package owns a dedicated `config` subpackage.

```text
features/order/config
features/order/adapter/incoming/httpapi/config
features/order/adapter/outgoing/persistence/postgres/config
workflows/checkout/config
```

The parent feature config aggregates child config types.

```text
appconfig
    │
    ▼
order/config
    │
    ├──► httpapi/config
    ├──► postgres/config
    └──► stripe/config
```

Implementation packages import only their own config package.

Forbidden:

```text
httpapi          ─X► order/config
postgres         ─X► order/config
stripe           ─X► order/config

httpapi/config   ─X► httpapi
postgres/config  ─X► postgres
stripe/config    ─X► stripe

child package    ─X► feature root package
```

Configuration packages contain configuration types and validation only.

Runtime configuration loading belongs at the application boundary.

## Dependency Registration

Apply this section when the project uses `github.com/samber/do/v2`.

Each runtime package SHOULD export its dependency registrations through a
`Package` variable or function.

Dependency registration belongs in `package.go`.

Business implementation files MUST remain independent from the dependency
container.

### Configurable Packages

A package that requires configuration MUST expose a `Package` function that
receives its package-owned configuration:

```go
package postgres

import (
	"github.com/samber/do/v2"

	postgresconfig "example.com/app/internal/features/order/adapter/outgoing/persistence/postgres/config"
	"example.com/app/internal/features/order/application/port/outgoing"
)

func Package(config postgresconfig.Config) func(do.Injector) {
	return do.Package(
		do.Eager(config),
		do.Lazy(provideRepository),
		do.Bind[*Repository, outgoing.OrderRepository](),
	)
}
```

This registration:

* provides the externally constructed configuration;
* lazily constructs the repository;
* exposes the concrete repository through the outgoing port consumed by the
  application.

The provider resolves dependencies and calls the normal constructor:

```go
func provideRepository(i do.Injector) (*Repository, error) {
	return NewRepository(
		do.MustInvoke[DB](i),
		do.MustInvoke[postgresconfig.Config](i),
	), nil
}
```

The implementation constructor MUST remain independent from `samber/do`:

```go
func NewRepository(
	db DB,
	config postgresconfig.Config,
) *Repository {
	return &Repository{
		db:     db,
		config: config,
	}
}
```

### Packages Without Configuration

A runtime package that does not require configuration MAY export a package
variable:

```go
var Package = do.Package(
	do.Lazy(provideHandler),
)
```

Pure domain packages MUST NOT export empty dependency-registration packages.

### Feature Registration

The root feature package composes registrations exported by its children:

```go
package order

import (
	"github.com/samber/do/v2"

	"example.com/app/internal/features/order/adapter/incoming/httpapi"
	orderpostgres "example.com/app/internal/features/order/adapter/outgoing/persistence/postgres"
	"example.com/app/internal/features/order/application/service"
	"example.com/app/internal/features/order/application/usecase/placeorder"
	orderconfig "example.com/app/internal/features/order/config"
)

func Package(config orderconfig.Config) func(do.Injector) {
	return do.Package(
		orderpostgres.Package(config.Postgres),
		placeorder.Package,
		service.Package,
		httpapi.Package(config.HTTPAPI),
	)
}
```

The root feature package is a composition boundary.

Child packages MUST NOT import the root feature package.

### Registration Rules

* Package registration MUST remain in `package.go`.
* Configurable packages MUST receive their own configuration type externally.
* Use `do.Eager` for externally constructed configuration values.
* Prefer `do.Lazy` for runtime services.
* Use `do.Bind` to expose concrete implementations through application ports.
* Providers may use the injector to resolve dependencies.
* Constructors and business implementation files MUST NOT use `do.Injector`.
* The root feature package composes registrations exported by child packages.
* Executables compose feature, workflow, and shared runtime packages.
* Dependency registration MUST preserve the dependency direction defined by
  this skill.
* Registration packages MUST NOT be used to bypass feature isolation or access
  another feature's internal implementation.

Use specialized dependency-injection skills for advanced lifecycle, scopes,
overrides, shutdown, and testing behavior.

## Composition Root

Each executable under `cmd` selects the features, workflows, adapters, and
shared runtime packages it requires.

The composition root owns:

* application configuration loading;
* dependency assembly;
* executable startup;
* lifecycle and shutdown.

It MUST NOT contain business logic.

## Implementation Workflow

When implementing architecture-related work:

1. Identify the owning business feature or workflow.
2. Define or update the domain behavior.
3. Add or update the use-case slice.
4. Add only the ports required by real consumers.
5. Implement the necessary adapters.
6. Add package-owned configuration where required.
7. Register dependencies at the package boundary.
8. Compose the feature or workflow from the executable.
9. Inspect imports for boundary violations.

## Expected Change Surface

A feature-local use case SHOULD primarily affect:

```text
features/<feature>/domain/                         # when business rules change
features/<feature>/application/usecase/<action>/
features/<feature>/application/port/               # only when required
features/<feature>/application/service/            # only when used
features/<feature>/adapter/                        # required adapters only
features/<feature>/config/                         # required configuration only
features/<feature>/package.go                      # composition only
```

A cross-feature workflow SHOULD primarily affect:

```text
workflows/<workflow>/
features/<feature>/application/port/incoming/      # only if capability is missing
cmd/<executable>/                                  # composition only
```

Changes outside the expected surface require a clear architectural reason.

## Architecture Validation

Before completing a change, verify:

* [ ] Code is organized around business features.
* [ ] Feature and workflow names use business terminology.
* [ ] New behavior has one clear owner.
* [ ] No feature imports another feature.
* [ ] No feature imports a workflow.
* [ ] Workflows depend on feature incoming ports.
* [ ] Workflows do not depend on another feature's outgoing ports or repositories.
* [ ] Domain code remains independent from infrastructure.
* [ ] Application code does not import adapter implementations.
* [ ] Adapter-specific models do not leak inward.
* [ ] Config dependencies are acyclic.
* [ ] Shared packages do not depend on concrete features or workflows.
* [ ] Dependency injection is absent from business logic.
* [ ] Package registration remains in `package.go`.
* [ ] Configurable packages receive their own config externally.
* [ ] The change remains inside the smallest reasonable feature or workflow.
