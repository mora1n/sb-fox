package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const nodeCols = `id, tag, type, server, server_port, country_code, country_source,
	source, source_ref, has_detour, detour, raw, created_at, updated_at`

func scanNode(sc interface{ Scan(...any) error }) (*models.Node, error) {
	var n models.Node
	var created, updated string
	var sourceRef sql.NullInt64
	var hasDetour int
	if err := sc.Scan(&n.ID, &n.Tag, &n.Type, &n.Server, &n.ServerPort, &n.CountryCode,
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
}

// ListNodes returns nodes matching filter, ordered by id.
func (s *Store) ListNodes(f NodeFilter) ([]*models.Node, error) {
	var where []string
	var args []any
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
	q := `SELECT ` + nodeCols + ` FROM nodes`
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
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
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
		VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource,
		n.Source, nullableInt64(n.SourceRef), boolToInt(n.HasDetour), n.Detour, n.Raw, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateNode updates the mutable fields of a node.
func (s *Store) UpdateNode(n *models.Node) error {
	_, err := s.db.Exec(`UPDATE nodes SET tag=?, type=?, server=?, server_port=?,
		country_code=?, country_source=?, has_detour=?, detour=?, raw=?, updated_at=?
		WHERE id=?`,
		n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource,
		boolToInt(n.HasDetour), n.Detour, n.Raw, now(), n.ID)
	return err
}

// DeleteNode removes a node by id.
func (s *Store) DeleteNode(id int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE id = ?`, id)
	return err
}

// DeleteNodesBySource removes all nodes attached to a subscription source.
func (s *Store) DeleteNodesBySource(sourceRef int64) error {
	_, err := s.db.Exec(`DELETE FROM nodes WHERE source = 'subscription' AND source_ref = ?`, sourceRef)
	return err
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
