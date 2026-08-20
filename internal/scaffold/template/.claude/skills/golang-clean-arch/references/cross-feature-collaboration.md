# Cross-Feature Collaboration

Read this reference when behavior needs data or capabilities from more than one
feature, when choosing between synchronous and asynchronous collaboration, or
when several features use the same external system.

The main skill's feature-isolation rule remains absolute: a feature does not
import another feature's domain, application implementation, outgoing ports,
repositories, adapters, or persistence models.

## Choose by Consistency and Ownership

Use the smallest collaboration mechanism that preserves feature ownership.
There is no universal preference order.

| Need | Prefer |
| --- | --- |
| One synchronous business operation coordinates several feature capabilities | Workflow depending on feature incoming ports |
| Driver or executable only needs to sequence independent calls | Composition at the executable or driver boundary |
| Consumer needs local, query-optimized data and can tolerate staleness | Consumer-owned read model or projection |
| Producer announces a completed fact and consumers react independently | Domain event with consumer-owned translation |
| Several features need the same stable concept with identical meaning and lifecycle | Genuinely shared contract or shared domain concept |

Do not use a shared contract merely to avoid defining an explicit workflow,
projection, or translation boundary.

## Synchronous Workflows

Place a multi-feature business operation under `internal/workflows/<name>` when
the coordination itself has business meaning.

A workflow:

* depends only on participating features' incoming ports;
* owns its input, output, errors, coordination, compensation, and recovery;
* may own outgoing ports for workflow state or infrastructure;
* does not import feature domain types, use-case implementations, outgoing
  ports, repositories, or adapters.

The composition root supplies concrete incoming-port implementations. Features
remain unaware of the workflow.

Use executable or driver composition instead when the sequence is merely a
delivery concern and has no cross-feature business rules.

## Consumer-Owned Read Models

A feature that repeatedly needs another capability's data may maintain its own
projection rather than synchronously querying or importing the owner.

```text
owning feature emits fact
        ↓
consumer translates event contract
        ↓
consumer-owned projection
        ↓
consumer use case queries local outgoing port
```

The consumer owns:

* the projection schema and storage;
* freshness expectations;
* idempotency and replay handling;
* reconciliation or rebuild behavior;
* translation into its own terminology.

The projection must not store or expose the producer's domain or ORM types as
the consumer's model. Account explicitly for eventual consistency.

Scheduled synchronization is acceptable when events are unavailable and the
freshness requirement permits it. Keep the same ownership and translation
rules.

## Events and Anti-Corruption Translation

Treat an event as a published contract describing a completed fact, not as the
producer's internal domain object.

At the consuming boundary:

1. deserialize into a wire-contract type owned by the incoming consumer
   adapter;
2. validate contract version and required transport fields;
3. translate into consumer-owned application input or domain concepts;
4. invoke the consumer's incoming port or use-case handler.

This anti-corruption translation prevents producer terminology, schema changes,
and implementation details from becoming the consumer's domain model.

Design consumers for duplicate delivery and out-of-order events when the
messaging system permits them. Use specialized messaging, concurrency, and
testing guidance for the mechanics.

## Shared External Systems

Sharing a Stripe, Jira, email, storage, or messaging connection does not justify
sharing a feature gateway.

Use three boundaries:

```text
internal/shared/.../integrations/<system>/
    raw client: auth, retries, pagination, rate limits, wire types

internal/features/<feature>/application/port/outgoing/
    feature-owned business capability

internal/features/<feature>/adapter/outgoing/<system>/
    translation from business capability to raw client
```

For example, checkout may define `PaymentGateway` while subscriptions defines
`RecurringBillingGateway`. Both adapters may use the same raw Stripe client,
but neither feature imports the other's port or adapter.

The raw client must remain technical:

* no feature-specific names;
* no business decisions or policies;
* no feature domain types;
* no feature-specific persistence or caching semantics.

Connection pools, authentication, wire protocols, retry behavior, pagination,
and rate limiting are appropriate shared-client responsibilities.

## Shared Concepts

Share a contract or domain concept only when all consumers agree on its meaning,
rules, and lifecycle. Stable identifiers, money, and time abstractions may
qualify. Feature entities, repositories, gateways, and use-case outputs usually
do not.

Prefer consumer-owned translation or limited duplication over a shared model
that silently couples feature evolution.

## Collaboration Review Checklist

* [ ] The collaboration has one explicit owner: workflow, driver, or consuming feature.
* [ ] No feature imports another feature.
* [ ] Workflows use only feature incoming ports.
* [ ] Projections are owned and queried by the consuming feature.
* [ ] Event contracts are translated into consumer-owned types.
* [ ] Eventual consistency, replay, and duplicate delivery are addressed where applicable.
* [ ] Shared raw clients contain technical behavior only.
* [ ] Each feature owns its business abstraction over a shared external system.
* [ ] Shared concepts have identical meaning and lifecycle for every consumer.
