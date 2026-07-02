package api

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/kernel"
	"github.com/mora1n/sb-fox/internal/store"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

// Server holds the API dependencies shared across handlers.
type Server struct {
	Store   *store.Store
	Auth    *auth.Authenticator
	Kernel  *kernel.Kernel
	Fetcher *subfetch.Fetcher
	// TemplateDir holds file-backed seed templates copied into each user.
	TemplateDir string
	// Secure marks whether session cookies should set the Secure flag (https).
	Secure bool
	// RegistrationEnabled exposes the public registration endpoint.
	RegistrationEnabled bool
	// DevMode skips serving the embedded frontend (API-only).
	DevMode bool
}

// newToken returns a 128-bit random hex token for public subscription links.
func newToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
