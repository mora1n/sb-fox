package store

import (
	"database/sql"
	"errors"
	"time"

	"github.com/mora1n/sb-fox/internal/models"
)

const sourceCols = `id, owner_user_id, name, url, last_fetch_at, last_status, node_count, created_at`

func scanSource(sc interface{ Scan(...any) error }) (*models.SubscriptionSource, error) {
	var s models.SubscriptionSource
	var created string
	var lastFetch sql.NullString
	if err := sc.Scan(&s.ID, &s.OwnerUserID, &s.Name, &s.URL, &lastFetch, &s.LastStatus, &s.NodeCount, &created); err != nil {
		return nil, err
	}
	if lastFetch.Valid {
		t := parseTime(lastFetch.String)
		s.LastFetchAt = &t
	}
	s.CreatedAt = parseTime(created)
	return &s, nil
}

// ListSources returns all subscription sources ordered by name.
func (s *Store) ListSources(ownerUserID int64, allOwners bool) ([]*models.SubscriptionSource, error) {
	q := `SELECT ` + sourceCols + ` FROM subscription_sources`
	var args []any
	if !allOwners {
		q += ` WHERE owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	q += ` ORDER BY name`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.SubscriptionSource
	for rows.Next() {
		src, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, src)
	}
	return out, rows.Err()
}

// GetSource returns one subscription source by id.
func (s *Store) GetSource(id int64) (*models.SubscriptionSource, error) {
	row := s.db.QueryRow(`SELECT `+sourceCols+` FROM subscription_sources WHERE id = ?`, id)
	src, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return src, err
}

// CreateSource inserts a subscription source and returns its id.
func (s *Store) GetSourceForUser(id, ownerUserID int64, allOwners bool) (*models.SubscriptionSource, error) {
	q := `SELECT ` + sourceCols + ` FROM subscription_sources WHERE id = ?`
	args := []any{id}
	if !allOwners {
		q += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	row := s.db.QueryRow(q, args...)
	src, err := scanSource(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return src, err
}

// CreateSource inserts a subscription source and returns its id.
func (s *Store) CreateSource(ownerUserID int64, name, url string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO subscription_sources (owner_user_id, name, url, last_status, node_count, created_at)
		VALUES (?, ?, ?, '', 0, ?)`, ownerUserID, name, url, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateSourceFetch records the outcome of a fetch attempt.
func (s *Store) UpdateSourceFetch(id int64, status string, nodeCount int) error {
	_, err := s.db.Exec(`UPDATE subscription_sources SET last_fetch_at = ?, last_status = ?, node_count = ?
		WHERE id = ?`, time.Now().UTC().Format(time.RFC3339Nano), status, nodeCount, id)
	return err
}

// DeleteSource removes a subscription source (nodes' source_ref is set null).
func (s *Store) DeleteSource(id int64) error {
	_, err := s.db.Exec(`DELETE FROM subscription_sources WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteSourceForUser(id, ownerUserID int64) error {
	return requireRowsAffected(s.db.Exec(`DELETE FROM subscription_sources WHERE id = ? AND owner_user_id = ?`, id, ownerUserID))
}
