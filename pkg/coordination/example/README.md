# coordination example

Five replicas of **one** app image. Only **three** of them run an embedded NATS
server. They decide who runs what with **one PostgreSQL table** — no Redis,
no Kubernetes API, no StatefulSet, no second NATS deployment.

```
        ┌─────────────────── same app image, 5 replicas ───────────────────┐
        │                                                                  │
   replica 1        replica 2        replica 3        replica 4     replica 5
 ┌ ─────────────┐ ┌ ─────────────┐ ┌ ─────────────┐ ┌ ────────┐  ┌ ────────┐
 │ nats-0  ◄─┐  │ │ nats-1  ◄─┐  │ │ nats-2  ◄─┐  │ │ client  │  │ client  │
 │ NATS server│  │ │ NATS server│  │ │ NATS server│  │ only    │  │ only    │
 └─────────┬──┘ └─────────┬──┘ └─────────┬──┘ └────┬────┘  └────┬────┘
           │  NATS cluster (3 nodes)     │         │            │
           ▼                             ▼         ▼            ▼
        ┌─────────────────────── one nats_slots table ───────────────────────┐
        │  slot 0: gen 2, active, owner aa57…   slot 1: gen 1  slot 2: gen 1 │
        └──────────────────────────────── postgres ──────────────────────────┘
```

## The problem

If your app embeds NATS (so you don't have to run NATS as a second service),
every replica wants to run *its own* NATS server:

- **Too many servers.** 40 replicas would mean a 40-node NATS cluster.
- **No stable identity.** A NATS server needs to be *nats-0*, *nats-1*, … with
  a fixed JetStream store and a fixed route address. Pods get random names and
  random IPs, so you reach for a **StatefulSet** + headless service just to get
  stable identities.
- **Clusters need seed lists.** A NATS server must be told upfront which peers
  to dial (`Routes: […])`); with per-pod embedded servers that list has to
  track *who is currently alive* — normally yet another piece of plumbing.
- **Rollouts are painful.** Every restart can duplicate or split the cluster,
  and a crashed server may keep answering traffic you thought was gone (split
  brain).
- **The usual fixes add infrastructure.** Distributed leases are typically done
  with **Redis, etcd, ZooKeeper** or the **Kubernetes API** (lease objects) —
  plus coordination logic of your own.

## The solution: the slots live in Postgres

Coordination says: a logical NATS identity is a **row** in a small table, and
ownership is a **lease on that row**. Nothing else in the stack is special:

```
nats_slots: slot_id | generation | state | owner_id | lease_until | route_url
             0            0      available        (empty)
             1            0      available        (empty)
             2            0      available        (empty)
```

- **Claim:** `SELECT … FOR UPDATE SKIP LOCKED LIMIT 1` picks a free row, bumps
  its `generation`, writes this replica's random `owner_id`, sets a
  `lease_until` (30s) deadline and *starts the embedded NATS server as
  `nats-<slot_id>`*.
- **Keep:** every 5s the replica renews the lease. Losing it (connection lost,
  restart, too slow) is fatal for that ownership.
- **Release:** on graceful shutdown the server stops **before** the slot is
  returned to the pool — the next claimant finds a clean slot.
- **The cap is data, not config:** 3 rows ⇒ at most 3 NATS servers. 40 replicas
  ⇒ still exactly 3. Add a row, get one more server.
- **Only dependency: Postgres.** No Redis, no etcd, no ZooKeeper, no Kubernetes
  API, no StatefulSet, no headless service, no separate NATS deployment.
  The app deploys as a plain replica set anywhere Postgres is reachable.

### The seeder: the table is also the seed list

A NATS server can't start clustered into thin air — it needs its peers up
front: a **seed list** of `nats-route://…` addresses. In Kubernetes this is
solved with another piece of plumbing (a headless service + per-pod DNS), or
with a static config — and embedded servers have neither.

Here, the claim **advertises each replica's route address** with its slot
(`route_url = nats-route://<POD_IP>:6222`), so the table *is* the seed registry:

```
replica claims slot 1
        │
        ▼
Peers(slot 1)          ──►  active rows in nats_slots:
        │                    slot 0 → nats-route://10.89.5.3:6222
        ▼
nats-0 starts with Routes: [nats-route://10.89.5.3:6222]
        ▼
the embedded server joins the existing cluster; slot 1 activates
```

- The seed list is read **at every claim** (every generation), so it is always
  fresh — and it only contains rows with a **live lease**, because expired
  leases are excluded from `Peers()`. Dead peers never poison the list.
- Each replica's address was written to the table *at claim time*, so the
  table converges without any DNS: rows appear, activate, expire — the seed
  list follows automatically.
- First claimant edge case (with a fresh table, or when scaling out after a
  reset): no peers yet ⇒ NATS starts **standalone** for that slot. Its own
  route address is already registered, so peers will try to reach it — but a
  standalone start never adds cluster listeners later; the server re-joins as
  a clustered node on its **next generation** (after release/re-claim with
  active peers). The eval logs show the benign "route connection refused"
  attempts from peers toward that one server in the meantime.

Because ownership is checked against `generation + owner_id` on **every** write
(renew, activate, release), a stale owner can never overwrite a new one — the
row is a one-way ratchet:

- a crashed replica's lease simply expires → it is quarantined as `suspect`,
- replicas that come *back* to life (e.g. after a network partition) fail their
  renewals with `ErrLost`, shut their server down, and re-claim like anyone else.

**Rollout generations.** Each claim is the *next generation* of that slot's
server: gen 1 (first owner) → release → gen 2 (next owner) → … The "rollout"
story: kill/replace/upgrade any replica whenever you like — the identity
`nats-0` survives, the generation counter proves which owner epoch is current,
JetStream storage is re-created for the new owner, and the rest of the fleet
picks up the peers from the table.

## The 5-replicas / 3-servers demo

Start 3 replicas against 3 slots, then scale to 5 (still 3 servers: the two
extra replicas keep asking every 5s and log *client-only* mode):

```bash
task docker:up                 # 3 replicas → 3 NATS servers
task docker:scale N=5          # 5 replicas → still 3 servers, 2 client-only
task docker:logs               # observe: "embedded NATS started …" vs
                               # "no NATS slot: running as client-only"
task docker:slots              # watch the state machine live
```

What you should see in the table:

| slot | generation | state | meaning |
|---|---|---|---|
| 0,1,2 | 1 | `active` | three replicas claimed and started their NATS servers |
| (none) | — | `available` | every new replica retries: *client-only* |

**A graceful rollout** (`task docker:rollout` — stops one replica, restarts it):
the stopping replica releases its slot (SIGTERM → stop server → release), and
the slot is re-claimed with `generation + 1`:

```
slot 0:  gen 1 active owner af01… route 10.89.5.3     ← before
slot 0:  gen 2 active owner a4dc… route 10.89.5.7     ← after: next generation
```

Note the re-claim is a **race**: any claiming replica can win — the container
that wins the slot runs the next generation of `nats-0`, whoever it is.

**A crash** (kill the replica that holds a slot — find it via `nats-route://<ip>`
in `task docker:slots` ↔ the container's IP: killing a client-only replica, as
the eval proved, is correctly a no-op). No release happens. The lease expires
(≤ 30s), the cleanup sidecar quarantines the row as `suspect`, and after a 60s
cooldown the demo returns it to `available` so the fleet self-heals — the next
claim is `gen + 1`:

```
t0      slot 0: gen 2 active   owner aa57…        ← holder is SIGKILLed
t+55s   slot 0: gen 2 suspect  owner aa57…        ← quarantined (frozen)
t+110s  slot 0: gen 3 active   owner 376c… new    ← recycled → reclaimed
```

The two client-only replicas also keep racing during all of this — exactly what
makes scale-up work: **run more replicas, they either become servers or serve
as clients**, nothing else to reconfigure.

Talk to the running cluster like any NATS:

```bash
task docker:client      # pub/sub round trip through the embedded servers
```

Clean up:

```bash
task docker:down        # keep state     (generations preserved)
task docker:down:volumes # reset everything
```

## Layout

| file | role |
|---|---|
| `main.go` | wires the bootstrapper: `PostgresProvider` → `NATSServerProvider` → `NATSClientProvider` |
| `boot.go` | the three boot providers (DI via `pkg/boot` + `samber/do`) + the embedded NATS `coordination.Service` |
| `sql/schema.sql` | the slot table (3 slots) — the ONLY coordination state anywhere |
| `Taskfile.yml` | `docker:*` tasks used above |
| `docker-compose.yml` | postgres + N `coordinator` replicas + the (demo-only) cleanup sidecar |
| `Dockerfile` | multi-stage build of this example |
| `EVAL.md` | the verification runbook used to validate everything above |

## Library API (what the example uses)

```go
backend := postgres.New(pool)                       // coordination.Backend over SQL
runner  := coordination.Runner{Backend: backend,
          LeaseTTL: 30 * time.Second,
          RenewEvery: 5 * time.Second,
          StopTimeout: 10 * time.Second}

err := runner.RunOnce(ctx, coordination.ClaimRequest{
    Owner: "random-per-process",
    RouteURL: "nats-route://10.0.5.7:6222",
}, natsService)     // claims ONE slot, runs the service until ctx is done,
                    // stops it and releases the slot
```

`ErrNoSlot` (all slots taken → client-only mode) and `ErrLost` (ownership lost —
renew/activate/release refused → stop and re-claim) are the two signals to
handle. Everything else is the runner's job.

## Production notes (honest bits)

- The `cleanup` sidecar that *recycles* `suspect` rows into `available` is
  **demo-only**. The design intentionally says: never hand an expired slot back
  without independently verified storage fencing. In production, park suspect
  slots and recycle them after verification (or keep them parked — the fleet
  runs on the remaining slots).
- JetStream storage here is per-container and **ephemeral**; a slot's data does
  not survive a move to another replica. Persist/n mount the store per slot and
  verify the fencing story if your data must outlive a roll.
- NATS auth/TLS is not configured in this demo.
- `example/sql/schema.sql` says it plainly: **never truncate or drop
  `nats_slots` during upgrades** — it is your cluster's identity record.

## Verify

`EVAL.md` is the executable evidence: static checks, the
claim/client-only/rollout/crash-fencing/traffic checks, and a report template
plus the evidence files it points at.
