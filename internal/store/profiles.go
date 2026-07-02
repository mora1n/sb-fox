package store

import (
	"database/sql"
	"errors"

	"github.com/mora1n/sb-fox/internal/models"
)

const profileCols = `id, name, template_id, options, token, created_at, updated_at`

func scanProfile(sc interface{ Scan(...any) error }) (*models.Profile, error) {
	var p models.Profile
	var created, updated string
	if err := sc.Scan(&p.ID, &p.Name, &p.TemplateID, &p.Options, &p.Token,
		&created, &updated); err != nil {
		return nil, err
	}
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return &p, nil
}

// ListProfiles returns all profiles (without node ids) ordered by name.
func (s *Store) ListProfiles() ([]*models.Profile, error) {
	rows, err := s.db.Query(`SELECT ` + profileCols + ` FROM profiles ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		p.NodeIDs, err = s.profileNodeIDs(p.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetProfile returns one profile with its node ids.
func (s *Store) GetProfile(id int64) (*models.Profile, error) {
	row := s.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE id = ?`, id)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.NodeIDs, err = s.profileNodeIDs(p.ID)
	return p, err
}

// GetProfileByToken returns one profile by its public token.
func (s *Store) GetProfileByToken(token string) (*models.Profile, error) {
	row := s.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE token = ?`, token)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.NodeIDs, err = s.profileNodeIDs(p.ID)
	return p, err
}

func (s *Store) profileNodeIDs(profileID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT node_id FROM profile_nodes WHERE profile_id = ? ORDER BY position`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CreateProfile inserts a profile and its ordered node membership in a tx.
func (s *Store) CreateProfile(p *models.Profile) (int64, error) {
	ts := now()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO profiles (name, template_id, options, token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, p.Name, p.TemplateID, p.Options, p.Token, ts, ts)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := insertProfileNodes(tx, id, p.NodeIDs); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateProfile updates a profile and replaces its node membership.
func (s *Store) UpdateProfile(p *models.Profile) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE profiles SET name=?, template_id=?, options=?, updated_at=?
		WHERE id=?`, p.Name, p.TemplateID, p.Options, now(), p.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM profile_nodes WHERE profile_id = ?`, p.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertProfileNodes(tx, p.ID, p.NodeIDs); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// SetProfileToken replaces a profile's public token.
func (s *Store) SetProfileToken(id int64, token string) error {
	_, err := s.db.Exec(`UPDATE profiles SET token = ?, updated_at = ? WHERE id = ?`, token, now(), id)
	return err
}

// DeleteProfile removes a profile (node membership cascades).
func (s *Store) DeleteProfile(id int64) error {
	_, err := s.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	return err
}

func insertProfileNodes(tx *sql.Tx, profileID int64, nodeIDs []int64) error {
	for pos, nodeID := range nodeIDs {
		if _, err := tx.Exec(`INSERT INTO profile_nodes (profile_id, node_id, position) VALUES (?, ?, ?)`,
			profileID, nodeID, pos); err != nil {
			return err
		}
	}
	return nil
}
