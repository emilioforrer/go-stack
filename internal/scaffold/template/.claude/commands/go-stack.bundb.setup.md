---
description: Instructions to setup the go-stack BunDB integration for database access,
  migration management, and the bundled DB CLI command.
---

# Setup BunDB Integration for `go-stack`

## 1. Install the `pkg/bundb` Go module

Add it to your project's `go.mod`:

```bash
go get github.com/emilioforrer/go-stack/pkg/bundb@latest
```

## 2. Add the `bun` CLI command to your application

Import the `buncmd` package and register the command in `cmd/app/root.go` inside `initCommands`:

```go
import "github.com/emilioforrer/go-stack/pkg/bundb/buncmd"

func initCommands(rootCmd *cobra.Command) {
	// ... existing flag setup ...

	var hasVersion bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "version" {
			hasVersion = true
		}
	}
	if !hasVersion {
		rootCmd.AddCommand(newVersionCmd())
		rootCmd.AddCommand(newCompletionCmd())
		rootCmd.AddCommand(serveCmd)
		// Add the BunDB command
		rootCmd.AddCommand(buncmd.NewCommand("bun"))
	}
}
```

## 3. Wire the BunDB provider into your boot setup

In your `cmd/app/serve.go` file, import the bundb package and add the BunProvider to your boot setup:

```go
import (
	"database/sql"
	"time"

	"github.com/emilioforrer/go-stack/pkg/bundb"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func runServe(ctx context.Context, _ *cobra.Command) error {
	// ... your existing code ...

	// Example: open a *sql.DB and wrap it with bun
	pool, err := pgxpool.New(context.Background(), "postgres://user:pass@localhost/db")
	if err != nil {
		return fmt.Errorf("database: failed to create connection pool: %w", err)
	}
	sqldb := stdlib.OpenDBFromPool(pool)

	// OR Open a *sql.DB directly with stdlib and wrap it with bun
	// sqldb, err := bundb.OpenSQLDB("pgx", "postgres://user:pass@localhost/db", bundb.SQLDBOptions{
	// 	MaxOpenConns:    25,
	// 	MaxIdleConns:    5,
	// 	ConnMaxLifetime: time.Hour,
	// })
	// if err != nil {
	// 	return fmt.Errorf("open database: %w", err)
	// }

	// Ensure the database is closed on application shutdown
	defer sqldb.Close()

	bootstrapper := newBootstrapper(
		i,
		provider.NewServerProvider(opts),
		// Add the BunProvider after the ServerProvider
		bundb.NewBunProvider(bundb.ProviderOptions{
			SQLDB:   sqldb,
			Dialect: pgdialect.New(),
		}),
	)
	// ... your existing code ...
}
```

> **Note:** If you already have a `*pgxpool.Pool` from another provider, pass it via `ProviderOptions.PGXPool` instead of `SQLDB`, and set the `Dialect` accordingly.
>
> **Note:** The connection pool settings (`MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`) are configured when opening the `*sql.DB`. They do not need to be repeated in `ProviderOptions` unless `BunProvider` maintains its own internal pool.
>
> **Note:** The `pgx` driver (`github.com/jackc/pgx/v5`) is required and is typically pulled in automatically as a transitive dependency of `pkg/bundb`. If it is missing from your module graph, add it explicitly with `go get github.com/jackc/pgx/v5`.

## 4. Create the migrations directory

Create a `migrations` directory in your project's root if it does not already exist:

```bash
mkdir -p migrations
```

## 5. Tidy up

```bash
go mod tidy
```

## 6. Verify the setup

Confirm the CLI command is registered and BunDB is wired correctly:

```bash
# List available commands — "bun" should appear
go run ./cmd/app --help

# Verify the bun subcommand works
go run ./cmd/app bun --help
```

## Additional References

- [Bun Documentation](https://bun.uptrace.dev/)
- [Bun Migrations](https://bun.uptrace.dev/guide/migrations.html)
- [pgx Documentation](https://github.com/jackc/pgx)
- [go-stack boot package](https://github.com/emilioforrer/go-stack/tree/main/pkg/boot)