# Cross-Feature Collaboration

Use this reference for business applications when behavior spans more than one
feature.

The business feature isolation rule remains: one feature does not import another
feature's domain, application implementation, outgoing ports, repositories,
adapters, or persistence models.

## Choose by Ownership and Consistency

| Need | Prefer |
| --- | --- |
| One synchronous business operation coordinates several feature capabilities | Workflow depending on incoming ports |
| Executable only sequences independent operations | Composition at executable/driver boundary |
| Consumer needs local query-optimized data and tolerates staleness | Consumer-owned projection/read model |
| Producer announces a completed fact | Event with consumer-owned translation |
| Several features share a truly identical stable concept | Shared contract/domain concept |

## Synchronous Workflows

Place cross-feature business coordination under `internal/workflows/<name>` when
the coordination itself has business meaning.

A workflow:

- depends on participating features' incoming capabilities;
- owns coordination, errors, compensation, and recovery;
- may define outgoing ports for workflow-owned infrastructure;
- does not import feature domains, use-case implementations, outgoing ports,
  repositories, or adapters.

Use executable composition instead when the sequence is merely operational and
contains no cross-feature business rule.

## Consumer-Owned Projections

A feature that repeatedly needs another feature's data may maintain its own read
model.

```text
owner emits fact
      ↓
consumer translates contract
      ↓
consumer-owned projection
      ↓
consumer queries local capability
```

The consumer owns schema, freshness, idempotency, replay, reconciliation, and
translation into its own terminology.

## Events

Treat events as published contracts describing completed facts, not as producer
internal models.

Consumers deserialize and validate the event contract, translate it into their
own language, and then invoke their own application boundary.

Design for duplicate or out-of-order delivery when the messaging system permits
it.

## Shared External Systems

Sharing an external vendor does not justify sharing a feature-specific gateway.

Prefer:

```text
shared raw technical client
          ↑ used by
feature-specific adapter
          ↑ implements
feature-owned outgoing capability
```

The raw client may own authentication, retries, pagination, rate limiting,
connection handling, and wire types. It MUST NOT own feature-specific business
rules.

## Shared Concepts

Share a concept only when all consumers agree on its meaning, rules, and
lifecycle.

Prefer consumer-owned translation or limited duplication over a shared model
that silently couples feature evolution.
