// Package store provides the data access layer backed by PostgreSQL. It
// combines sqlc-generated CRUD operations with hand-written custom queries
// for pagination, filtering, search, and partial updates.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// NewDB opens a PostgreSQL connection pool, configures connection limits and
// timeouts, and verifies reachability with a ping. Closing the returned *sql.DB
// is the caller's responsibility.
func NewDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	if err := db.PingContext(context.Background()); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}
