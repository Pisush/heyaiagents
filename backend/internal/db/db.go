// Package db owns the SQLite connection and data-access helpers for the
// HeyAI Agents backend. SQL is kept Postgres-compatible so the store can be
// swapped later; sqlc-generated query code will live alongside this file.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// modernc.org/sqlite is a pure-Go SQLite driver (no cgo), registered under
	// the driver name "sqlite".
	_ "modernc.org/sqlite"
)

// Open opens (and lazily creates) the SQLite database at path and verifies the
// connection with a ping.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite at %q: %w", path, err)
	}

	// SQLite handles a single writer at a time; keep the pool small and enforce
	// foreign keys for the schema added in later milestones.
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := Ping(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Ping verifies the database is reachable within a short timeout.
func Ping(conn *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}
