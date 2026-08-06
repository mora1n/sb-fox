package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const nodeCols = `id, owner_user_id, tag, type, server, server_port, country_code, country_source,
	source, source_ref, has_detour, detour, raw, created_at, updated_at`

func scanNode(sc interface{ Scan(...any) error }) (*models.Node, error) {
	var n models.Node
	var created, updated string
	var sourceRef sql.NullInt64
	var hasDetour int
	if err := sc.Scan(&n.ID, &n.OwnerUserID, &n.Tag, &n.Type, &n.Server, &n.ServerPort, &n.CountryCode,
		&n.CountrySource, &n.Source, &sourceRef, &hasDetour, &n.Detour, &n.Raw,
		&created, &updated); err != nil {
		return nil, err
	}
	if sourceRef.Valid {
		n.SourceRef = &sourceRef.Int64
	}
	n.HasDetour = hasDetour != 0
	n.CreatedAt = parseTime(created)
	n.UpdatedAt = parseTime(updated)
	return &n, nil
}

// NodeFilter narrows ListNodes results. Empty fields are ignored.
type NodeFilter struct {
	Source      string
	CountryCode string
	Type        string
	Search      string // substring match on tag
	OwnerUserID int64
	AllOwners   bool
	OmitRaw     bool
}

// ListNodes returns nodes matching filter, ordered by id.
func (s *Store) ListNodes(f NodeFilter) ([]*models.Node, error) {
	var where []string
	var args []any
	if !f.AllOwners {
		where = append(where, "owner_user_id = ?")
		args = append(args, f.OwnerUserID)
	} else if f.OwnerUserID != 0 {
		where = append(where, "owner_user_id = ?")
		args = append(args, f.OwnerUserID)
	}
	if f.Source != "" {
		where = append(where, "source = ?")
		args = append(args, f.Source)
	}
	if f.CountryCode != "" {
		where = append(where, "country_code = ?")
		args = append(args, f.CountryCode)
	}
	if f.Type != "" {
		where = append(where, "type = ?")
		args = append(args, f.Type)
	}
	if f.Search != "" {
		where = append(where, "tag LIKE ?")
		args = append(args, "%"+f.Search+"%")
	}
	cols := nodeCols
	if f.OmitRaw {
		cols = `id, owner_user_id, tag, type, server, server_port, country_code, country_source,
	source, source_ref, has_detour, detour, created_at, updated_at`
	}
	q := `SELECT ` + cols + ` FROM nodes`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY id"

	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Node
	for rows.Next() {
		var n *models.Node
		var err error
		if f.OmitRaw {
			n, err = scanNodeSummary(rows)
		} else {
			n, err = scanNode(rows)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func scanNodeSummary(sc interface{ Scan(...any) error }) (*models.Node, error) {
	var n models.Node
	var created, updated string
	var sourceRef sql.NullInt64
	var hasDetour int
	if err := sc.Scan(&n.ID, &n.OwnerUserID, &n.Tag, &n.Type, &n.Server, &n.ServerPort, &n.CountryCode,
		&n.CountrySource, &n.Source, &sourceRef, &hasDetour, &n.Detour, &created, &updated); err != nil {
		return nil, err
	}
	if sourceRef.Valid {
		n.SourceRef = &sourceRef.Int64
	}
	n.HasDetour = hasDetour != 0
	n.CreatedAt = parseTime(created)
	n.UpdatedAt = parseTime(updated)
	return &n, nil
}

// GetNode returns one node by id.
func (s *Store) GetNode(id int64) (*models.Node, error) {
	row := s.db.QueryRow(`SELECT `+nodeCols+` FROM nodes WHERE id = ?`, id)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

func (s *Store) GetNodeForUser(id, ownerUserID int64, allOwners bool) (*models.Node, error) {
	q := `SELECT ` + nodeCols + ` FROM nodes WHERE id = ?`
	args := []any{id}
	if !allOwners {
		q += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	row := s.db.QueryRow(q, args...)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return n, err
}

// GetNodes returns nodes for the given ids, preserving the id order requested.
func (s *Store) GetNodes(ids []int64) ([]*models.Node, error) {
	out := make([]*models.Node, 0, len(ids))
	for _, id := range ids {
		n, err := s.GetNode(id)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// CreateNode inserts a node and returns its id.
func (s *Store) CreateNode(n *models.Node) (int64, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO nodes (`+nodeCols+`)
		VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.OwnerUserID, n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource,
		n.Source, nullableInt64(n.SourceRef), boolToInt(n.HasDetour), n.Detour, n.Raw, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateNode updates the mutable fields of a node.
func (s *Store) UpdateNode(n *models.Node) error {
	return requireRowsAffected(s.db.Exec(`UPDATE nodes SET tag=?, type=?, server=?, server_port=?,
		country_code=?, country_source=?, has_detour=?, detour=?, raw=?, updated_at=?
		WHERE id=? AND owner_user_id=?`,
		n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource,
		boolToInt(n.HasDetour), n.Detour, n.Raw, now(), n.ID, n.OwnerUserID))
}

// DeleteNode removes a node by id.
func (s *Store) DeleteNode(id int64) error {
	_, err := s.deleteNodesWithReferences([]int64{id}, nil)
	return err
}

func (s *Store) DeleteNodeForUser(id, ownerUserID int64) error {
	_, err := s.deleteNodesWithReferences([]int64{id}, &ownerUserID)
	return err
}

func (s *Store) ListNodeUsage(id, ownerUserID int64, allOwners bool) ([]*models.NodeUsage, error) {
	var out []*models.NodeUsage
	direct := `SELECT p.id, p.name FROM profiles p
		JOIN profile_nodes pn ON pn.profile_id = p.id
		WHERE pn.node_id = ?`
	args := []any{id}
	if !allOwners {
		direct += ` AND p.owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	direct += ` ORDER BY p.name`
	rows, err := s.db.Query(direct, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		u := &models.NodeUsage{}
		if err := rows.Scan(&u.ProfileID, &u.ProfileName); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	viaGroup := `SELECT p.id, p.name, ng.id, ng.name FROM profiles p
		JOIN profile_node_groups png ON png.profile_id = p.id
		JOIN node_groups ng ON ng.id = png.group_id
		JOIN node_group_nodes ngn ON ngn.group_id = ng.id
		WHERE ngn.node_id = ?`
	args = []any{id}
	if !allOwners {
		viaGroup += ` AND p.owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	viaGroup += ` ORDER BY p.name, ng.name`
	rows, err = s.db.Query(viaGroup, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		u := &models.NodeUsage{}
		if err := rows.Scan(&u.ProfileID, &u.ProfileName, &u.ViaGroupID, &u.ViaGroupName); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// DeleteNodesBySource removes all nodes attached to a subscription source.
func (s *Store) DeleteNodesBySource(sourceRef int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE source = 'subscription' AND source_ref = ?`, sourceRef)
	return err
}

func (s *Store) DeleteNodesBySourceForUser(sourceRef, ownerUserID int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE source = 'subscription' AND source_ref = ? AND owner_user_id = ?`, sourceRef, ownerUserID)
	return err
}

func (s *Store) CountNodes(ownerUserID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE owner_user_id = ?`, ownerUserID).Scan(&n)
	return n, err
}

func (s *Store) CountNodesBySource(sourceRef int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM nodes WHERE source = 'subscription' AND source_ref = ?`, sourceRef).Scan(&n)
	return n, err
}

func nullableInt64(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
