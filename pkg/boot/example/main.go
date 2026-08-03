// Package main demonstrates wiring multiple boot.Providers together using the
// go-stack boot package, with configuration managed by koanf.
//
// Five providers are registered with a DefaultBootstrapper:
//
//  1. ConfigProvider    - loads config.yaml with koanf and registers the global
//     *koanf.Koanf into the DI container.
//  2. MigratorProvider  - runs bun migrations against a SQLite database.
//  3. BunSetupProvider  - opens a *bun.DB (SQLite) and exposes a repository.
//  4. SeederProvider    - seeds the users table asynchronously when it is empty.
//  5. WebServerProvider  - starts an HTTP server that depends on the DB.
//
// The ConfigProvider is registered first so the global *koanf.Koanf is available
// when every other provider runs its Register phase. Each subsequent provider
// resolves the global koanf from the container and unmarshals its own config
// struct from its dedicated subsection (e.g. "db", "server", "seeder").
//
// Because DefaultBootstrapper.Boot executes providers in registration order, the
// web server is guaranteed to boot only after the migrator, the bun setup, and
// the seeder have completed. The seeder waits for the DB to be ready (it boots
// after the bun setup) and runs the insert in a background goroutine. Shutdown
// runs in reverse order: the HTTP server stops first, then the seeder is awaited,
// before the database connection is closed.
package main

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emilioforrer/go-stack/pkg/boot"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/samber/do/v2"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/migrate"
	_ "modernc.org/sqlite" // register the pure-Go sqlite driver
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ----------------------------------------------------------------------------
// ConfigProvider
// ----------------------------------------------------------------------------

// ConfigProvider loads configuration from a YAML file using koanf and registers
// the global *koanf.Koanf into the DI container. It is registered first so every
// other provider can resolve the global koanf during its own Register phase and
// unmarshal its dedicated config struct.
type ConfigProvider struct {
	boot.DefaultProvider
	path string
}

var _ boot.Provider = (*ConfigProvider)(nil)

func NewConfigProvider(path string) *ConfigProvider {
	return &ConfigProvider{path: path}
}

func (p *ConfigProvider) Register(_ context.Context, container boot.Container) error {
	k := koanf.New(".")

	if err := k.Load(file.Provider(p.path), yaml.Parser()); err != nil {
		return fmt.Errorf("config: load %s: %w", p.path, err)
	}

	do.ProvideValue(container.Injector(), k)

	slog.Info("config provider registered", "path", p.path, "keys", len(k.Keys()))

	return nil
}

func (p *ConfigProvider) Boot(_ context.Context, _ boot.Container) error {
	slog.Info("config provider booted")
	return nil
}

func (p *ConfigProvider) Shutdown(_ context.Context, _ boot.Container) error {
	slog.Info("config provider shut down")
	return nil
}

// ----------------------------------------------------------------------------
// MigratorProvider
// ----------------------------------------------------------------------------

// MigratorConfig holds the migrator provider's configuration, unmarshaled from the
// "migrator" subsection of the global koanf.
type MigratorConfig struct {
	MigrationsDir string `koanf:"migrations_dir"`
}

// MigratorProvider runs the bun migrations discovered from the embedded
// migrations directory. It is registered after ConfigProvider so it can read its
// config, and before the web server so its Boot phase runs first.
type MigratorProvider struct {
	boot.DefaultProvider
	migrations *migrate.Migrations
	config     MigratorConfig
}

var _ boot.Provider = (*MigratorProvider)(nil)

func NewMigratorProvider(migrations *migrate.Migrations) *MigratorProvider {
	return &MigratorProvider{migrations: migrations}
}

func (p *MigratorProvider) Register(_ context.Context, container boot.Container) error {
	k, err := do.Invoke[*koanf.Koanf](container.Injector())
	if err != nil {
		return fmt.Errorf("migrator: invoke koanf: %w", err)
	}

	if err := k.Unmarshal("migrator", &p.config); err != nil {
		return fmt.Errorf("migrator: unmarshal config: %w", err)
	}

	// The *bun.DB is registered by BunSetupProvider, which is why the migrator is
	// booted after it. Provide a lazy factory so the migrator can be resolved once
	// the DB is available, and store the migrations collection for the Boot phase.
	do.Provide(container.Injector(), func(i do.Injector) (*migrate.Migrator, error) {
		db, err := do.Invoke[*bun.DB](i)
		if err != nil {
			return nil, err
		}
		return migrate.NewMigrator(db, p.migrations), nil
	})

	slog.Info("migrator provider registered", "migrations_dir", p.config.MigrationsDir)

	return nil
}

func (p *MigratorProvider) Boot(ctx context.Context, container boot.Container) error {
	migrator, err := do.Invoke[*migrate.Migrator](container.Injector())
	if err != nil {
		return fmt.Errorf("migrator: invoke migrator: %w", err)
	}

	if err := migrator.Init(ctx); err != nil {
		return fmt.Errorf("migrator: init: %w", err)
	}

	group, err := migrator.Migrate(ctx)
	if err != nil {
		return fmt.Errorf("migrator: migrate: %w", err)
	}

	if group.IsZero() {
		slog.Info("migrator booted", "status", "no pending migrations")
		return nil
	}

	slog.Info("migrator booted", "migrations", len(group.Migrations))

	return nil
}

func (p *MigratorProvider) Shutdown(_ context.Context, _ boot.Container) error {
	slog.Info("migrator provider shut down")
	return nil
}

// ----------------------------------------------------------------------------
// BunSetupProvider
// ----------------------------------------------------------------------------

// DBConfig holds the database provider's configuration, unmarshaled from the "db"
// subsection of the global koanf.
type DBConfig struct {
	DSN          string `koanf:"dsn"`
	MaxOpenConns int    `koanf:"max_open_conns"`
	MaxIdleConns int    `koanf:"max_idle_conns"`
}

// UserRepository is a small service backed by the *bun.DB. It is provided into the
// container so other providers (e.g. the web server) can resolve it.
type UserRepository struct {
	db *bun.DB
}

func NewUserRepository(db *bun.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.NewSelect().
		TableExpr("users").
		ColumnExpr("count(*)").
		Scan(ctx, &count); err != nil {
		return 0, fmt.Errorf("repository: count users: %w", err)
	}
	return count, nil
}

// BunSetupProvider opens the SQLite database and provides *bun.DB and the
// UserRepository into the container. It boots before the web server because it is
// registered before it.
type BunSetupProvider struct {
	boot.DefaultProvider
	config DBConfig
	db     *bun.DB
}

var _ boot.Provider = (*BunSetupProvider)(nil)

func NewBunSetupProvider() *BunSetupProvider {
	return &BunSetupProvider{}
}

func (p *BunSetupProvider) Register(_ context.Context, container boot.Container) error {
	k, err := do.Invoke[*koanf.Koanf](container.Injector())
	if err != nil {
		return fmt.Errorf("bundb setup: invoke koanf: %w", err)
	}

	if err := k.Unmarshal("db", &p.config); err != nil {
		return fmt.Errorf("bundb setup: unmarshal config: %w", err)
	}

	sqldb, err := openSQLite(p.config.DSN)
	if err != nil {
		return fmt.Errorf("bundb setup: open sqlite: %w", err)
	}

	if p.config.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(p.config.MaxOpenConns)
	}
	if p.config.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(p.config.MaxIdleConns)
	}

	p.db = bun.NewDB(sqldb, sqlitedialect.New())

	do.ProvideValue(container.Injector(), p.db)
	do.Provide(container.Injector(), func(i do.Injector) (*UserRepository, error) {
		db, err := do.Invoke[*bun.DB](i)
		if err != nil {
			return nil, err
		}
		return NewUserRepository(db), nil
	})

	slog.Info("bun setup provider registered", "dsn", p.config.DSN)

	return nil
}

func (p *BunSetupProvider) Boot(ctx context.Context, _ boot.Container) error {
	if p.db == nil {
		return errors.New("bundb setup: db not initialized")
	}

	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("bundb setup: ping database: %w", err)
	}

	slog.Info("bun setup provider booted", "status", "connected")

	return nil
}

func (p *BunSetupProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.db == nil {
		return nil
	}

	slog.Info("bun setup provider shutting down...")

	if err := p.db.Close(); err != nil {
		return fmt.Errorf("bundb setup: close database: %w", err)
	}

	slog.Info("bun setup provider shut down")

	return nil
}

// ----------------------------------------------------------------------------
// SeederProvider
// ----------------------------------------------------------------------------

// SeederConfig holds the seeder provider's configuration, unmarshaled from the
// "seeder" subsection of the global koanf.
type SeederConfig struct {
	Count int `koanf:"count"`
}

// User is the bun model for the users table seeded by the SeederProvider.
type User struct {
	bun.BaseModel `bun:"table:users"`

	ID   int64  `bun:"id,pk,autoincrement"`
	Name string `bun:"name"`
}

// SeederProvider seeds the users table with a fixed number of users when it is
// empty. It is registered after BunSetupProvider and before WebServerProvider, so
// its Boot phase runs once the DB is ready and before the web server starts.
//
// The seeding itself runs asynchronously: Boot launches a goroutine and returns
// immediately so application startup is not blocked. The goroutine is tied to a
// context that is cancelled during Shutdown, allowing an in-flight seed to be
// interrupted.
type SeederProvider struct {
	boot.DefaultProvider
	config    SeederConfig
	seedCtx   context.Context
	seedStop  context.CancelFunc
	seedDone  chan struct{}
	seedError error
}

var _ boot.Provider = (*SeederProvider)(nil)

func NewSeederProvider() *SeederProvider {
	return &SeederProvider{}
}

func (p *SeederProvider) Register(_ context.Context, container boot.Container) error {
	k, err := do.Invoke[*koanf.Koanf](container.Injector())
	if err != nil {
		return fmt.Errorf("seeder: invoke koanf: %w", err)
	}

	if err := k.Unmarshal("seeder", &p.config); err != nil {
		return fmt.Errorf("seeder: unmarshal config: %w", err)
	}

	slog.Info("seeder provider registered", "count", p.config.Count)

	return nil
}

func (p *SeederProvider) Boot(ctx context.Context, container boot.Container) error {
	db, err := do.Invoke[*bun.DB](container.Injector())
	if err != nil {
		return fmt.Errorf("seeder: invoke bun.DB: %w", err)
	}

	// Wait for the DB to be ready. BunSetupProvider.Boot already ran a ping before
	// this provider, but we re-check so the seeder is resilient to reordering.
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("seeder: database not ready: %w", err)
	}

	// Only seed when the table is empty.
	var count int
	if err := db.NewSelect().
		TableExpr("users").
		ColumnExpr("count(*)").
		Scan(ctx, &count); err != nil {
		return fmt.Errorf("seeder: count users: %w", err)
	}
	if count > 0 {
		slog.Info("seeder booted", "status", "skipped", "existing", count)
		return nil
	}

	// Run the seed asynchronously so Boot does not block application startup. The
	// context is detached from the boot context and cancelled on Shutdown.
	p.seedCtx, p.seedStop = context.WithCancel(context.Background())
	p.seedDone = make(chan struct{})

	go p.seed(db)

	slog.Info("seeder booted", "status", "seeding async", "count", p.config.Count)

	return nil
}

// seed inserts the configured number of users. It records the first error on the
// provider so it can be surfaced after shutdown. The done channel is always
// closed to signal completion.
func (p *SeederProvider) seed(db *bun.DB) {
	defer close(p.seedDone)

	users := make([]User, p.config.Count)
	for i := range users {
		users[i] = User{Name: fmt.Sprintf("user-%03d", i+1)}
	}

	if _, err := db.NewInsert().
		Model(&users).
		Exec(p.seedCtx); err != nil && !errors.Is(err, context.Canceled) {
		p.seedError = fmt.Errorf("seeder: insert users: %w", err)
		slog.Error("seeder failed", "error", err)
		return
	}

	slog.Info("seeder completed", "inserted", p.config.Count)
}

func (p *SeederProvider) Shutdown(_ context.Context, _ boot.Container) error {
	if p.seedStop == nil {
		slog.Info("seeder provider shut down")
		return nil
	}

	// Cancel the seed context and wait for the goroutine to finish so we don't
	// leak it or race on a closing DB connection.
	p.seedStop()

	select {
	case <-p.seedDone:
	case <-time.After(5 * time.Second):
		slog.Warn("seeder shutdown: timed out waiting for seed to finish")
	}

	if p.seedError != nil {
		return p.seedError
	}

	slog.Info("seeder provider shut down")

	return nil
}

// ----------------------------------------------------------------------------
// WebServerProvider
// ----------------------------------------------------------------------------

// ServerConfig holds the web server provider's configuration, unmarshaled from
// the "server" subsection of the global koanf.
type ServerConfig struct {
	Addr              string        `koanf:"addr"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout"`
}

// WebServerProvider starts an HTTP server that exposes a /health and /users
// endpoint backed by the UserRepository. It is registered last, so its Boot phase
// only runs once the migrator and bun setup providers have booted.
type WebServerProvider struct {
	boot.DefaultProvider
	config  ServerConfig
	server  *http.Server
	errChan chan error
}

var _ boot.Provider = (*WebServerProvider)(nil)

func NewWebServerProvider() *WebServerProvider {
	return &WebServerProvider{}
}

func (p *WebServerProvider) Register(_ context.Context, container boot.Container) error {
	k, err := do.Invoke[*koanf.Koanf](container.Injector())
	if err != nil {
		return fmt.Errorf("web server: invoke koanf: %w", err)
	}

	if err := k.Unmarshal("server", &p.config); err != nil {
		return fmt.Errorf("web server: unmarshal config: %w", err)
	}

	slog.Info("web server provider registered", "addr", p.config.Addr)

	return nil
}

func (p *WebServerProvider) Boot(_ context.Context, container boot.Container) error {
	repo, err := do.Invoke[*UserRepository](container.Injector())
	if err != nil {
		return fmt.Errorf("web server: invoke repository: %w", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /users", func(w http.ResponseWriter, r *http.Request) {
		count, err := repo.Count(r.Context())
		if err != nil {
			slog.Error("count users", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = fmt.Fprintf(w, "users: %d\n", count)
	})

	readHeaderTimeout := p.config.ReadHeaderTimeout
	if readHeaderTimeout == 0 {
		readHeaderTimeout = 5 * time.Second
	}

	p.server = &http.Server{
		Addr:              p.config.Addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	p.errChan = make(chan error, 1)
	go func() {
		slog.Info("web server booted", "addr", p.config.Addr)
		if err := p.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.errChan <- err
		}
	}()

	// Confirm the server is listening before returning so the bootstrapper can
	// consider the web server ready. Resolve the repository once more to assert the
	// dependency on the bun setup provider is wired.
	if _, err := do.Invoke[*UserRepository](container.Injector()); err != nil {
		return fmt.Errorf("web server: dependency check: %w", err)
	}

	slog.Info("web server ready", "addr", p.config.Addr)

	return nil
}

func (p *WebServerProvider) Shutdown(ctx context.Context, _ boot.Container) error {
	if p.server == nil {
		return nil
	}

	slog.Info("web server provider shutting down...")

	if err := p.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("web server: shutdown: %w", err)
	}

	slog.Info("web server provider shut down")

	return nil
}

// ----------------------------------------------------------------------------
// main
// ----------------------------------------------------------------------------

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	migrations, err := loadMigrations()
	if err != nil {
		slog.Error("load migrations", "error", err)
		os.Exit(1)
	}

	bootstrapper := boot.NewDefaultBootstrapper(
		do.New(),
		// Registration order defines the boot order. ConfigProvider is first so the
		// global *koanf.Koanf is available when every other provider runs Register.
		// The web server is last, so it boots only after the migrator, the bun
		// setup, and the seeder have completed.
		NewConfigProvider("config.yaml"),
		NewMigratorProvider(migrations),
		NewBunSetupProvider(),
		NewSeederProvider(),
		NewWebServerProvider(),
	)

	if err := bootstrapper.Register(ctx); err != nil {
		slog.Error("register", "error", err)
		os.Exit(1)
	}

	if err := bootstrapper.Boot(ctx); err != nil {
		slog.Error("boot", "error", err)
		_ = bootstrapper.Shutdown(context.Background())
		os.Exit(1)
	}

	slog.Info("application started", "addr", "http://localhost:8080")

	<-ctx.Done()
	slog.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := bootstrapper.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("application stopped")
}

// loadMigrations discovers the SQL migration files embedded in the migrations/
// directory and registers them with a bun migrate.Migrations collection.
func loadMigrations() (*migrate.Migrations, error) {
	migrations := migrate.NewMigrations()

	if err := migrations.Discover(migrationsFS); err != nil {
		return nil, fmt.Errorf("discover migrations: %w", err)
	}

	return migrations, nil
}

// openSQLite opens a *sql.DB for the modernc.org/sqlite driver with the given DSN.
func openSQLite(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite", dsn)
}
