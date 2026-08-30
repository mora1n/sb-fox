package store

import (
	"database/sql"
	"errors"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const ruleSetCols = `id, owner_user_id, name, description, published_json, published_srs,
	rule_count, json_size, srs_size, json_sha256, srs_sha256, kernel_version,
	published_at, last_attempt_at, last_error, created_at, updated_at`

const ruleSetSummaryCols = `id, owner_user_id, name, description, NULL, NULL,
	rule_count, json_size, srs_size, json_sha256, srs_sha256, kernel_version,
	published_at, last_attempt_at, last_error, created_at, updated_at`

func scanRuleSet(sc interface{ Scan(...any) error }) (*models.RuleSet, error) {
	var item models.RuleSet
	var published, attempted, created, updated string
	if err := sc.Scan(
		&item.ID, &item.OwnerUserID, &item.Name, &item.Description,
		&item.PublishedJSON, &item.PublishedSRS, &item.RuleCount,
		&item.JSONSize, &item.SRSSize, &item.JSONSHA256, &item.SRSSHA256,
		&item.KernelVersion, &published, &attempted, &item.LastError,
		&created, &updated,
	); err != nil {
		return nil, err
	}
	item.PublishedAt = parseTime(published)
	item.LastAttemptAt = parseTime(attempted)
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	item.Sources = []*models.RuleSetSource{}
	return &item, nil
}

func (s *Store) ListRuleSets(ownerUserID int64, allOwners bool) ([]*models.RuleSet, error) {
	query := `SELECT ` + ruleSetSummaryCols + ` FROM rule_sets`
	var args []any
	if !allOwners {
		query += ` WHERE owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	query += ` ORDER BY name`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []*models.RuleSet{}
	for rows.Next() {
		item, err := scanRuleSet(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	counts, err := s.ruleSetSourceCounts(ruleSetIDs(items))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item.SourceCount = counts[item.ID]
	}
	return items, nil
}

func (s *Store) GetRuleSetForUser(id, ownerUserID int64, allOwners bool) (*models.RuleSet, error) {
	query := `SELECT ` + ruleSetCols + ` FROM rule_sets WHERE id = ?`
	args := []any{id}
	if !allOwners {
		query += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	item, err := scanRuleSet(s.db.QueryRow(query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.Sources, err = s.ruleSetSources(item.ID)
	item.SourceCount = len(item.Sources)
	return item, err
}

func (s *Store) GetRuleSetByNameAndToken(name, token string) (*models.RuleSet, error) {
	item, err := scanRuleSet(s.db.QueryRow(`SELECT `+ruleSetCols+` FROM rule_sets
		WHERE name = ? AND owner_user_id = (
			SELECT id FROM users WHERE subscription_token = ?
		)`, name, token))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return item, err
}

func (s *Store) CreateRuleSet(item *models.RuleSet) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	ts := now()
	res, err := tx.Exec(`INSERT INTO rule_sets (
		owner_user_id, name, description, published_json, published_srs, rule_count,
		json_size, srs_size, json_sha256, srs_sha256, kernel_version,
		published_at, last_attempt_at, last_error, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)`,
		item.OwnerUserID, item.Name, item.Description, item.PublishedJSON,
		item.PublishedSRS, item.RuleCount, item.JSONSize, item.SRSSize,
		item.JSONSHA256, item.SRSSHA256, item.KernelVersion, ts, ts, ts, ts)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := insertRuleSetSources(tx, id, item.Sources); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) UpdateRuleSet(item *models.RuleSet) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	ts := now()
	res, err := tx.Exec(`UPDATE rule_sets SET name=?, description=?, published_json=?,
		published_srs=?, rule_count=?, json_size=?, srs_size=?, json_sha256=?,
		srs_sha256=?, kernel_version=?, published_at=?, last_attempt_at=?, last_error='', updated_at=?
		WHERE id=? AND owner_user_id=?`, item.Name, item.Description, item.PublishedJSON,
		item.PublishedSRS, item.RuleCount, item.JSONSize, item.SRSSize, item.JSONSHA256,
		item.SRSSHA256, item.KernelVersion, ts, ts, ts, item.ID, item.OwnerUserID)
	if err := requireRowsAffected(res, err); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.Exec(`DELETE FROM rule_set_sources WHERE rule_set_id = ?`, item.ID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := insertRuleSetSources(tx, item.ID, item.Sources); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) RefreshRuleSetArtifact(item *models.RuleSet) error {
	ts := now()
	return requireRowsAffected(s.db.Exec(`UPDATE rule_sets SET published_json=?, published_srs=?,
		rule_count=?, json_size=?, srs_size=?, json_sha256=?, srs_sha256=?, kernel_version=?,
		published_at=?, last_attempt_at=?, last_error='', updated_at=?
		WHERE id=? AND owner_user_id=?`, item.PublishedJSON, item.PublishedSRS,
		item.RuleCount, item.JSONSize, item.SRSSize, item.JSONSHA256, item.SRSSHA256,
		item.KernelVersion, ts, ts, ts, item.ID, item.OwnerUserID))
}

func (s *Store) RecordRuleSetFailure(id, ownerUserID int64, message string) error {
	message = strings.TrimSpace(message)
	return requireRowsAffected(s.db.Exec(`UPDATE rule_sets SET last_attempt_at=?, last_error=?
		WHERE id=? AND owner_user_id=?`, now(), message, id, ownerUserID))
}

func (s *Store) DeleteRuleSetForUser(id, ownerUserID int64) error {
	return requireRowsAffected(s.db.Exec(`DELETE FROM rule_sets WHERE id=? AND owner_user_id=?`, id, ownerUserID))
}

func (s *Store) DeleteRuleSetsForUser(ids []int64, ownerUserID int64) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, id := range ids {
		res, err := tx.Exec(`DELETE FROM rule_sets WHERE id=? AND owner_user_id=?`, id, ownerUserID)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		deleted += int(n)
	}
	if deleted != len(ids) {
		_ = tx.Rollback()
		return 0, ErrNotFound
	}
	return deleted, tx.Commit()
}

func insertRuleSetSources(tx *sql.Tx, ruleSetID int64, sources []*models.RuleSetSource) error {
	for position, source := range sources {
		_, err := tx.Exec(`INSERT INTO rule_set_sources
			(rule_set_id, kind, format, url, content, position) VALUES (?, ?, ?, ?, ?, ?)`,
			ruleSetID, source.Kind, source.Format, source.URL, source.Content, position)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ruleSetSources(ruleSetID int64) ([]*models.RuleSetSource, error) {
	rows, err := s.db.Query(`SELECT id, rule_set_id, kind, format, url, content, position
		FROM rule_set_sources WHERE rule_set_id=? ORDER BY position`, ruleSetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sources := []*models.RuleSetSource{}
	for rows.Next() {
		var source models.RuleSetSource
		if err := rows.Scan(&source.ID, &source.RuleSetID, &source.Kind, &source.Format,
			&source.URL, &source.Content, &source.Position); err != nil {
			return nil, err
		}
		sources = append(sources, &source)
	}
	return sources, rows.Err()
}

func (s *Store) ruleSetSourceCounts(ids []int64) (map[int64]int, error) {
	counts := make(map[int64]int, len(ids))
	if len(ids) == 0 {
		return counts, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.Query(`SELECT rule_set_id, COUNT(*) FROM rule_set_sources
		WHERE rule_set_id IN (`+placeholders+`) GROUP BY rule_set_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}

func ruleSetIDs(items []*models.RuleSet) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
