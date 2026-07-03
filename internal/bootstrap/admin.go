// Package bootstrap contains one-time application initialization helpers.
package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/store"
)

// AdminInit reports whether the default admin was created.
type AdminInit struct {
	Created   bool
	Username  string
	Password  string
	Generated bool
	FromEnv   bool
}

// EnsureAdmin creates the default admin account when the database has no
// admin. SB_FOX_ADMIN_PASSWORD sets a known initial password without exposing
// it in the returned result.
func EnsureAdmin(db *store.Store) (*AdminInit, error) {
	exists, err := db.AdminExists()
	if err != nil {
		return nil, err
	}
	if exists {
		return &AdminInit{}, nil
	}
	password := os.Getenv("SB_FOX_ADMIN_PASSWORD")
	generated := false
	if password == "" {
		password = randomHex(12)
		generated = true
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	if err := db.SetAdmin("admin", hash); err != nil {
		return nil, err
	}
	result := &AdminInit{
		Created:   true,
		Username:  "admin",
		Generated: generated,
		FromEnv:   !generated,
	}
	if generated {
		result.Password = password
	}
	return result, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
