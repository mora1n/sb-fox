// Package store is the SQLite persistence layer for sb-fox, using the pure-Go
// modernc.org/sqlite driver (no CGO) so the binary cross-compiles cleanly.
package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the database handle and provides typed accessors.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, enables WAL and
// foreign keys, and applies pending migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite writer serialization; simplest correct choice
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the raw handle for advanced callers (e.g. health checks).
func (s *Store) DB() *sql.DB { return s.db }

func requireRowsAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// migrate applies any migrations not yet recorded in schema_migrations.
func (s *Store) migrate() error {
	// The bookkeeping table (migration 0) is created unconditionally first.
	if _, err := s.db.Exec(migrations[0]); err != nil {
		return fmt.Errorf("store: create schema_migrations: %w", err)
	}

	applied, err := s.appliedVersions()
	if err != nil {
		return err
	}

	for i := 1; i < len(migrations); i++ {
		version := i
		if applied[version] {
			continue
		}
		disableFK := strings.Contains(migrations[i], "-- sb-fox:disable-foreign-keys")
		if disableFK {
			if _, err := s.db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
				return fmt.Errorf("store: disable foreign keys for migration %d: %w", version, err)
			}
		}
		tx, err := s.db.Begin()
		if err != nil {
			if disableFK {
				_, _ = s.db.Exec(`PRAGMA foreign_keys = ON`)
			}
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			_ = tx.Rollback()
			if disableFK {
				_, _ = s.db.Exec(`PRAGMA foreign_keys = ON`)
			}
			return fmt.Errorf("store: migration %d: %w", version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES(?, ?)`,
			version, now()); err != nil {
			_ = tx.Rollback()
			if disableFK {
				_, _ = s.db.Exec(`PRAGMA foreign_keys = ON`)
			}
			return fmt.Errorf("store: record migration %d: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			if disableFK {
				_, _ = s.db.Exec(`PRAGMA foreign_keys = ON`)
			}
			return err
		}
		if disableFK {
			if _, err := s.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
				return fmt.Errorf("store: re-enable foreign keys after migration %d: %w", version, err)
			}
		}
	}
	return nil
}

func (s *Store) appliedVersions() (map[int]bool, error) {
	rows, err := s.db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// now returns the current time as an RFC3339 string (storage format).
func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// parseTime parses a stored RFC3339 timestamp, returning zero time on error.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
