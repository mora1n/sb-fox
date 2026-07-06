package store

import (
	"strings"
)

func (s *Store) DeleteNodesByIDs(ids []int64) (int, error) {
	return s.deleteByIDs("nodes", ids, "")
}

func (s *Store) DeleteNodeGroupsByIDs(ids []int64) (int, error) {
	return s.deleteByIDs("node_groups", ids, "")
}

func (s *Store) DeleteTemplatesByIDs(ids []int64) (int, error) {
	return s.deleteByIDs("templates", ids, " AND kind = 'user'")
}

func (s *Store) DeleteProfilesByIDs(ids []int64) (int, error) {
	return s.deleteByIDs("profiles", ids, "")
}

func (s *Store) deleteByIDs(table string, ids []int64, extraWhere string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	res, err := tx.Exec(`DELETE FROM `+table+` WHERE id IN (`+placeholders+`)`+extraWhere, args...)
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
