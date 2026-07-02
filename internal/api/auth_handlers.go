package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/models"
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

type meResponse struct {
	ID            int64  `json:"id"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	NodeLimit     int    `json:"node_limit"`
	ProfileLimit  int    `json:"profile_limit"`
	TemplateLimit int    `json:"template_limit"`
}

// handleLogin verifies credentials and sets a session cookie.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := s.Store.GetUserByUsername(req.Username)
	if err == store.ErrNotFound {
		respondError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !auth.VerifyPassword(user.PasswordHash, req.Password) {
		respondError(w, http.StatusUnauthorized, "unauthorized", "invalid username or password")
		return
	}
	s.Auth.SetSessionCookie(w, strconv.FormatInt(user.ID, 10), s.Secure)
	respondJSON(w, http.StatusOK, userMe(user))
}

// handleLogout clears the session cookie.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleMe returns the current authenticated admin.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, userMe(u))
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
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	fresh, err := s.Store.GetUser(u.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !auth.VerifyPassword(fresh.PasswordHash, req.OldPassword) {
		respondError(w, http.StatusUnauthorized, "unauthorized", "current password is incorrect")
		return
	}
	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.SetUserPassword(fresh.ID, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.RegistrationEnabled {
		respondError(w, http.StatusForbidden, "registration_disabled", "registration is disabled")
		return
	}
	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "username is required")
		return
	}
	if len(req.Password) < 8 {
		respondError(w, http.StatusBadRequest, "bad_request", "password must be at least 8 characters")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	u := &models.User{Username: username, PasswordHash: hash, Role: models.RoleUser}
	id, err := s.Store.CreateUser(u)
	if err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	u.ID = id
	if err := s.seedTemplatesForUser(u.ID); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.Auth.SetSessionCookie(w, strconv.FormatInt(u.ID, 10), s.Secure)
	respondJSON(w, http.StatusCreated, userMe(u))
}

func userMe(u *models.User) meResponse {
	return meResponse{
		ID:            u.ID,
		Username:      u.Username,
		Role:          u.Role,
		NodeLimit:     u.NodeLimit,
		ProfileLimit:  u.ProfileLimit,
		TemplateLimit: u.TemplateLimit,
	}
}
