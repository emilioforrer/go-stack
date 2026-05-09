package bundb

import (
	"database/sql"
	"fmt"
	"time"
)

// SQLDBOptions holds connection pool configuration for a *sql.DB.
type SQLDBOptions struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// OpenSQLDB opens a *sql.DB with the given driver and DSN, applying pool options.
func OpenSQLDB(driver, dsn string, opts SQLDBOptions) (*sql.DB, error) {
	sqldb, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("bundb: open sql db: %w", err)
	}

	if opts.MaxOpenConns > 0 {
		sqldb.SetMaxOpenConns(opts.MaxOpenConns)
	}
	if opts.MaxIdleConns > 0 {
		sqldb.SetMaxIdleConns(opts.MaxIdleConns)
	}
	if opts.ConnMaxLifetime > 0 {
		sqldb.SetConnMaxLifetime(opts.ConnMaxLifetime)
	}

	return sqldb, nil
}
