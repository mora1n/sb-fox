package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/store"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type passwordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleLogin verifies credentials and sets a session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	admin, err := s.Store.GetAdmin()
	if err == store.ErrNotFound {
		respondError(w, http.StatusUnauthorized, "unauthorized", "admin not initialized")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if req.Username != admin.Username || !auth.VerifyPassword(admin.PasswordHash, req.Password) {
		respondError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}
	s.Auth.SetSessionCookie(w, admin.Username, s.Secure)
	respondJSON(w, http.StatusOK, map[string]string{"username": admin.Username})
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the current authenticated admin.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	subject, ok := auth.Subject(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"username": subject})
}

// handleChangePassword updates the admin password after verifying the old one.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req passwordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 8 {
		respondError(w, http.StatusBadRequest, "bad_request", "new password must be at least 8 characters")
		return
	}
	admin, err := s.Store.GetAdmin()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !auth.VerifyPassword(admin.PasswordHash, req.OldPassword) {
		respondError(w, http.StatusUnauthorized, "unauthorized", "current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.SetAdmin(admin.Username, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
