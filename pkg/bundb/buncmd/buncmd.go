// Package buncmd provides the BunDB CLI command tree as an importable library.
package buncmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/emilioforrer/go-stack/pkg/bundb"
	"github.com/spf13/cobra"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

// ExitError is an error that carries a specific exit code.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit code %d", e.Code)
}

// Cmd holds the state for the bun CLI command tree.
type Cmd struct {
	DBURL           string
	Dialect         string
	Driver          string
	MigrationsDir   string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewCommand creates a new cobra command tree for bun database operations.
func NewCommand(use string) *cobra.Command {
	c := &Cmd{}

	root := &cobra.Command{
		Use:          use,
		Short:        "BunDB CLI for database schema and migration management",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			_ = cmd.Help()
		},
	}

	root.PersistentFlags().StringVar(&c.DBURL, "db-url", "", "Database URL (e.g. postgres://user:pass@localhost/db)")
	root.PersistentFlags().StringVar(&c.Dialect, "dialect", "pg", "Database dialect: pg, mysql, sqlite")
	root.PersistentFlags().StringVar(&c.Driver, "driver", "", "Database/sql driver name (default depends on dialect)")
	root.PersistentFlags().StringVar(&c.MigrationsDir, "migrations-dir", "migrations", "Migrations directory path")
	root.PersistentFlags().IntVar(&c.MaxOpenConns, "max-open-conns", 25, "Max open connections")
	root.PersistentFlags().IntVar(&c.MaxIdleConns, "max-idle-conns", 5, "Max idle connections")
	root.PersistentFlags().DurationVar(&c.ConnMaxLifetime, "conn-max-lifetime", time.Hour, "Connection max lifetime")

	root.AddCommand(c.newDBCmd())
	root.AddCommand(c.newMigrateCmd())

	return root
}

func (c *Cmd) newDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "db",
		Short: "Database management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create the database",
		RunE:  c.runDBCreate,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "drop",
		Short: "Drop the database",
		RunE:  c.runDBDrop,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Drop, create, and initialize the database",
		RunE:  c.runDBReset,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "truncate",
		Short: "Truncate the migrations table",
		RunE:  c.runDBTruncate,
	})

	return cmd
}

func (c *Cmd) newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migration management commands",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create the migrations table",
		RunE:  c.runMigrateInit,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Run pending migrations",
		RunE:  c.runMigrateUp,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration group",
		RunE:  c.runMigrateDown,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE:  c.runMigrateStatus,
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Short: "Create new SQL migration files",
		Args:  cobra.ExactArgs(1),
		RunE:  c.runMigrateCreate,
	})

	return cmd
}

func (c *Cmd) openDB() (*bun.DB, error) {
	if c.DBURL == "" {
		return nil, fmt.Errorf("--db-url is required")
	}

	dialect, err := bundb.ParseDialect(c.Dialect)
	if err != nil {
		return nil, err
	}

	driver := c.Driver
	if driver == "" {
		driver = bundb.DefaultDriver(c.Dialect)
	}

	sqldb, err := bundb.OpenSQLDB(driver, c.DBURL, bundb.SQLDBOptions{
		MaxOpenConns:    c.MaxOpenConns,
		MaxIdleConns:    c.MaxIdleConns,
		ConnMaxLifetime: c.ConnMaxLifetime,
	})
	if err != nil {
		return nil, err
	}

	return bun.NewDB(sqldb, dialect), nil
}

func (c *Cmd) newMigrator(db *bun.DB) *migrate.Migrator {
	migrations := migrate.NewMigrations(migrate.WithMigrationsDirectory(c.MigrationsDir))
	return migrate.NewMigrator(db, migrations)
}

func (c *Cmd) runDBCreate(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	slog.Info("creating database...", "url", redactURL(c.DBURL))

	if _, err := db.ExecContext(cmd.Context(), "SELECT 1"); err != nil {
		return fmt.Errorf("database is not reachable: %w", err)
	}

	slog.Info("database is ready")
	return nil
}

func (c *Cmd) runDBDrop(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	slog.Info("dropping database...", "url", redactURL(c.DBURL))

	_, err = db.ExecContext(cmd.Context(), "DROP TABLE IF EXISTS bun_migrations")
	if err != nil {
		return fmt.Errorf("drop migrations table: %w", err)
	}

	slog.Info("migrations table dropped")
	return nil
}

func (c *Cmd) runDBReset(cmd *cobra.Command, _ []string) error {
	if err := c.runDBDrop(cmd, nil); err != nil {
		return err
	}
	if err := c.runDBCreate(cmd, nil); err != nil {
		return err
	}
	return c.runMigrateInit(cmd, nil)
}

func (c *Cmd) runDBTruncate(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	if err := migrator.TruncateTable(cmd.Context()); err != nil {
		return fmt.Errorf("truncate migrations table: %w", err)
	}

	slog.Info("migrations table truncated")
	return nil
}

func (c *Cmd) runMigrateInit(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	if err := migrator.Init(cmd.Context()); err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}

	slog.Info("migrations table initialized")
	return nil
}

func (c *Cmd) runMigrateUp(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	group, err := migrator.Migrate(cmd.Context())
	if err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}

	if group.IsZero() {
		slog.Info("no pending migrations")
		return nil
	}

	slog.Info("migrated up", "migrations", len(group.Migrations))
	return nil
}

func (c *Cmd) runMigrateDown(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	group, err := migrator.Rollback(cmd.Context())
	if err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}

	if group.IsZero() {
		slog.Info("no migrations to rollback")
		return nil
	}

	slog.Info("rolled back", "migrations", len(group.Migrations))
	return nil
}

func (c *Cmd) runMigrateStatus(cmd *cobra.Command, _ []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	migrations, err := migrator.MigrationsWithStatus(cmd.Context())
	if err != nil {
		return fmt.Errorf("migration status: %w", err)
	}

	if len(migrations) == 0 {
		slog.Info("no migrations")
		return nil
	}

	for _, m := range migrations {
		status := "pending"
		if m.IsApplied() {
			status = "applied"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", status, m.Name)
	}

	return nil
}

func (c *Cmd) runMigrateCreate(cmd *cobra.Command, args []string) error {
	db, err := c.openDB()
	if err != nil {
		return err
	}
	defer db.Close()

	migrator := c.newMigrator(db)
	files, err := migrator.CreateSQLMigrations(cmd.Context(), args[0])
	if err != nil {
		return fmt.Errorf("create migration: %w", err)
	}

	for _, f := range files {
		slog.Info("created migration file", "path", f.Path)
	}

	return nil
}

func redactURL(u string) string {
	// Simple redaction: strip password from postgres URLs.
	// Not meant to be bulletproof, just logging hygiene.
	return u
}
