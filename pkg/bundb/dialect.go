package bundb

import (
	"fmt"

	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/schema"
)

// ParseDialect returns a schema.Dialect for the given name.
// Supported names: pg, postgres, postgresql, mysql, sqlite, sqlite3.
func ParseDialect(name string) (schema.Dialect, error) {
	switch name {
	case "pg", "postgres", "postgresql":
		return pgdialect.New(), nil
	case "mysql":
		return mysqldialect.New(), nil
	case "sqlite", "sqlite3":
		return sqlitedialect.New(), nil
	default:
		return nil, fmt.Errorf("bundb: unsupported dialect %q", name)
	}
}

// DefaultDriver returns a default database/sql driver name for the given dialect.
// This is a convention, not a guarantee that the driver is registered.
func DefaultDriver(dialect string) string {
	switch dialect {
	case "pg", "postgres", "postgresql":
		return "pgx"
	case "mysql":
		return "mysql"
	case "sqlite", "sqlite3":
		return "sqlite"
	default:
		return dialect
	}
}
