# EVAL.md — Evaluation runbook for the coordination example

Instructions for an agent (human or LLM) to **test, evaluate and verify** that
`pkg/coordination` slot leasing behaves as intended when several replicas of
the same app race for slots in one PostgreSQL table under Docker.

Everything runs from THIS directory (`pkg/coordination/example`) via the
`docker:*` tasks in `Taskfile.yml` — those tasks are the interface. Record
every check's output (tee to files or a report) and fill the report template
at the end of this document. Run the checks in order; the suite is idempotent
if you always start with the reset in S0.

## S0 — Pre-flight + suite reset

```bash
# Engine sanity (the ambient Docker API; set DOCKER_HOST if your container
# runtime exposes a Docker-compatible socket on a non-default path)
docker compose version                     # Compose v2.x
task --list                                # docker:build/up/scale/... listed

# ALWAYS reset slot state + generations before the suite:
task docker:down:volumes                   # removes containers, network, volume
docker volume ls --format '{{.Name}}' | grep coordination-example   # expect: (empty)
```

## Note 1 — non-default container runtimes

Testcontainers and the Docker CLI resolve the Docker API the same way:
`DOCKER_HOST` (if set) wins, then the default `docker.sock`. Nothing in the
suite or the tasks is bound to a specific engine; whichever runtime answers on
the resolved socket runs the demo and the integration tests.

## E1 — Static verification (library + example)

From `pkg/coordination` (one level up):

```bash
go build ./...                                   # exit 0
go vet ./...                                     # exit 0
go test -count=1 -race ./...                     # unit + integration backends
golangci-lint run -c ../../.golangci.yml ./...   # 0 issues
```

**PASS** = all four succeed.

The `postgres/` package also runs **SQL integration tests** — the database comes
from **testcontainers-go** (https://golang.testcontainers.org/), module
`testcontainers-go/modules/postgres`:
`COORDINATION_TEST_DATABASE_URL` wins if set; otherwise a throwaway
`postgres:18-alpine` container is started through the ambient Docker API and
terminated after the suite; with no reachable runtime the tests skip
(`COORDINATION_TEST_DATABASE_URL` set but unreachable ⇒ suite fails loudly).

**PASS** = `ok` for `coordination` and `postgres`; when the container path is
used, coverage for the backend lands around **87%** (`go test -cover ./...`).

## E2 — Baseline: N replicas claim N slots (default 3)

```bash
task docker:up                     # builds + starts postgres, cleanup, 3 coordinators
sleep 45                           # claims settle (claims + NATS ready ≈ 10-20s)
task docker:ps                     # 5 containers: postgres, cleanup, coordinator-1..3
task docker:slots                  # the state machine
for c in 1 2 3; do
  docker logs coordination-example-coordinator-$c 2>&1 | grep -c "embedded NATS started"
done
```

**PASS**
- `docker:slots` shows **exactly 3 rows**, each `state='active'`, `generation ≥ 1`,
  `owner_id` non-null, `route_url` = `nats-route://<ip>:<port>`, `lease_until` in the future.
- Each coordinator logged exactly **one** `embedded NATS started server=nats-<slot>` line
  (peer counts 0/1/2 depending on claim order).
- No `panic` occurrences in any coordinator log.

## E3 — Surplus replicas run client-only

```bash
task docker:scale N=5              # two replicas MORE than slots
sleep 30
task docker:slots                  # MUST still show exactly 3 rows
for c in 1 2 3 4 5; do
  n=$(docker logs coordination-example-coordinator-$c 2>&1 | grep -c "running as client-only")
  s=$(docker logs coordination-example-coordinator-$c 2>&1 | grep -c "embedded NATS started")
  echo "coordinator-$c: servers_started=$s client_only_retries=$n"
done
```

**PASS** — after a fresh baseline (E2, no rollouts yet):
- Exactly **2** containers with `servers_started=0` (coordinator-4, coordinator-5),
  each with `client_only_retries ≥ 1`.
- The 3 slot holders unchanged: rows still `active`, renewing (compare `lease_until`
  between two samples ~5s apart → value must advance).

## E4 — Graceful rollout → generation bump

```bash
task docker:slots                                           # BEFORE: note the row whose
                                                            # holder you will roll
holder=$(docker logs coordination-example-coordinator-1 2>&1 |
         grep -m1 "embedded NATS started" | sed -E 's/.*server=(nats-[0-9]).*/\1/')
cid=$(docker compose ps -q coordinator | head -1)
cname=$(docker inspect "$cid" --format '{{.Name}}')

task docker:rollout                # graceful stop (SIGTERM -> Stop -> Release) + start
sleep 25
task docker:slots                  # AFTER
docker logs "$cname" 2>&1 | grep -c "embedded NATS started"   # expect: 2 (old + new proc)
docker logs "$cname" 2>&1 | grep -ic panic                                    # expect: 0
```

**PASS**
- The rolled instance's slot row now has **`generation = before + 1`** (identity reused,
  fencing counter advanced), `state='active'`, fresh lease, new `owner_id`.
- The rolled container shows **2** NATS starts, **0** panics (SIGTERM fully released
  the slot; `docker stop` grace 20s ≥ StopTimeout 10s). *(Engine note: some
  container runtimes rotate `docker logs` for a manually stopped+started
  container, showing only the new process — the DB row is the source of truth
  for the re-claim.)*
- Optional (nice): sample `task docker:slots` every 2s during the rollout and
  capture the intermediate `available` row before the re-claim. The re-claim race
  may be won by a surplus client-only replica — that is correct behavior; re-identify
  holders per E5.

## E5 — Crash + fencing lifecycle (lease expiry, quarantine, recycle)

Identify the CURRENT holder by `route_url` → container IP — after a rollout the slot
may belong to any replica (E4's re-claim is a race; don't assume):

```bash
# map: for each active row find the container whose IP is in route_url
row=$(docker compose exec -T postgres psql -U coordinator -tAc \
  "SELECT route_url FROM nats_slots WHERE state='active' ORDER BY slot_id LIMIT 1")
holder=""
for cid in $(docker compose ps -q coordinator); do
  ip=$(docker inspect "$cid" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}')
  case "$row" in *"$ip"*) holder=$(docker inspect "$cid" --format '{{.Name}}') ;; esac
done
echo "holder: $holder (row: $row)"      # must not be empty

# Kill a slot HOLDER (SIGKILL: no release path runs)
docker kill "$holder"
sleep 60;  task docker:slots
sleep 60;  task docker:slots
```

Timeline expectations (TTL 30s, cleanup loop 10s, suspect cooldown 60s, retry 5s —
allow ±15s drift):

| moment | killed slot | other slots |
|---|---|---|
| t0 | `active`, gen N, fresh lease | active, renewing |
| ~t+55s | **`suspect`** — lease frozen at expiry, owner preserved | active, renewing |
| ~t+110s | **`active`, gen N+1** — recycled after cooldown and claimed (possibly by a surplus client-only replica, which then starts its own NATS server) | active, renewing |

**PASS** — `suspect` observed, then re-claim with `generation` incremented; healthy
slots never flip through `suspect` (the cleanup only touches expired leases).
The killed container self-restarts via `restart: unless-stopped` and (if it lost the
race) sits in client-only mode until a slot frees.

## E6 — NATS traffic through the claimed cluster

```bash
task docker:client
```

**PASS** — the task starts a subscriber + publisher (`nats-box`) on the compose
network; the subscriber must print the exact published message
(`[#1] Received on "demo.e2e"` + `hello-from-docker-demo`).

## E7 — Teardown + state retention

```bash
task docker:down                   # stops stack, KEEPS the postgres volume
docker volume ls --format '{{.Name}}' | grep coordination-example    # volume still listed
task docker:down:volumes           # full reset for the next suite run
```

**PASS** — down removes all containers + network, `ps -a` prints nothing; the volume
survives `down` and is removed by `down:volumes`.

## Invariants to assert on ANY `task docker:slots` capture

1. Row count == number of slots seeded by `sql/schema.sql` (3 by default — the cap is
   the data, not the code).
2. `generation` per slot is monotonically non-decreasing (until `down:volumes` resets it).
3. Constraint holds: `available` ⇒ NULL owner/lease/route; otherwise owner + lease non-null.
4. Surplus replicas ⇒ client-only; every `active`/`starting` row ⇒ exactly one NATS server.

## Failure triage

| symptom | cause | check |
|---|---|---|
| slot stuck `active` with expired lease, never `suspect` | cleanup sidecar dead | `docker logs coordination-example-cleanup-1` |
| `panic: close of nil channel` (shutdownEventing) during stop | app + nats embedded SIGTERM handler racing (`Server.Start` installs it unless `opts.NoSigs`) | grep coordinator logs for `panic`; confirm `NoSigs: true` in `example/boot.go` |
| nats FTL `JetStream cluster requires configured routes` | first claimant started clustered with 0 peers | confirm standalone-first behavior (`peers=0` log line, no Cluster opts) |
| postgres refuses to start | PG 18+ expects the mount at `/var/lib/postgresql` | inspect compose `volumes` |
| `runner: slot renew failed` warnings in app logs | lease renewals failing against the backend | backend probe (see `logs` at that moment; slots table forensics) |

## Environment (validated reference results)

Validated end-to-end with Compose v2 (v5.7.0) against a Docker-API-compatible
container runtime — see the report below for numbers.

## Report template (fill and ship alongside the evidence files)

```
# EVAL report — coordination example
Engine:          container runtime (<compose version>, <engine version>)
Host:            <os / kernel>
Suite reset:     down -v before E2 (fresh DB)  [yes/no]

| Check | Status (PASS/FAIL) | Key evidence (numbers/rows) | Notes |
|-------|--------------------|-----------------------------|-------|
| E1 static  |  | build/vet/test/lint results |  |
| E2 claim   |  | 3 rows active+gen, NATS lines per coordinator |  |
| E3 surplus |  | 2 client-only, rows still 3 |  |
| E4 rollout |  | slot gen before->after, NATS starts, panics |  |
| E5 crash   |  | suspect at t+x, gen++ at t+y |  |
| E6 traffic |  | pub/sub message observed |  |
| E7 teardown|  | volume kept on down, removed by down:volumes |  |

Anomalies/notes:
- <intermediate states observed, drift, anything failing PASS>
```
