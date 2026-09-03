package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type profileOptionsUpdate struct {
	id      int64
	options string
}

func (s *Store) deleteNodesWithReferences(ids []int64, ownerUserID *int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := int64Args(ids)
	query := `SELECT id, owner_user_id FROM nodes WHERE id IN (` + placeholders + `)`
	if ownerUserID != nil {
		query += ` AND owner_user_id = ?`
		args = append(args, *ownerUserID)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	owners, err := nodeOwners(tx, query, args, len(ids))
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	emptyGroups, err := emptyNodeGroups(tx, ids, ownerUserID)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	updates, err := cleanProfileReferences(tx, owners, ids, emptyGroups)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	for _, update := range updates {
		if err := requireRowsAffected(tx.Exec(`UPDATE profiles SET options = ?, updated_at = ? WHERE id = ?`,
			update.options, now(), update.id)); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}

	deleteQuery := `DELETE FROM nodes WHERE id IN (` + placeholders + `)`
	deleteArgs := int64Args(ids)
	if ownerUserID != nil {
		deleteQuery += ` AND owner_user_id = ?`
		deleteArgs = append(deleteArgs, *ownerUserID)
	}
	res, err := tx.Exec(deleteQuery, deleteArgs...)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if affected != int64(len(ids)) {
		_ = tx.Rollback()
		return 0, ErrNotFound
	}
	if len(emptyGroups) > 0 {
		groupPlaceholders := strings.TrimRight(strings.Repeat("?,", len(emptyGroups)), ",")
		if _, err := tx.Exec(`DELETE FROM node_groups WHERE id IN (`+groupPlaceholders+`)`, int64Args(emptyGroups)...); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(affected), nil
}

func (s *Store) deleteNodeGroupsWithReferences(ids []int64, ownerUserID *int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := int64Args(ids)
	query := `SELECT id, owner_user_id FROM node_groups WHERE id IN (` + placeholders + `)`
	if ownerUserID != nil {
		query += ` AND owner_user_id = ?`
		args = append(args, *ownerUserID)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	owners, err := nodeOwners(tx, query, args, len(ids))
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	updates, err := cleanProfileReferences(tx, owners, nil, ids)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	for _, update := range updates {
		if err := requireRowsAffected(tx.Exec(`UPDATE profiles SET options = ?, updated_at = ? WHERE id = ?`,
			update.options, now(), update.id)); err != nil {
			_ = tx.Rollback()
			return 0, err
		}
	}
	deleteQuery := `DELETE FROM node_groups WHERE id IN (` + placeholders + `)`
	deleteArgs := int64Args(ids)
	if ownerUserID != nil {
		deleteQuery += ` AND owner_user_id = ?`
		deleteArgs = append(deleteArgs, *ownerUserID)
	}
	res, err := tx.Exec(deleteQuery, deleteArgs...)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if affected != int64(len(ids)) {
		_ = tx.Rollback()
		return 0, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(affected), nil
}

func emptyNodeGroups(tx *sql.Tx, nodeIDs []int64, ownerUserID *int64) ([]int64, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(nodeIDs)), ",")
	args := int64Args(nodeIDs)
	query := `SELECT ngn.group_id FROM node_group_nodes ngn
		JOIN node_groups ng ON ng.id = ngn.group_id
		WHERE EXISTS (SELECT 1 FROM node_group_nodes affected
			WHERE affected.group_id = ngn.group_id AND affected.node_id IN (` + placeholders + `))`
	if ownerUserID != nil {
		query += ` AND ng.owner_user_id = ?`
		args = append(args, *ownerUserID)
	}
	query += ` GROUP BY ngn.group_id HAVING SUM(CASE WHEN ngn.node_id IN (` + placeholders + `) THEN 0 ELSE 1 END) = 0`
	args = append(args, int64Args(nodeIDs)...)
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

func nodeOwners(tx *sql.Tx, query string, args []any, expected int) ([]int64, error) {
	rows, err := tx.Query(query, args...)
	if err != nil {
		return nil, err
	}
	owners := map[int64]bool{}
	count := 0
	for rows.Next() {
		var id, ownerID int64
		if err := rows.Scan(&id, &ownerID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		count++
		owners[ownerID] = true
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if count != expected {
		return nil, ErrNotFound
	}
	out := make([]int64, 0, len(owners))
	for ownerID := range owners {
		out = append(out, ownerID)
	}
	return out, nil
}

func cleanProfileNodeReferences(tx *sql.Tx, ownerIDs, nodeIDs []int64) ([]profileOptionsUpdate, error) {
	return cleanProfileReferences(tx, ownerIDs, nodeIDs, nil)
}

func cleanProfileReferences(tx *sql.Tx, ownerIDs, nodeIDs, groupIDs []int64) ([]profileOptionsUpdate, error) {
	if len(ownerIDs) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ownerIDs)), ",")
	rows, err := tx.Query(`SELECT id, options FROM profiles WHERE owner_user_id IN (`+placeholders+`) ORDER BY id`,
		int64Args(ownerIDs)...)
	if err != nil {
		return nil, err
	}
	deleted := make(map[int64]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		deleted[id] = true
	}
	var updates []profileOptionsUpdate
	for rows.Next() {
		var id int64
		var options string
		if err := rows.Scan(&id, &options); err != nil {
			_ = rows.Close()
			return nil, err
		}
		cleaned, changed, err := removeIDsFromProfileOptions(options, deleted, int64Set(groupIDs))
		if err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("clean profile %d options: %w", id, err)
		}
		if changed {
			updates = append(updates, profileOptionsUpdate{id: id, options: cleaned})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return updates, nil
}

func removeNodeIDsFromProfileOptions(blob string, deleted map[int64]bool) (string, bool, error) {
	return removeIDsFromProfileOptions(blob, deleted, nil)
}

func removeIDsFromProfileOptions(blob string, deletedNodes, deletedGroups map[int64]bool) (string, bool, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal([]byte(blob), &root); err != nil {
		return "", false, err
	}
	if root == nil {
		return blob, false, nil
	}
	changed := false
	if raw, ok := root["chainProxyNodeId"]; ok {
		var id int64
		if err := json.Unmarshal(raw, &id); err != nil {
			return "", false, fmt.Errorf("chainProxyNodeId: %w", err)
		}
		if deletedNodes[id] {
			delete(root, "chainProxyNodeId")
			changed = true
		}
	}
	if updated, ok, err := filterNodeIDArray(root["chainProxyNodeIds"], deletedNodes); err != nil {
		return "", false, fmt.Errorf("chainProxyNodeIds: %w", err)
	} else if ok {
		root["chainProxyNodeIds"] = updated
		changed = true
	}
	for _, key := range []string{"autoCountrySelection", "chainProxySelection"} {
		updated, ok, err := filterSelectionIDs(root[key], deletedNodes, deletedGroups)
		if err != nil {
			return "", false, fmt.Errorf("%s: %w", key, err)
		}
		if ok {
			root[key] = updated
			changed = true
		}
	}
	if raw, ok := root["groupSelections"]; ok {
		updated, groupChanged, err := filterGroupSelectionIDs(raw, deletedNodes, deletedGroups)
		if err != nil {
			return "", false, fmt.Errorf("groupSelections: %w", err)
		}
		if groupChanged {
			root["groupSelections"] = updated
			changed = true
		}
	}
	if !changed {
		return blob, false, nil
	}
	out, err := json.Marshal(root)
	if err != nil {
		return "", false, err
	}
	return string(out), true, nil
}

func int64Set(ids []int64) map[int64]bool {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func filterGroupSelectionNodeIDs(raw json.RawMessage, deleted map[int64]bool) (json.RawMessage, bool, error) {
	return filterGroupSelectionIDs(raw, deleted, nil)
}

func filterGroupSelectionIDs(raw json.RawMessage, deletedNodes, deletedGroups map[int64]bool) (json.RawMessage, bool, error) {
	var selections map[string]json.RawMessage
	if err := json.Unmarshal(raw, &selections); err != nil {
		return nil, false, err
	}
	changed := false
	for tag, selection := range selections {
		updated, ok, err := filterSelectionIDs(selection, deletedNodes, deletedGroups)
		if err != nil {
			return nil, false, fmt.Errorf("%s: %w", tag, err)
		}
		if ok {
			selections[tag] = updated
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	out, err := json.Marshal(selections)
	return out, true, err
}

func filterSelectionNodeIDs(raw json.RawMessage, deleted map[int64]bool) (json.RawMessage, bool, error) {
	return filterSelectionIDs(raw, deleted, nil)
}

func filterSelectionIDs(raw json.RawMessage, deletedNodes, deletedGroups map[int64]bool) (json.RawMessage, bool, error) {
	if raw == nil {
		return raw, false, nil
	}
	var selection map[string]json.RawMessage
	if err := json.Unmarshal(raw, &selection); err != nil {
		return nil, false, err
	}
	if selection == nil {
		return raw, false, nil
	}
	updated, nodeChanged, err := filterNodeIDArray(selection["nodeIds"], deletedNodes)
	if err != nil {
		return nil, false, err
	}
	changed := false
	if nodeChanged {
		selection["nodeIds"] = updated
		changed = true
	}
	if updatedGroups, ok, err := filterNodeGroupIDArray(selection["nodeGroupIds"], deletedGroups); err != nil {
		return nil, false, err
	} else if ok {
		selection["nodeGroupIds"] = updatedGroups
		changed = true
	}
	out, err := json.Marshal(selection)
	return out, changed, err
}

func filterNodeIDArray(raw json.RawMessage, deleted map[int64]bool) (json.RawMessage, bool, error) {
	if raw == nil {
		return raw, false, nil
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, false, err
	}
	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !deleted[id] {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == len(ids) {
		return raw, false, nil
	}
	out, err := json.Marshal(filtered)
	return out, true, err
}

func filterNodeGroupIDArray(raw json.RawMessage, deleted map[int64]bool) (json.RawMessage, bool, error) {
	if raw == nil || len(deleted) == 0 {
		return raw, false, nil
	}
	var ids []int64
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, false, err
	}
	filtered := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !deleted[id] {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == len(ids) {
		return raw, false, nil
	}
	out, err := json.Marshal(filtered)
	return out, true, err
}

func int64Args(ids []int64) []any {
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	return args
}
