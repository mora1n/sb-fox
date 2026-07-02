package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
)

var errQuotaExceeded = errors.New("quota exceeded")

type quotaKind string

const (
	quotaNodes     quotaKind = "nodes"
	quotaProfiles  quotaKind = "profiles"
	quotaTemplates quotaKind = "templates"
)

func (s *Server) checkQuota(w http.ResponseWriter, user *models.User, kind quotaKind, delta int) bool {
	ok, message, err := s.quotaAllowed(user, kind, delta)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return false
	}
	if !ok {
		respondError(w, http.StatusForbidden, "quota_exceeded", message)
		return false
	}
	return true
}

func (s *Server) quotaAllowed(user *models.User, kind quotaKind, delta int) (bool, string, error) {
	if user.IsAdmin() || delta <= 0 {
		return true, "", nil
	}
	limit := 0
	count := 0
	var err error
	switch kind {
	case quotaNodes:
		limit = user.NodeLimit
		count, err = s.Store.CountNodes(user.ID)
	case quotaProfiles:
		limit = user.ProfileLimit
		count, err = s.Store.CountProfiles(user.ID)
	case quotaTemplates:
		limit = user.TemplateLimit
		count, err = s.Store.CountTemplates(user.ID)
	default:
		return false, "", errors.New("unknown quota kind")
	}
	if err != nil {
		return false, "", err
	}
	if limit > 0 && count+delta > limit {
		return false, fmt.Sprintf("%s limit exceeded (%d/%d)", kind, count, limit), nil
	}
	return true, "", nil
}
