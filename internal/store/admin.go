package store

import (
	"database/sql"
	"errors"

	"github.com/mora1n/sb-fox/internal/models"
)

// ErrNotFound is returned when a lookup finds no row.
var ErrNotFound = errors.New("store: not found")

// GetAdmin returns the single admin row, or ErrNotFound if unset.
func (s *Store) GetAdmin() (*models.Admin, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, created_at, updated_at FROM admin WHERE id = 1`)
	var a models.Admin
	var created, updated string
	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.CreatedAt = parseTime(created)
	a.UpdatedAt = parseTime(updated)
	return &a, nil
}

// SetAdmin creates or replaces the admin row (id = 1) with the given username
// and bcrypt hash.
func (s *Store) SetAdmin(username, passwordHash string) error {
	ts := now()
	_, err := s.db.Exec(`
		INSERT INTO admin (id, username, password_hash, created_at, updated_at)
		VALUES (1, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			password_hash = excluded.password_hash,
			updated_at = excluded.updated_at`,
		username, passwordHash, ts, ts)
	return err
}

// AdminExists reports whether the admin row is set.
func (s *Store) AdminExists() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM admin WHERE id = 1`).Scan(&n)
	return n > 0, err
}
