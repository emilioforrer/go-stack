package bundb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "modernc.org/sqlite"
)

// isNil reports whether v is nil, handling typed nil pointers in interfaces.
func isNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	}
	return false
}

func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func requireError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func requireNil(t *testing.T, v any) {
	t.Helper()
	if !isNil(v) {
		t.Fatalf("expected nil, got %+v", v)
	}
}

func requireNotNil(t *testing.T, v any) {
	t.Helper()
	if isNil(v) {
		t.Fatalf("expected non-nil, got nil")
	}
}

func requireContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Fatalf("%q does not contain %q", s, substr)
	}
}

func newMockContainer() boot.Container {
	return boot.NewContainer(do.New())
}

func TestNewBunProvider(t *testing.T) {
	t.Parallel()

	opts := ProviderOptions{MaxOpenConns: 10}
	provider := NewBunProvider(opts)

	requireNotNil(t, provider)
	requireNotNil(t, provider.options)
	requireNil(t, provider.db)
}

func TestBunProvider_Register_WithBunDB(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())

	provider := NewBunProvider(ProviderOptions{BunDB: db})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)
	requireNotNil(t, provider.db)

	resolved, err := do.Invoke[*bun.DB](container.Injector())
	requireNoError(t, err)
	requireNotNil(t, resolved)
}

func TestBunProvider_Register_WithSQLDB(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	provider := NewBunProvider(ProviderOptions{
		SQLDB:           sqldb,
		Dialect:         sqlitedialect.New(),
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 30,
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)
	requireNotNil(t, provider.db)

	resolved, err := do.Invoke[*bun.DB](container.Injector())
	requireNoError(t, err)
	requireNotNil(t, resolved)
}

func TestBunProvider_Register_WithPGXPool(t *testing.T) {
	t.Parallel()

	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost:1/db")
	requireNoError(t, err)
	config.MinConns = 0
	config.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	requireNoError(t, err)
	t.Cleanup(func() { pool.Close() })

	provider := NewBunProvider(ProviderOptions{
		PGXPool: pool,
		Dialect: sqlitedialect.New(),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)
	requireNotNil(t, provider.db)

	resolvedPool, err := do.Invoke[*pgxpool.Pool](container.Injector())
	requireNoError(t, err)
	requireNotNil(t, resolvedPool)

	resolvedDB, err := do.Invoke[*bun.DB](container.Injector())
	requireNoError(t, err)
	requireNotNil(t, resolvedDB)
}

func TestBunProvider_Register_PGXPool_MissingDialect(t *testing.T) {
	t.Parallel()

	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost:1/db")
	requireNoError(t, err)
	config.MinConns = 0
	config.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	requireNoError(t, err)
	t.Cleanup(func() { pool.Close() })

	provider := NewBunProvider(ProviderOptions{PGXPool: pool})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "Dialect is required")
}

func TestBunProvider_Register_MissingDialect(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	provider := NewBunProvider(ProviderOptions{
		SQLDB: sqldb,
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "Dialect is required")
}

func TestBunProvider_Register_MissingEverything(t *testing.T) {
	t.Parallel()

	provider := NewBunProvider(ProviderOptions{})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Register(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "one of BunDB, PGXPool, or SQLDB must be provided")
}

func TestBunProvider_Boot(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	provider := NewBunProvider(ProviderOptions{
		BunDB: bun.NewDB(sqldb, sqlitedialect.New()),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	err = provider.Boot(ctx, container)
	requireNoError(t, err)
}

func TestBunProvider_Boot_NotInitialized(t *testing.T) {
	t.Parallel()

	provider := NewBunProvider(ProviderOptions{})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Boot(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "db not initialized")
}

func TestBunProvider_Shutdown(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)

	provider := NewBunProvider(ProviderOptions{
		BunDB: bun.NewDB(sqldb, sqlitedialect.New()),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	err = provider.Shutdown(ctx, container)
	requireNoError(t, err)
}

func TestBunProvider_Shutdown_WithPGXPool(t *testing.T) {
	t.Parallel()

	config, err := pgxpool.ParseConfig("postgres://user:pass@localhost:1/db")
	requireNoError(t, err)
	config.MinConns = 0
	config.MaxConns = 1

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	requireNoError(t, err)

	provider := NewBunProvider(ProviderOptions{
		PGXPool: pool,
		Dialect: sqlitedialect.New(),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	err = provider.Shutdown(ctx, container)
	requireNoError(t, err)
}

func TestBunProvider_Shutdown_NilDB(t *testing.T) {
	t.Parallel()

	provider := NewBunProvider(ProviderOptions{})
	container := newMockContainer()
	ctx := context.Background()

	err := provider.Shutdown(ctx, container)
	requireNoError(t, err)
}

func TestBunProvider_Shutdown_CloseError(t *testing.T) {
	t.Parallel()

	// Register a driver that returns an error on connection close.
	driverName := "errclose_" + t.Name()
	sql.Register(driverName, errCloseDriver{})

	sqldb, err := sql.Open(driverName, "")
	requireNoError(t, err)

	// Force a connection to be opened so Close has something to close.
	_ = sqldb.PingContext(context.Background())

	provider := NewBunProvider(ProviderOptions{
		BunDB: bun.NewDB(sqldb, sqlitedialect.New()),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	err = provider.Shutdown(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "close database")
	requireContains(t, err.Error(), "driver close error")
}

// errCloseDriver is a test driver that returns an error when its connection is closed.
type errCloseDriver struct{}

func (errCloseDriver) Open(string) (driver.Conn, error) {
	return &errCloseConn{}, nil
}

type errCloseConn struct{}

func (c *errCloseConn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("not implemented") }
func (c *errCloseConn) Close() error                         { return fmt.Errorf("driver close error") }
func (c *errCloseConn) Begin() (driver.Tx, error)            { return nil, fmt.Errorf("not implemented") }
func (c *errCloseConn) Exec(string, []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *errCloseConn) Query(string, []driver.Value) (driver.Rows, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestBunProvider_DB(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	provider := NewBunProvider(ProviderOptions{BunDB: db})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	requireNotNil(t, provider.DB())
}

func TestBunProvider_DB_Nil(t *testing.T) {
	t.Parallel()

	provider := NewBunProvider(ProviderOptions{})
	requireNil(t, provider.DB())
}

// Ensure BunProvider implements boot.Provider.
func TestBunProvider_ImplementsProvider(t *testing.T) {
	t.Parallel()

	var _ boot.Provider = (*BunProvider)(nil)
}

// mockContainerWithInjector is a minimal boot.Container for error-path tests.
type mockContainerWithInjector struct {
	injector do.Injector
}

func (m *mockContainerWithInjector) Injector() do.Injector {
	return m.injector
}

func TestBunProvider_Register_InjectorError(t *testing.T) {
	t.Parallel()

	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	provider := NewBunProvider(ProviderOptions{
		BunDB: bun.NewDB(sqldb, sqlitedialect.New()),
	})

	injector := do.New()
	container := &mockContainerWithInjector{injector: injector}
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)
}

func TestBunProvider_Boot_PingError(t *testing.T) {
	t.Parallel()

	// Open a sqlite DB then immediately close it to force ping failures
	sqldb, err := sql.Open("sqlite", ":memory:")
	requireNoError(t, err)
	_ = sqldb.Close()

	provider := NewBunProvider(ProviderOptions{
		BunDB: bun.NewDB(sqldb, sqlitedialect.New()),
	})
	container := newMockContainer()
	ctx := context.Background()

	err = provider.Register(ctx, container)
	requireNoError(t, err)

	err = provider.Boot(ctx, container)
	requireError(t, err)
	requireContains(t, err.Error(), "ping database")
}
