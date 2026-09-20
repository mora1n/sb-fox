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

// SyncSubscriptionNodes applies a source refresh atomically. Existing nodes
// are updated in place, new nodes are inserted, and stale nodes are removed
// with the same profile/group reference cleanup used by normal deletion.
func (s *Store) SyncSubscriptionNodes(ownerUserID int64, updates, creates []*models.Node, deleteIDs []int64) ([]int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	rollback := func(err error) ([]int64, error) { _ = tx.Rollback(); return nil, err }
	for _, n := range updates {
		if err := requireRowsAffected(tx.Exec(`UPDATE nodes SET tag=?, type=?, server=?, server_port=?, country_code=?, country_source=?, has_detour=?, detour=?, raw=?, updated_at=? WHERE id=? AND owner_user_id=?`,
			n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource, boolToInt(n.HasDetour), n.Detour, n.Raw, now(), n.ID, ownerUserID)); err != nil {
			return rollback(err)
		}
	}
	createdIDs := make([]int64, 0, len(creates))
	for _, n := range creates {
		res, err := tx.Exec(`INSERT INTO nodes (owner_user_id, tag, type, server, server_port, country_code, country_source, source, source_ref, has_detour, detour, raw, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			n.OwnerUserID, n.Tag, n.Type, n.Server, n.ServerPort, n.CountryCode, n.CountrySource, n.Source, nullableInt64(n.SourceRef), boolToInt(n.HasDetour), n.Detour, n.Raw, now(), now())
		if err != nil {
			return rollback(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return rollback(err)
		}
		createdIDs = append(createdIDs, id)
	}
	if len(deleteIDs) > 0 {
		sourceIDs, err := subscriptionSourceIDs(tx, deleteIDs, &ownerUserID)
		if err != nil {
			return rollback(err)
		}
		query, args := scopedIDOwnerQuery("nodes", deleteIDs, &ownerUserID)
		owners, err := nodeOwners(tx, query, args, len(deleteIDs))
		if err != nil {
			return rollback(err)
		}
		emptyGroups, err := emptyNodeGroups(tx, deleteIDs, &ownerUserID)
		if err != nil {
			return rollback(err)
		}
		updates, err := cleanProfileReferences(tx, owners, deleteIDs, emptyGroups)
		if err != nil {
			return rollback(err)
		}
		if err := applyProfileOptionsUpdates(tx, updates); err != nil {
			return rollback(err)
		}
		if _, err := deleteIDsInTx(tx, "nodes", deleteIDs, &ownerUserID); err != nil {
			return rollback(err)
		}
		if len(emptyGroups) > 0 {
			if _, err := deleteIDsInTx(tx, "node_groups", emptyGroups, &ownerUserID); err != nil {
				return rollback(err)
			}
		}
		emptySources, err := cleanupSubscriptionSources(tx, sourceIDs, &ownerUserID)
		if err != nil {
			return rollback(err)
		}
		if err := clearSubscriptionSourceExclusions(tx, emptySources); err != nil {
			return rollback(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return createdIDs, nil
}

func subscriptionSourceIDs(tx *sql.Tx, nodeIDs []int64, ownerUserID *int64) ([]int64, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(nodeIDs)), ",")
	args := int64Args(nodeIDs)
	query := `SELECT DISTINCT source_ref FROM nodes WHERE source = 'subscription' AND source_ref IS NOT NULL AND id IN (` + placeholders + `)`
	if ownerUserID != nil {
		query += ` AND owner_user_id = ?`
		args = append(args, *ownerUserID)
	}
	rows, err := tx.Query(query, args...)
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

type subscriptionSourceDeletion struct {
	sourceID int64
	raw      string
}

func subscriptionSourceDeletions(tx *sql.Tx, nodeIDs []int64, ownerUserID *int64) ([]subscriptionSourceDeletion, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(nodeIDs)), ",")
	args := int64Args(nodeIDs)
	query := `SELECT source_ref, raw FROM nodes WHERE source = 'subscription' AND source_ref IS NOT NULL AND id IN (` + placeholders + `)`
	if ownerUserID != nil {
		query += ` AND owner_user_id = ?`
		args = append(args, *ownerUserID)
	}
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []subscriptionSourceDeletion
	for rows.Next() {
		var item subscriptionSourceDeletion
		if err := rows.Scan(&item.sourceID, &item.raw); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func sourceIDsFromDeletions(items []subscriptionSourceDeletion) []int64 {
	seen := make(map[int64]bool, len(items))
	var out []int64
	for _, item := range items {
		if !seen[item.sourceID] {
			seen[item.sourceID] = true
			out = append(out, item.sourceID)
		}
	}
	return out
}

func recordSubscriptionDeletions(tx *sql.Tx, items []subscriptionSourceDeletion) error {
	for _, item := range items {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO subscription_source_exclusions (source_id, raw) VALUES (?, ?)`, item.sourceID, item.raw); err != nil {
			return err
		}
	}
	return nil
}

func cleanupSubscriptionSources(tx *sql.Tx, sourceIDs []int64, ownerUserID *int64) ([]int64, error) {
	var empty []int64
	for _, sourceID := range sourceIDs {
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM nodes WHERE source = 'subscription' AND source_ref = ?`, sourceID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			empty = append(empty, sourceID)
		}
		query := `UPDATE subscription_sources SET node_count = ? WHERE id = ?`
		args := []any{count, sourceID}
		if ownerUserID != nil {
			query += ` AND owner_user_id = ?`
			args = append(args, *ownerUserID)
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return nil, err
		}
	}
	return empty, nil
}

// ListSubscriptionSourceExclusions returns raw outbounds explicitly removed by the user.
func (s *Store) ListSubscriptionSourceExclusions(sourceID int64) ([]string, error) {
	rows, err := s.db.Query(`SELECT raw FROM subscription_source_exclusions WHERE source_id = ?`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, rows.Err()
}

func (s *Store) ClearSubscriptionSourceExclusions(sourceID int64) error {
	_, err := s.db.Exec(`DELETE FROM subscription_source_exclusions WHERE source_id = ?`, sourceID)
	return err
}

func clearSubscriptionSourceExclusions(tx *sql.Tx, sourceIDs []int64) error {
	for _, sourceID := range sourceIDs {
		if _, err := tx.Exec(`DELETE FROM subscription_source_exclusions WHERE source_id = ?`, sourceID); err != nil {
			return err
		}
	}
	return nil
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

func (s *Store) DeleteNodeForUserDetailed(id, ownerUserID int64) (NodeDeleteResult, error) {
	return s.deleteNodesWithReferencesDetailed([]int64{id}, &ownerUserID)
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
	ids, err := s.nodeIDsBySource(sourceRef, nil)
	if err != nil {
		return err
	}
	_, err = s.deleteNodesWithReferences(ids, nil)
	return err
}

func (s *Store) DeleteNodesBySourceForUser(sourceRef, ownerUserID int64) error {
	ids, err := s.nodeIDsBySource(sourceRef, &ownerUserID)
	if err != nil {
		return err
	}
	_, err = s.deleteNodesWithReferences(ids, &ownerUserID)
	return err
}

func (s *Store) nodeIDsBySource(sourceRef int64, ownerUserID *int64) ([]int64, error) {
	query := `SELECT id FROM nodes WHERE source = 'subscription' AND source_ref = ?`
	args := []any{sourceRef}
	if ownerUserID != nil {
		query += ` AND owner_user_id = ?`
		args = append(args, *ownerUserID)
	}
	rows, err := s.db.Query(query, args...)
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
