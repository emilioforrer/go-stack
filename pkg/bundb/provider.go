package bundb

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/schema"
)

// ProviderOptions holds configuration for the BunProvider.
type ProviderOptions struct {
	// BunDB is an optional pre-configured *bun.DB. If set, SQLDB and PGXPool are ignored.
	BunDB *bun.DB

	// PGXPool is an optional pgx connection pool. If set, it is converted to *sql.DB
	// via stdlib.OpenDBFromPool, and SQLDB is ignored.
	PGXPool *pgxpool.Pool

	// SQLDB is the underlying *sql.DB to wrap with bun. Required if BunDB and PGXPool are nil.
	SQLDB *sql.DB

	// Dialect is the bun dialect to use with SQLDB or PGXPool. Required if BunDB is nil.
	Dialect schema.Dialect

	// MaxOpenConns sets the maximum number of open connections to the database.
	// Applied only when SQLDB is provided directly.
	MaxOpenConns int

	// MaxIdleConns sets the maximum number of idle connections in the pool.
	// Applied only when SQLDB is provided directly.
	MaxIdleConns int

	// ConnMaxLifetime sets the maximum amount of time a connection may be reused.
	// Applied only when SQLDB is provided directly.
	ConnMaxLifetime time.Duration
}

// BunProvider implements boot.Provider to integrate uptrace/bun into the application lifecycle.
type BunProvider struct {
	boot.DefaultProvider
	options ProviderOptions
	db      *bun.DB
}

var _ boot.Provider = (*BunProvider)(nil)

// NewBunProvider creates a new BunProvider with the given options.
func NewBunProvider(opts ProviderOptions) *BunProvider {
	return &BunProvider{options: opts}
}

// Register initializes the bun.DB instance and registers it in the container.
func (p *BunProvider) Register(_ context.Context, container boot.Container) error {
	var db *bun.DB

	switch {
	case p.options.BunDB != nil:
		db = p.options.BunDB
	case p.options.PGXPool != nil:
		if p.options.Dialect == nil {
			return fmt.Errorf("bundb provider: Dialect is required when PGXPool is provided")
		}
		sqldb := stdlib.OpenDBFromPool(p.options.PGXPool)
		db = bun.NewDB(sqldb, p.options.Dialect)
		do.ProvideValue(container.Injector(), p.options.PGXPool)
	case p.options.SQLDB != nil:
		if p.options.Dialect == nil {
			return fmt.Errorf("bundb provider: Dialect is required when SQLDB is provided")
		}

		sqldb := p.options.SQLDB

		if p.options.MaxOpenConns > 0 {
			sqldb.SetMaxOpenConns(p.options.MaxOpenConns)
		}
		if p.options.MaxIdleConns > 0 {
			sqldb.SetMaxIdleConns(p.options.MaxIdleConns)
		}
		if p.options.ConnMaxLifetime > 0 {
			sqldb.SetConnMaxLifetime(p.options.ConnMaxLifetime)
		}

		db = bun.NewDB(sqldb, p.options.Dialect)
	default:
		return fmt.Errorf("bundb provider: one of BunDB, PGXPool, or SQLDB must be provided")
	}

	p.db = db

	do.ProvideValue(container.Injector(), p.db)

	slog.Info("bundb provider registered")

	return nil
}

// Boot verifies the database connection by pinging it.
func (p *BunProvider) Boot(ctx context.Context, _ boot.Container) error {
	if p.db == nil {
		return fmt.Errorf("bundb provider: db not initialized")
	}

	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("bundb provider: ping database: %w", err)
	}

	slog.Info("bundb provider booted", "status", "connected")

	return nil
}

// Shutdown closes the underlying database connection.
func (p *BunProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.db == nil {
		return nil
	}

	slog.Info("bundb provider shutting down...")

	if err := p.db.Close(); err != nil {
		return fmt.Errorf("bundb provider: close database: %w", err)
	}

	if p.options.PGXPool != nil {
		p.options.PGXPool.Close()
	}

	slog.Info("bundb provider shut down")

	return nil
}

// DB returns the initialized *bun.DB instance.
func (p *BunProvider) DB() *bun.DB {
	return p.db
}
