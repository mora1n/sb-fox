package store

import (
	"errors"

	"github.com/mora1n/sb-fox/internal/models"
)

// ErrNotFound is returned when a lookup finds no row.
var ErrNotFound = errors.New("store: not found")

// GetAdmin returns the first admin user, or ErrNotFound if unset.
func (s *Store) GetAdmin() (*models.Admin, error) {
	u, err := s.FirstAdmin()
	if err != nil {
		return nil, err
	}
	return &models.Admin{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

// SetAdmin creates or updates the first admin user with the given username and
// bcrypt hash.
func (s *Store) SetAdmin(username, passwordHash string) error {
	admin, err := s.FirstAdmin()
	if errors.Is(err, ErrNotFound) {
		_, err = s.CreateUser(&models.User{Username: username, PasswordHash: passwordHash, Role: models.RoleAdmin})
		return err
	}
	if err != nil {
		return err
	}
	admin.Username = username
	admin.PasswordHash = passwordHash
	admin.Role = models.RoleAdmin
	if err := s.UpdateUser(admin); err != nil {
		return err
	}
	return s.SetUserPassword(admin.ID, passwordHash)
}

// AdminExists reports whether the admin row is set.
func (s *Store) AdminExists() (bool, error) {
	n, err := s.CountAdmins()
	return n > 0, err
}
