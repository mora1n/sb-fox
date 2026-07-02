package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

type userRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	Role          string `json:"role"`
	NodeLimit     int    `json:"node_limit"`
	ProfileLimit  int    `json:"profile_limit"`
	TemplateLimit int    `json:"template_limit"`
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	users, err := s.Store.ListUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	var req userRequest
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
	role, err := store.NormalizeRole(req.Role)
	if err != nil || role != models.RoleUser {
		respondError(w, http.StatusBadRequest, "bad_request", "users created from the panel must have role user")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	u := &models.User{
		Username:      username,
		PasswordHash:  hash,
		Role:          role,
		NodeLimit:     req.NodeLimit,
		ProfileLimit:  req.ProfileLimit,
		TemplateLimit: req.TemplateLimit,
	}
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
	respondJSON(w, http.StatusCreated, u)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	existing, err := s.Store.GetUser(pathID(r))
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var req userRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "username is required")
		return
	}
	if req.Role != "" && req.Role != existing.Role {
		respondError(w, http.StatusBadRequest, "bad_request", "user role cannot be changed")
		return
	}
	existing.Username = username
	existing.NodeLimit = req.NodeLimit
	existing.ProfileLimit = req.ProfileLimit
	existing.TemplateLimit = req.TemplateLimit
	if err := s.Store.UpdateUser(existing); err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	updated, err := s.Store.GetUser(existing.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	current, ok := requireAdmin(w, r)
	if !ok {
		return
	}
	id := pathID(r)
	if current.ID == id {
		respondError(w, http.StatusForbidden, "forbidden", "cannot delete the current user")
		return
	}
	if err := s.Store.DeleteUser(id); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		if strings.Contains(err.Error(), "last admin") {
			respondError(w, http.StatusForbidden, "forbidden", err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	u, err := s.Store.GetUser(pathID(r))
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	password := randomPassword()
	hash, err := auth.HashPassword(password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.SetUserPassword(u.ID, hash); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"password": password})
}

func randomPassword() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic("sb-fox: cannot read random bytes")
	}
	return hex.EncodeToString(b)
}
