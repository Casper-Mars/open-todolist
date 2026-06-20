package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed 001_init.up.sql
var initSQL string

// DB wraps the SQLite database connection.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at the given path.
// If the database file does not exist, it creates the directory and runs migrations.
func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data directory %s: %w", dir, err)
	}

	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and run migrations
	if _, err := conn.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("set pragma: %w", err)
	}

	if _, err := conn.Exec(initSQL); err != nil {
		conn.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &DB{conn}, nil
}

// DefaultPath returns the default database path: ~/.open-todolist/data.db
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".open-todolist", "data.db"), nil
}
