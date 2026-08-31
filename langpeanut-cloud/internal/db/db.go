// Package db provides the SQLite access layer for langpeanut-cloud.
// Migrations are plain versioned .sql files embedded at compile time via go:embed.
// WAL mode and foreign key enforcement are turned on at every connection open.
package db

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps *sql.DB and owns the connection lifecycle.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite file at path, enables WAL + foreign keys,
// and applies all pending migrations in version order.
func Open(path string) (*DB, error) {
	// go-sqlite3 DSN: enable WAL and FK enforcement via query params.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000", path)
	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}
	// SQLite is single-writer; allow one writer + a small read pool.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(time.Hour)

	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db migrate: %w", err)
	}
	return db, nil
}

// migrate creates the schema_migrations table (if missing) and applies any
// migration files that haven't been recorded yet, in ascending filename order.
func (db *DB) migrate() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
	)`)
	if err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	// Sort ascending so migrations apply in order.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := strings.TrimSuffix(entry.Name(), ".sql")

		var applied string
		err := db.QueryRow("SELECT version FROM schema_migrations WHERE version=?", version).Scan(&applied)
		if err == nil {
			continue // already applied
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", version, err)
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		// Strip the leading comment line we add to each file for readability.
		sql := stripFirstLineIfComment(string(content))

		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := db.Exec("INSERT INTO schema_migrations(version) VALUES(?)", version); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
	}
	return nil
}

func stripFirstLineIfComment(s string) string {
	if strings.HasPrefix(s, "--") {
		idx := strings.Index(s, "\n")
		if idx != -1 {
			return s[idx+1:]
		}
	}
	return s
}
