package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const nodeGroupCols = `id, owner_user_id, name, description, created_at, updated_at`

func scanNodeGroup(sc interface{ Scan(...any) error }) (*models.NodeGroup, error) {
	var g models.NodeGroup
	var created, updated string
	if err := sc.Scan(&g.ID, &g.OwnerUserID, &g.Name, &g.Description, &created, &updated); err != nil {
		return nil, err
	}
	g.CreatedAt = parseTime(created)
	g.UpdatedAt = parseTime(updated)
	return &g, nil
}

func (s *Store) ListNodeGroups(ownerUserID int64, allOwners bool) ([]*models.NodeGroup, error) {
	q := `SELECT ` + nodeGroupCols + ` FROM node_groups`
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
	var out []*models.NodeGroup
	for rows.Next() {
		g, err := scanNodeGroup(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(out))
	for _, g := range out {
		ids = append(ids, g.ID)
	}
	nodeIDs, err := s.nodeGroupNodeIDsByGroup(ids)
	if err != nil {
		return nil, err
	}
	for _, g := range out {
		g.NodeIDs = nodeIDs[g.ID]
	}
	return out, nil
}

func (s *Store) nodeGroupNodeIDsByGroup(groupIDs []int64) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	if len(groupIDs) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(groupIDs)), ",")
	q := `SELECT group_id, node_id FROM node_group_nodes WHERE group_id IN (` + placeholders + `) ORDER BY group_id, position`
	args := make([]any, 0, len(groupIDs))
	for _, id := range groupIDs {
		args = append(args, id)
	}
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var groupID, nodeID int64
		if err := rows.Scan(&groupID, &nodeID); err != nil {
			return nil, err
		}
		out[groupID] = append(out[groupID], nodeID)
	}
	return out, rows.Err()
}

func (s *Store) GetNodeGroupForUser(id, ownerUserID int64, allOwners bool) (*models.NodeGroup, error) {
	q := `SELECT ` + nodeGroupCols + ` FROM node_groups WHERE id = ?`
	args := []any{id}
	if !allOwners {
		q += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	row := s.db.QueryRow(q, args...)
	g, err := scanNodeGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.NodeIDs, err = s.nodeGroupNodeIDs(g.ID)
	return g, err
}

func (s *Store) GetNodeGroupsForUser(ids []int64, ownerUserID int64, allOwners bool) ([]*models.NodeGroup, error) {
	out := make([]*models.NodeGroup, 0, len(ids))
	for _, id := range ids {
		g, err := s.GetNodeGroupForUser(id, ownerUserID, allOwners)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, nil
}

func (s *Store) CreateNodeGroup(g *models.NodeGroup) (int64, error) {
	ts := now()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`INSERT INTO node_groups (owner_user_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, g.OwnerUserID, g.Name, g.Description, ts, ts)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := insertNodeGroupNodes(tx, id, g.NodeIDs); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) UpdateNodeGroup(g *models.NodeGroup) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	res, err := tx.Exec(`UPDATE node_groups SET name=?, description=?, updated_at=? WHERE id=? AND owner_user_id=?`,
		g.Name, g.Description, now(), g.ID, g.OwnerUserID)
	if err := requireRowsAffected(res, err); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM node_group_nodes WHERE group_id = ?`, g.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertNodeGroupNodes(tx, g.ID, g.NodeIDs); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteNodeGroup(id int64) error {
	_, err := s.db.Exec(`DELETE FROM node_groups WHERE id = ?`, id)
	return err
}

func (s *Store) DeleteNodeGroupForUser(id, ownerUserID int64) error {
	return requireRowsAffected(s.db.Exec(`DELETE FROM node_groups WHERE id = ? AND owner_user_id = ?`, id, ownerUserID))
}

func (s *Store) nodeGroupNodeIDs(groupID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT node_id FROM node_group_nodes WHERE group_id = ? ORDER BY position`, groupID)
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

func insertNodeGroupNodes(tx *sql.Tx, groupID int64, nodeIDs []int64) error {
	for pos, nodeID := range nodeIDs {
		if _, err := tx.Exec(`INSERT INTO node_group_nodes (group_id, node_id, position) VALUES (?, ?, ?)`,
			groupID, nodeID, pos); err != nil {
			return err
		}
	}
	return nil
}
