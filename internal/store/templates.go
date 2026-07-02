package store

import (
	"database/sql"
	"errors"

	"github.com/mora1n/sb-fox/internal/models"
)

func scanTemplate(sc interface{ Scan(...any) error }) (*models.Template, error) {
	var t models.Template
	var created, updated string
	if err := sc.Scan(&t.ID, &t.Name, &t.Kind, &t.Content, &t.Description, &created, &updated); err != nil {
		return nil, err
	}
	t.CreatedAt = parseTime(created)
	t.UpdatedAt = parseTime(updated)
	return &t, nil
}

const templateCols = `id, name, kind, content, description, created_at, updated_at`

// ListTemplates returns all templates ordered by kind then name.
func (s *Store) ListTemplates() ([]*models.Template, error) {
	rows, err := s.db.Query(`SELECT ` + templateCols + ` FROM templates ORDER BY kind, name`)
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

// GetTemplateByName returns one template by unique name.
func (s *Store) GetTemplateByName(name string) (*models.Template, error) {
	row := s.db.QueryRow(`SELECT `+templateCols+` FROM templates WHERE name = ?`, name)
	t, err := scanTemplate(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// CreateTemplate inserts a template and returns its id.
func (s *Store) CreateTemplate(t *models.Template) (int64, error) {
	ts := now()
	res, err := s.db.Exec(`INSERT INTO templates (name, kind, content, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, t.Name, t.Kind, t.Content, t.Description, ts, ts)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SeedUserTemplate inserts a file-backed template when the name does not
// already exist. It never overwrites user edits.
func (s *Store) SeedUserTemplate(name, content, description string) (bool, error) {
	ts := now()
	res, err := s.db.Exec(`
		INSERT INTO templates (name, kind, content, description, created_at, updated_at)
		VALUES (?, 'user', ?, ?, ?, ?)
		ON CONFLICT(name) DO NOTHING`,
		name, content, description, ts, ts)
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
