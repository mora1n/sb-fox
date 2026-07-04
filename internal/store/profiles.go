package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const profileCols = `id, owner_user_id, name, template_id, options, subscription_enabled, token, created_at, updated_at`

func scanProfile(sc interface{ Scan(...any) error }) (*models.Profile, error) {
	var p models.Profile
	var created, updated string
	var enabled int
	if err := sc.Scan(&p.ID, &p.OwnerUserID, &p.Name, &p.TemplateID, &p.Options, &enabled, &p.Token,
		&created, &updated); err != nil {
		return nil, err
	}
	p.SubEnabled = enabled != 0
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return &p, nil
}

// ListProfiles returns all profiles (without node ids) ordered by name.
func (s *Store) ListProfiles(ownerUserID int64, allOwners bool) ([]*models.Profile, error) {
	q := `SELECT ` + profileCols + ` FROM profiles`
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
	var out []*models.Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(out))
	for _, p := range out {
		ids = append(ids, p.ID)
	}
	nodeIDs, err := s.profileMembershipIDs(ids, "profile_nodes", "node_id")
	if err != nil {
		return nil, err
	}
	groupIDs, err := s.profileMembershipIDs(ids, "profile_node_groups", "group_id")
	if err != nil {
		return nil, err
	}
	for _, p := range out {
		p.NodeIDs = nodeIDs[p.ID]
		p.NodeGroupIDs = groupIDs[p.ID]
	}
	return out, nil
}

func (s *Store) profileMembershipIDs(profileIDs []int64, table, idCol string) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	if len(profileIDs) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(profileIDs)), ",")
	q := `SELECT profile_id, ` + idCol + ` FROM ` + table + ` WHERE profile_id IN (` + placeholders + `) ORDER BY profile_id, position`
	args := make([]any, 0, len(profileIDs))
	for _, id := range profileIDs {
		args = append(args, id)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var profileID, id int64
		if err := rows.Scan(&profileID, &id); err != nil {
			return nil, err
		}
		out[profileID] = append(out[profileID], id)
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
	if err != nil {
		return nil, err
	}
	p.NodeGroupIDs, err = s.profileNodeGroupIDs(p.ID)
	return p, err
}

func (s *Store) GetProfileForUser(id, ownerUserID int64, allOwners bool) (*models.Profile, error) {
	q := `SELECT ` + profileCols + ` FROM profiles WHERE id = ?`
	args := []any{id}
	if !allOwners {
		q += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	row := s.db.QueryRow(q, args...)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.NodeIDs, err = s.profileNodeIDs(p.ID)
	if err != nil {
		return nil, err
	}
	p.NodeGroupIDs, err = s.profileNodeGroupIDs(p.ID)
	return p, err
}

func (s *Store) GetProfileByNameForUser(name string, ownerUserID int64) (*models.Profile, error) {
	row := s.db.QueryRow(`SELECT `+profileCols+` FROM profiles WHERE owner_user_id = ? AND name = ?`,
		ownerUserID, strings.TrimSpace(name))
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.NodeIDs, err = s.profileNodeIDs(p.ID)
	if err != nil {
		return nil, err
	}
	p.NodeGroupIDs, err = s.profileNodeGroupIDs(p.ID)
	return p, err
}

func (s *Store) profileNodeIDs(profileID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT node_id FROM profile_nodes WHERE profile_id = ? ORDER BY position`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) profileNodeGroupIDs(profileID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT group_id FROM profile_node_groups WHERE profile_id = ? ORDER BY position`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
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
	if !p.SubEnabledSet {
		p.SubEnabled = true
	}
	if strings.TrimSpace(p.Token) == "" {
		token, err := randomToken()
		if err != nil {
			return 0, err
		}
		p.Token = token
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO profiles (owner_user_id, name, template_id, options, subscription_enabled, token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, p.OwnerUserID, p.Name, p.TemplateID, p.Options, boolInt(p.SubEnabled), p.Token, ts, ts)
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
	if err := insertProfileNodeGroups(tx, id, p.NodeGroupIDs); err != nil {
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
	if _, err := tx.Exec(`UPDATE profiles SET name=?, template_id=?, options=?, subscription_enabled=?, updated_at=?
		WHERE id=?`, p.Name, p.TemplateID, p.Options, boolInt(p.SubEnabled), now(), p.ID); err != nil {
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
	if _, err := tx.Exec(`DELETE FROM profile_node_groups WHERE profile_id = ?`, p.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertProfileNodeGroups(tx, p.ID, p.NodeGroupIDs); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// DeleteProfile removes a profile (node membership cascades).
func (s *Store) DeleteProfile(id int64) error {
	_, err := s.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	return err
}

func (s *Store) SetProfileSubscriptionEnabled(id int64, enabled bool) error {
	_, err := s.db.Exec(`UPDATE profiles SET subscription_enabled = ?, updated_at = ? WHERE id = ?`, boolInt(enabled), now(), id)
	return err
}

func (s *Store) CountProfiles(ownerUserID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM profiles WHERE owner_user_id = ?`, ownerUserID).Scan(&n)
	return n, err
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

func insertProfileNodeGroups(tx *sql.Tx, profileID int64, groupIDs []int64) error {
	for pos, groupID := range groupIDs {
		if _, err := tx.Exec(`INSERT INTO profile_node_groups (profile_id, group_id, position) VALUES (?, ?, ?)`,
			profileID, groupID, pos); err != nil {
			return err
		}
	}
	return nil
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
