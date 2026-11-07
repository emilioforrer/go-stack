// Package main demonstrates the go-stack coordination package wired through
// the go-stack boot lifecycle (pkg/boot). Three boot providers are installed:
//
//  1. PostgresProvider   - opens the pgx pool (DATABASE_URL) and registers it
//     and the coordination.Backend in the DI container.
//  2. NATSServerProvider - owns one coordination.Runner and starts the
//     claim/retry loop in its Boot phase: a slot is claimed (FOR UPDATE SKIP
//     LOCKED, lease TTL 30s), the embedded NATS server starts for the claimed
//     slot name (e.g. nats-0) clustered with active peers, the slot is
//     activated and kept renewed in the background. When another instance
//     owns every slot the instance logs client-only mode and retries. On
//     shutdown the loop is cancelled, which makes RunOnce stop the embedded
//     server and release the slot.
//  3. NATSClientProvider - connects a real NATS client to the live server;
//     instances that never claim a slot report client-only mode.
//
// Container services (see boot.go): *pgxpool.Pool, *postgres.Backend,
// *Coordinator, *embeddedNATS and *nats.Conn.
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
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/samber/do/v2"
)

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

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	bootstrapper := boot.NewDefaultBootstrapper(
		do.New(),
		// Registration order defines boot order; Shutdown runs in reverse:
		// the NATS client drains first, then the coordinator loop is cancelled
		// (RunOnce stops the embedded server and releases the slot), and only
		// then does the postgres connection close.
		NewPostgresProvider(),
		NewNATSServerProvider(),
		NewNATSClientProvider(),
	)

	if err := bootstrapper.Register(ctx); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	if err := bootstrapper.Boot(ctx); err != nil {
		shutdownErr := bootstrapper.Shutdown(context.Background())
		return errors.Join(fmt.Errorf("boot: %w", err), shutdownErr)
	}

	slog.Info("application started")

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := bootstrapper.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	slog.Info("application stopped")
	return nil
}
