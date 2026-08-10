// Package db is the SQLite persistence layer: peers and users. Schema is
// unchanged from the original Python app's, so an existing /data/peers.db
// carries over as-is across the rewrite.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

// DB is a handle to the peers/users SQLite database, opened via Open.
type DB struct {
	sql *sql.DB
}

// Open creates the parent directory if needed, opens (or creates) the SQLite
// file at path, and ensures the peers/users tables exist.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("db: create dir %s: %w", dir, err)
		}
	}

	sqlDB, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("db: open %s: %w", path, err)
	}
	// SQLite has no real concurrent-writer story; modernc's driver serializes
	// via its own locking, but capping to one open connection avoids
	// SQLITE_BUSY under concurrent HTTP handlers without adding a busy_timeout
	// dance.
	sqlDB.SetMaxOpenConns(1)

	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db: create schema: %w", err)
	}

	return &DB{sql: sqlDB}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.sql.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS peers (
	public_key   TEXT PRIMARY KEY,
	private_key  TEXT NOT NULL,
	ipv4_address TEXT NOT NULL,
	ipv6_address TEXT NOT NULL,
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	expires_at   TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
	username      TEXT PRIMARY KEY,
	password_hash BLOB NOT NULL,
	created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
`
