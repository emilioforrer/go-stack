package postgres

// Integration tests exercise the SQL backend against a real PostgreSQL.
//
// The database comes from testcontainers-go (https://golang.testcontainers.org/):
//  1. COORDINATION_TEST_DATABASE_URL — an explicitly provided DSN wins; a
//     connect failure fails the suite (explicit intent).
//  2. otherwise a throwaway postgres:18-alpine container is started via
//     testcontainers against the ambient Docker API, and terminated after the
//     suite.
//  3. without a reachable Docker runtime, every test skips.
//
// The DDL below mirrors example/sql/schema.sql; keep the two in sync.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/emilioforrer/go-stack/pkg/coordination"
	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	envTestDSN     = "COORDINATION_TEST_DATABASE_URL"
	containerImage = "postgres:18-alpine"
)

const slotsSchema = `CREATE TABLE IF NOT EXISTS nats_slots (
  slot_id integer PRIMARY KEY CHECK (slot_id >= 0),
  generation bigint NOT NULL DEFAULT 0,
  owner_id text,
  state text NOT NULL DEFAULT 'available' CHECK (state IN ('available','starting','active','suspect')),
  lease_until timestamptz,
  route_url text,
  CHECK ((state='available' AND owner_id IS NULL AND lease_until IS NULL AND route_url IS NULL)
    OR (state<>'available' AND owner_id IS NOT NULL AND lease_until IS NOT NULL))
)`

var (
	testPool  *pgxpool.Pool
	testSetup string // why the pool is absent (env/engine/unreachable), for skip messages
)

func TestMain(m *testing.M) {
	setupCtx, cancelSetup := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancelSetup()

	pool, terminate, setupErr := setupDatabase(setupCtx)
	switch {
	case setupErr == nil:
		testPool = pool
	case os.Getenv(envTestDSN) != "":
		// an explicitly requested database that does not work fails the suite
		fmt.Fprintf(os.Stderr, "integration: %s unreachable: %v\n", envTestDSN, setupErr)
		os.Exit(1)
	default:
		testSetup = setupErr.Error()
	}

	code := m.Run()

	if testPool != nil {
		testPool.Close()
	}
	if terminate != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = terminate(shutdownCtx)
	}
	os.Exit(code)
}

// setupDatabase resolves the integration database. terminate is nil when no
// container was started by this suite.
func setupDatabase(ctx context.Context) (pool *pgxpool.Pool, terminate func(context.Context) error, err error) {
	if dsn := os.Getenv(envTestDSN); dsn != "" {
		pool, err := connectPool(ctx, dsn)
		return pool, nil, err
	}

	ctr, err := tcpostgres.Run(ctx, containerImage,
		tcpostgres.WithDatabase("coordination_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		// Without a reachable Docker runtime the suite skips instead of
		// failing; point COORDINATION_TEST_DATABASE_URL at an existing
		// PostgreSQL to exercise the backend anyway.
		return nil, nil, fmt.Errorf("start %s container: %w", containerImage, err)
	}

	cs, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, fmt.Errorf("container connection string: %w", err)
	}

	pool, err = connectPool(ctx, cs)
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, nil, err
	}
	return pool, func(target context.Context) error { return ctr.Terminate(target) }, nil
}

func connectPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if _, err := pool.Exec(ctx, slotsSchema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := seedSlots(ctx, pool, 3); err != nil {
		pool.Close()
		return nil, fmt.Errorf("seed slots: %w", err)
	}
	return pool, nil
}

// db returns the shared pool and leaves a clean slot table for the test.
// Skipping (instead of failing) is reserved for environments without any
// container runtime; an explicit DSN that is unreachable exits the suite.
func db(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("integration tests need PostgreSQL: " + testSetup)
	}
	if err := resetSlots(context.Background(), 0); err != nil {
		t.Fatalf("reset slots: %v", err)
	}
	t.Cleanup(func() { _ = resetSlots(context.Background(), 0) })
	return testPool
}

func resetSlots(ctx context.Context, count int) error {
	if _, err := testPool.Exec(ctx, `TRUNCATE nats_slots`); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	return seedSlots(ctx, testPool, count)
}

func seedSlots(ctx context.Context, pool *pgxpool.Pool, count int) error {
	for id := 0; id < count; id++ {
		if _, err := pool.Exec(ctx, `INSERT INTO nats_slots(slot_id) VALUES ($1) ON CONFLICT DO NOTHING`, id); err != nil {
			return err
		}
	}
	return nil
}

func mustClaim(t *testing.T, ctx context.Context, pool *pgxpool.Pool, owner string) coordination.Assignment {
	t.Helper()
	b := New(pool)
	a, err := b.Claim(ctx, coordination.ClaimRequest{Owner: owner, RouteURL: "nats-route://127.0.0.1:6222"}, 30*time.Second)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	return a
}

// rowState reads the raw row for assertions.
func rowState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, slot int) (state string, generation int64, owner string, leasePending bool) {
	t.Helper()
	err := pool.QueryRow(ctx, `SELECT state, generation, coalesce(owner_id,''), coalesce(lease_until > clock_timestamp(), false) FROM nats_slots WHERE slot_id=$1`, slot).
		Scan(&state, &generation, &owner, &leasePending)
	if err != nil {
		t.Fatalf("read row: %v", err)
	}
	return state, generation, owner, leasePending
}

// --- backend behaviour ---

func TestBackendClaimActivateRenewReleaseCycle(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 2); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)

	a := mustClaim(t, ctx, pool, "owner-a")
	if a.Slot != 0 || a.Generation != 1 || a.Owner != "owner-a" {
		t.Fatalf("unexpected assignment: %+v", a)
	}
	if state, gen, owner, leasePending := rowState(t, ctx, pool, a.Slot); state != "starting" || gen != 1 || owner != "owner-a" || !leasePending {
		t.Fatalf("claimed row: state=%s gen=%d owner=%s leasePending=%v", state, gen, owner, leasePending)
	}

	if err := backend.Activate(ctx, a); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if state, _, _, _ := rowState(t, ctx, pool, a.Slot); state != "active" {
		t.Fatalf("want active, got %s", state)
	}

	var leaseBefore, leaseAfter time.Time
	if err := pool.QueryRow(ctx, `SELECT lease_until FROM nats_slots WHERE slot_id=$1`, a.Slot).Scan(&leaseBefore); err != nil {
		t.Fatal(err)
	}
	if err := backend.Renew(ctx, a, 30*time.Second); err != nil {
		t.Fatalf("renew: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT lease_until FROM nats_slots WHERE slot_id=$1`, a.Slot).Scan(&leaseAfter); err != nil {
		t.Fatal(err)
	}
	if !leaseAfter.After(leaseBefore) {
		t.Fatalf("renew must extend lease_until: before=%v after=%v", leaseBefore, leaseAfter)
	}

	if err := backend.Release(ctx, a); err != nil {
		t.Fatalf("release: %v", err)
	}
	state, generation, owner, leasePending := rowState(t, ctx, pool, a.Slot)
	if state != "available" || owner != "" || leasePending {
		t.Fatalf("released row: state=%s gen=%d owner=%q leasePending=%v", state, generation, owner, leasePending)
	}

	// a fresh claim reuses the same slot with a bumped generation
	a2 := mustClaim(t, ctx, pool, "owner-b")
	if a2.Slot != a.Slot || a2.Generation != 2 {
		t.Fatalf("re-claim want same slot gen 2, got %+v", a2)
	}
}

func TestBackendClaimContentionSkipLocked(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 2); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)

	const racers = 4
	for round := 0; round < 5; round++ {
		if err := resetSlots(ctx, 2); err != nil {
			t.Fatal(err)
		}
		results := make(chan error, racers)
		for i := 0; i < racers; i++ {
			go func() {
				_, err := backend.Claim(ctx, coordination.ClaimRequest{Owner: "racer", RouteURL: "nats-route://127.0.0.1:6222"}, 30*time.Second)
				results <- err
			}()
		}
		won, noSlot := 0, 0
		for i := 0; i < racers; i++ {
			err := <-results
			switch {
			case err == nil:
				won++
			case errors.Is(err, coordination.ErrNoSlot):
				noSlot++
			default:
				t.Fatalf("unexpected claim error: %v", err)
			}
		}
		if won != 2 || noSlot != racers-2 {
			t.Fatalf("round %d: want exactly 2 winners and %d ErrNoSlot, got %d/%d", round, racers-2, won, noSlot)
		}
	}
}

func TestBackendRejectsStaleOwnerAndGeneration(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 1); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)
	a := mustClaim(t, ctx, pool, "stale-owner")

	// another process wins the slot: new generation, new owner, fresh lease
	if _, err := pool.Exec(ctx, `UPDATE nats_slots
 SET owner_id='thief', generation=generation+1, lease_until=clock_timestamp()+interval '30 seconds'
 WHERE slot_id=$1`, a.Slot); err != nil {
		t.Fatal(err)
	}

	if err := backend.Renew(ctx, a, 30*time.Second); !errors.Is(err, coordination.ErrLost) {
		t.Fatalf("renew want ErrLost, got %v", err)
	}
	if err := backend.Activate(ctx, a); !errors.Is(err, coordination.ErrLost) {
		t.Fatalf("activate want ErrLost, got %v", err)
	}
	if err := backend.Release(ctx, a); !errors.Is(err, coordination.ErrLost) {
		t.Fatalf("release want ErrLost, got %v", err)
	}
	// the slot keeps its new generation and owner: a stale write cannot revert it
	if _, gen, owner, _ := rowState(t, ctx, pool, a.Slot); owner != "thief" || gen != a.Generation+1 {
		t.Fatalf("stale owner/generation must not be overwritten: gen=%d owner=%s", gen, owner)
	}
}

func TestBackendReleaseAndRenewRefusedAfterLeaseExpiry(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 1); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)
	a := mustClaim(t, ctx, pool, "owner-a")

	// expire the lease under the owner's feet (no renewal arrived in time)
	if _, err := pool.Exec(ctx, `UPDATE nats_slots SET lease_until=clock_timestamp()-interval '1 second' WHERE slot_id=$1`, a.Slot); err != nil {
		t.Fatal(err)
	}

	if err := backend.Renew(ctx, a, 30*time.Second); !errors.Is(err, coordination.ErrLost) {
		t.Fatalf("renew of expired lease want ErrLost, got %v", err)
	}
	if err := backend.Release(ctx, a); !errors.Is(err, coordination.ErrLost) {
		t.Fatalf("release of expired lease want ErrLost, got %v", err)
	}
	// the row stays fenced as-is: not available, not reallocated
	if state, _, owner, _ := rowState(t, ctx, pool, a.Slot); state != "starting" || owner != "owner-a" {
		t.Fatalf("expired lease must stay allocated: state=%s owner=%s", state, owner)
	}
}

func TestBackendQuarantineExpiredLeases(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 2); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)
	expired := mustClaim(t, ctx, pool, "zombie")
	fresh := mustClaim(t, ctx, pool, "healthy")

	// zombie's lease drains; healthy keeps renewing
	if _, err := pool.Exec(ctx, `UPDATE nats_slots SET lease_until=clock_timestamp()-interval '1 second' WHERE slot_id=$1`, expired.Slot); err != nil {
		t.Fatal(err)
	}
	_ = backend.Renew(ctx, fresh, 30*time.Second)

	n, err := backend.QuarantineExpired(ctx)
	if err != nil {
		t.Fatalf("quarantine: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 quarantined slot, got %d", n)
	}
	if state, _, owner, _ := rowState(t, ctx, pool, expired.Slot); state != "suspect" || owner != "zombie" {
		t.Fatalf("expired lease must become suspect with preserved owner: state=%s owner=%s", state, owner)
	}
	// the healthy slot keeps its pre-quarantine state and owner
	if state, _, owner, _ := rowState(t, ctx, pool, fresh.Slot); state == "suspect" || owner != "healthy" {
		t.Fatalf("healthy slot must be untouched: state=%s owner=%s", state, owner)
	}
	// idempotent: nothing left to quarantine
	if n, err := backend.QuarantineExpired(ctx); err != nil || n != 0 {
		t.Fatalf("second quarantine want 0/%v, got %d/%v", nil, n, err)
	}
}

func TestBackendPeersListsOnlyLiveLeases(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 3); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)

	a0 := mustClaim(t, ctx, pool, "peer-0")
	if err := backend.Activate(ctx, a0); err != nil {
		t.Fatal(err)
	}
	a1 := mustClaim(t, ctx, pool, "peer-1")
	if err := backend.Activate(ctx, a1); err != nil {
		t.Fatal(err)
	}

	// slot 1's lease drains: no longer a peer
	if _, err := pool.Exec(ctx, `UPDATE nats_slots SET lease_until=clock_timestamp()-interval '1 second' WHERE slot_id=$1`, a1.Slot); err != nil {
		t.Fatal(err)
	}

	peers, err := backend.Peers(ctx, a0.Slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("expired peer must be excluded, got %v", peers)
	}

	// restore the lease, then exclude self: exactly the other live slot
	if _, err := pool.Exec(ctx, `UPDATE nats_slots SET lease_until=clock_timestamp()+interval '30 seconds' WHERE slot_id=$1`, a1.Slot); err != nil {
		t.Fatal(err)
	}
	peers, err = backend.Peers(ctx, a0.Slot)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].Slot != a1.Slot || peers[0].RouteURL != "nats-route://127.0.0.1:6222" {
		t.Fatalf("want [slot %d route], got %v", a1.Slot, peers)
	}
}

func TestBackendClaimErrNoSlotWhenAllTaken(t *testing.T) {
	ctx := context.Background()
	pool := db(t)
	if err := resetSlots(ctx, 1); err != nil {
		t.Fatal(err)
	}
	backend := New(pool)
	mustClaim(t, ctx, pool, "a")
	_, err := backend.Claim(ctx, coordination.ClaimRequest{Owner: "b", RouteURL: "nats-route://127.0.0.1:6222"}, 30*time.Second)
	if !errors.Is(err, coordination.ErrNoSlot) {
		t.Fatalf("want ErrNoSlot, got %v", err)
	}
}
