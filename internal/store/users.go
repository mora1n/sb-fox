package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
)

const userCols = `id, username, password_hash, role, node_limit, profile_limit, template_limit, active_kernel_id, country_heat_order, subscription_token, created_at, updated_at`

func scanUser(sc interface{ Scan(...any) error }) (*models.User, error) {
	var u models.User
	var created, updated string
	if err := sc.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.NodeLimit,
		&u.ProfileLimit, &u.TemplateLimit, &u.ActiveKernelID, &u.CountryHeatOrder, &u.SubscriptionToken, &created, &updated); err != nil {
		return nil, err
	}
	u.CreatedAt = parseTime(created)
	u.UpdatedAt = parseTime(updated)
	return &u, nil
}

func NormalizeRole(role string) (string, error) {
	switch strings.TrimSpace(role) {
	case "", models.RoleUser:
		return models.RoleUser, nil
	case models.RoleAdmin:
		return models.RoleAdmin, nil
	default:
		return "", fmt.Errorf("invalid role %q", role)
	}
}

func NormalizeLimit(v int) (int, error) {
	if v < 0 {
		return 0, errors.New("limit cannot be negative")
	}
	return v, nil
}

func (s *Store) ListUsers() ([]*models.User, error) {
	rows, err := s.db.Query(`SELECT ` + userCols + ` FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) GetUser(id int64) (*models.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE username = ?`, strings.TrimSpace(username))
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) GetUserBySubscriptionToken(token string) (*models.User, error) {
	row := s.db.QueryRow(`SELECT `+userCols+` FROM users WHERE subscription_token = ?`, strings.TrimSpace(token))
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) CreateUser(u *models.User) (int64, error) {
	role, err := NormalizeRole(u.Role)
	if err != nil {
		return 0, err
	}
	nodeLimit, err := NormalizeLimit(u.NodeLimit)
	if err != nil {
		return 0, err
	}
	profileLimit, err := NormalizeLimit(u.ProfileLimit)
	if err != nil {
		return 0, err
	}
	templateLimit, err := NormalizeLimit(u.TemplateLimit)
	if err != nil {
		return 0, err
	}
	token := strings.TrimSpace(u.SubscriptionToken)
	if token == "" {
		token, err = randomToken()
		if err != nil {
			return 0, err
		}
	}
	ts := now()
	res, err := s.db.Exec(`INSERT INTO users
		(username, password_hash, role, node_limit, profile_limit, template_limit, active_kernel_id, country_heat_order, subscription_token, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(u.Username), u.PasswordHash, role, nodeLimit, profileLimit, templateLimit, strings.TrimSpace(u.ActiveKernelID), strings.TrimSpace(u.CountryHeatOrder), token, ts, ts)
	if err != nil {
		return 0, err
	}
	u.SubscriptionToken = token
	return res.LastInsertId()
}

func (s *Store) UpdateUser(u *models.User) error {
	role, err := NormalizeRole(u.Role)
	if err != nil {
		return err
	}
	nodeLimit, err := NormalizeLimit(u.NodeLimit)
	if err != nil {
		return err
	}
	profileLimit, err := NormalizeLimit(u.ProfileLimit)
	if err != nil {
		return err
	}
	templateLimit, err := NormalizeLimit(u.TemplateLimit)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET username = ?, role = ?, node_limit = ?,
		profile_limit = ?, template_limit = ?, updated_at = ? WHERE id = ?`,
		strings.TrimSpace(u.Username), role, nodeLimit, profileLimit, templateLimit, now(), u.ID)
	return err
}

func (s *Store) SetUserPassword(id int64, passwordHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, now(), id)
	return err
}

func (s *Store) SetUserSubscriptionToken(id int64, token string) error {
	_, err := s.db.Exec(`UPDATE users SET subscription_token = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(token), now(), id)
	return err
}

func (s *Store) SetUserActiveKernel(id int64, kernelID string) error {
	_, err := s.db.Exec(`UPDATE users SET active_kernel_id = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(kernelID), now(), id)
	return err
}

func (s *Store) SetUserCountryHeatOrder(id int64, order string) error {
	_, err := s.db.Exec(`UPDATE users SET country_heat_order = ?, updated_at = ? WHERE id = ?`, strings.TrimSpace(order), now(), id)
	return err
}

func (s *Store) DeleteUser(id int64) error {
	u, err := s.GetUser(id)
	if err != nil {
		return err
	}
	if u.Role == models.RoleAdmin {
		n, err := s.CountAdmins()
		if err != nil {
			return err
		}
		if n <= 1 {
			return errors.New("cannot delete the last admin")
		}
	}
	_, err = s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&n)
	return n, err
}

func (s *Store) FirstAdmin() (*models.User, error) {
	row := s.db.QueryRow(`SELECT ` + userCols + ` FROM users WHERE role = 'admin' ORDER BY id LIMIT 1`)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
