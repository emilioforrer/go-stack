// Package main demonstrates wiring an embedded NATS server to the go-stack
// coordination package: the process claims a logical identity slot from
// PostgreSQL, starts an embedded NATS server for that slot (clustered with the
// slots owned by other instances), keeps the lease renewed in the background,
// and releases the slot on shutdown.
//
// The example:
//
//  1. Connects to PostgreSQL with pgx and wraps it in the postgres Backend
//     (implements coordination.Backend via SELECT ... FOR UPDATE SKIP LOCKED
//     row locks, lease expiry, and generation fencing).
//  2. Calls Runner.RunOnce, which claims a slot (bumping its generation and
//     stamping lease_until), reads the currently active peers, and starts the
//     embedded NATS Service for the claimed slot name (e.g. nats-0).
//  3. Activates the slot and keeps it renewed every RenewEvery. Losing the
//     lease cancels the work context so the server is stopped before the slot
//     is released.
//  4. On SIGINT/SIGTERM the runner stops the server within StopTimeout and
//     releases the slot, returning it to the pool for other instances.
//  5. If another instance owns every slot, RunOnce reports ErrNoSlot, the
//     demo logs client-only mode, and the loop retries after a delay.
//
// Before running, create the slot table (three slots by default):
//
//	psql "$DATABASE_URL" -f sql/schema.sql
//
// Run one or more instances against the same database. On the same machine,
// give every instance distinct client/route ports and a distinct data dir:
//
//	go run .                                                  # instance 1
//	NATS_CLIENT_PORT=14222 NATS_ROUTE_PORT=16222 \
//	NATS_DATA_DIR=/tmp/nats-2 go run .                        # instance 2
//
// Environment:
//
//	DATABASE_URL       postgres DSN (default: local dev postgres)
//	POD_IP             routable IP advertised to cluster peers (default: 127.0.0.1)
//	NATS_CLIENT_PORT   embedded NATS client port (default: 4222)
//	NATS_ROUTE_PORT    embedded NATS cluster/route port (default: 6222)
//	NATS_CLUSTER_NAME  NATS cluster name (default: app-nats)
//	NATS_DATA_DIR      JetStream storage root (default: ./data)
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/emilioforrer/go-stack/pkg/coordination"
	pgbackend "github.com/emilioforrer/go-stack/pkg/coordination/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats-server/v2/server"
)

// errNatsNotReady is returned when the embedded server fails to accept
// connections; Start shuts it down again before returning.
var errNatsNotReady = errors.New("NATS failed to become ready")

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func port(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil {
		slog.Error("parse port", "env", k, "error", e)
		os.Exit(1)
	}
	return v
}

func ownerID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		slog.Error("generate owner id", "error", e)
		os.Exit(1)
	}
	return hex.EncodeToString(b)
}

// embeddedNATS implements coordination.Service backed by a NATS server started
// inside this process. Start binds the client port, joins the cluster through
// the route port, and seeds JetStream storage per slot; Stop waits for the
// server to quiesce.
type embeddedNATS struct {
	ns        *server.Server
	routePort int
	ip, dir   string
}

func (s *embeddedNATS) Start(ctx context.Context, a coordination.Assignment, peers []coordination.Peer) error {
	routes := make([]*url.URL, 0, len(peers))
	for _, p := range peers {
		u, e := url.Parse(p.RouteURL)
		if e != nil {
			return e
		}
		routes = append(routes, u)
	}
	opts := &server.Options{
		ServerName:         a.Name(),
		Host:               "0.0.0.0",
		Port:               port("NATS_CLIENT_PORT", 4222),
		JetStream:          true,
		StoreDir:           filepath.Join(s.dir, a.Name()),
		JetStreamMaxStore:  10 << 30,
		JetStreamMaxMemory: 256 << 20,
		// The app owns process signal handling (signal.NotifyContext) and the
		// graceful shutdown order (service.Stop -> backend.Release). NATS's
		// embedded signal handler (installed by Server.Start; NoSigs is the
		// only opt-out) would race it: on SIGTERM both paths call
		// Server.Shutdown concurrently, which nats-server does not tolerate
		// (shutdownEventing runs before the idempotency guard, so the second
		// entrant panics on close of the already-nulled resetCh, or the
		// handler's os.Exit kills the process before the slot is released).
		NoSigs: true,
		Routes: routes,
	}
	// JetStream cluster mode fatally requires at least one route connection,
	// so the process that claims the first slot (no peers yet) must start a
	// standalone server. It keeps the slot identity; NATS routes are static
	// per start here, so it rejoins as a clustered node on later generations
	// (after release + a fresh claim with active peers).
	if len(routes) > 0 {
		opts.Cluster = server.ClusterOpts{
			Name:      env("NATS_CLUSTER_NAME", "app-nats"),
			Host:      "0.0.0.0",
			Port:      s.routePort,
			Advertise: net.JoinHostPort(s.ip, strconv.Itoa(s.routePort)),
		}
	}
	ns, e := server.NewServer(opts)
	if e != nil {
		return e
	}
	s.ns = ns
	// Deliberately NOT calling ns.ConfigureLogger(): in embedded (library)
	// mode it also installs nats-server's own process signal handler, which
	// then races the app's graceful-shutdown path and can panic the server
	// (nats-server v2.10.26, server/events.go shutdownEventing). The app owns
	// signal handling and the lifecycle log lines.
	ns.Start()
	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		ns.WaitForShutdown()
		s.ns = nil
		return errNatsNotReady
	}
	slog.Info("embedded NATS started", "server", a.Name(), "store_dir", opts.StoreDir, "peers", len(routes))
	return nil
}

func (s *embeddedNATS) Stop(ctx context.Context) error {
	if s.ns == nil {
		return nil
	}
	s.ns.Shutdown()
	done := make(chan struct{})
	go func() { s.ns.WaitForShutdown(); close(done) }()
	select {
	case <-done:
		s.ns = nil
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

// run keeps claiming a slot and running the embedded NATS server until the
// process is signalled. ErrNoSlot is not fatal: with no free slot the
// instance serves as client-only and the loop retries after a delay.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := env("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	ip := env("POD_IP", "127.0.0.1")

	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer db.Close()
	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	backend := pgbackend.New(db)
	runner := coordination.Runner{Backend: backend, LeaseTTL: 30 * time.Second, RenewEvery: 5 * time.Second, StopTimeout: 10 * time.Second}
	routePort := port("NATS_ROUTE_PORT", 6222)
	request := coordination.ClaimRequest{Owner: ownerID(), RouteURL: "nats-route://" + net.JoinHostPort(ip, strconv.Itoa(routePort))}

	// Retry when another instance owns all three slots. A real app would serve
	// HTTP as a NATS client while this loop runs; this demo only logs that role.
	for ctx.Err() == nil {
		nats := &embeddedNATS{routePort: routePort, ip: ip, dir: env("NATS_DATA_DIR", "./data")}
		err := runner.RunOnce(ctx, request, nats)
		if ctx.Err() != nil {
			return nil
		}
		switch {
		case errors.Is(err, coordination.ErrNoSlot):
			slog.Info("no NATS slot: running as client-only, retrying", "delay", "5s")
		case err != nil:
			slog.Error("coordinator stopped, retrying", "error", err)
		default:
			slog.Info("NATS server exited, retrying")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
		}
	}

	return nil
}
