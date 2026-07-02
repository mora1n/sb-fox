package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/mora1n/sb-fox/internal/auth"
	"github.com/mora1n/sb-fox/internal/models"
)

type userContextKey struct{}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return s.Auth.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subject, ok := auth.Subject(r)
		if !ok {
			respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
			return
		}
		id, err := strconv.ParseInt(subject, 10, 64)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized", "invalid session")
			return
		}
		u, err := s.Store.GetUser(id)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized", "user not found")
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

func currentUser(r *http.Request) (*models.User, bool) {
	u, ok := r.Context().Value(userContextKey{}).(*models.User)
	return u, ok
}

func requireCurrentUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	u, ok := currentUser(r)
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return nil, false
	}
	return u, true
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return nil, false
	}
	if !u.IsAdmin() {
		respondError(w, http.StatusForbidden, "forbidden", "admin only")
		return nil, false
	}
	return u, true
}

func ownerScope(r *http.Request) (int64, bool) {
	u, ok := currentUser(r)
	if !ok {
		return 0, false
	}
	if u.IsAdmin() {
		return 0, true
	}
	return u.ID, false
}
