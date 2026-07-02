package store

import (
	"database/sql"
	"errors"

	"github.com/mora1n/sb-fox/internal/models"
)

func scanTemplate(sc interface{ Scan(...any) error }) (*models.Template, error) {
	var t models.Template
	var created, updated string
	if err := sc.Scan(&t.ID, &t.OwnerUserID, &t.Name, &t.Kind, &t.Content, &t.Description, &created, &updated); err != nil {
		return nil, err
	}
	t.CreatedAt = parseTime(created)
	t.UpdatedAt = parseTime(updated)
	return &t, nil
}

const templateCols = `id, owner_user_id, name, kind, content, description, created_at, updated_at`

// ListTemplates returns all templates ordered by kind then name.
func (s *Store) ListTemplates(ownerUserID int64, allOwners bool) ([]*models.Template, error) {
	q := `SELECT ` + templateCols + ` FROM templates`
	var args []any
	if !allOwners {
		q += ` WHERE owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	q += ` ORDER BY kind, name`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Template
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetTemplate returns one template by id.
func (s *Store) GetTemplate(id int64) (*models.Template, error) {
	row := s.db.QueryRow(`SELECT `+templateCols+` FROM templates WHERE id = ?`, id)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (s *Store) GetTemplateForUser(id, ownerUserID int64, allOwners bool) (*models.Template, error) {
	q := `SELECT ` + templateCols + ` FROM templates WHERE id = ?`
	args := []any{id}
	if !allOwners {
		q += ` AND owner_user_id = ?`
		args = append(args, ownerUserID)
	}
	row := s.db.QueryRow(q, args...)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// GetTemplateByName returns one template by unique name.
func (s *Store) GetTemplateByName(name string) (*models.Template, error) {
	row := s.db.QueryRow(`SELECT `+templateCols+` FROM templates WHERE name = ?`, name)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (s *Store) GetTemplateByNameForUser(ownerUserID int64, name string) (*models.Template, error) {
	row := s.db.QueryRow(`SELECT `+templateCols+` FROM templates WHERE owner_user_id = ? AND name = ?`,
		ownerUserID, name)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// CreateTemplate inserts a template and returns its id.
func (s *Store) CreateTemplate(t *models.Template) (int64, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO templates (owner_user_id, name, kind, content, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, t.OwnerUserID, t.Name, t.Kind, t.Content, t.Description, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SeedUserTemplate inserts a file-backed template when the name does not
// already exist. It never overwrites user edits.
func (s *Store) SeedUserTemplate(ownerUserID int64, name, content, description string) (bool, error) {
	ts := now()
	res, err := s.db.Exec(`
		INSERT INTO templates (owner_user_id, name, kind, content, description, created_at, updated_at)
		VALUES (?, ?, 'user', ?, ?, ?, ?)
		ON CONFLICT(owner_user_id, name) DO NOTHING`,
		ownerUserID, name, content, description, ts, ts)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// UpdateTemplate updates a user template's content/description.
func (s *Store) UpdateTemplate(id int64, content, description string) error {
	_, err := s.db.Exec(`UPDATE templates SET content = ?, description = ?, updated_at = ?
		WHERE id = ? AND kind = 'user'`, content, description, now(), id)
	return err
}

// DeleteTemplate removes a user template. Builtins are protected by the caller.
func (s *Store) DeleteTemplate(id int64) error {
	_, err := s.db.Exec(`DELETE FROM templates WHERE id = ? AND kind = 'user'`, id)
	return err
}

func (s *Store) CountTemplates(ownerUserID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM templates WHERE owner_user_id = ?`, ownerUserID).Scan(&n)
	return n, err
}
