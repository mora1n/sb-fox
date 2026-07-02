package api

import (
	"encoding/json"
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

// handleListProfiles returns all profiles.
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListProfiles()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleGetProfile returns one profile with its node ids.
func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.GetProfile(pathID(r))
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, p)
}

type profileRequest struct {
	Name       string                `json:"name"`
	TemplateID int64                 `json:"template_id"`
	NodeIDs    []int64               `json:"node_ids"`
	Options    models.ProfileOptions `json:"options"`
}

// handleCreateProfile creates a profile and generates its public token
// (requirement f).
func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.TemplateID == 0 {
		respondError(w, http.StatusBadRequest, "bad_request", "name and template_id are required")
		return
	}
	if _, err := s.Store.GetTemplate(req.TemplateID); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "template not found")
		return
	}
	token, err := newToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	p := &models.Profile{
		Name:       req.Name,
		TemplateID: req.TemplateID,
		Options:    marshalOptions(req.Options),
		Token:      token,
		NodeIDs:    req.NodeIDs,
	}
	id, err := s.Store.CreateProfile(p)
	if err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	p.ID = id
	respondJSON(w, http.StatusCreated, p)
}

// handleUpdateProfile updates a profile and its node membership.
func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	existing, err := s.Store.GetProfile(pathID(r))
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	existing.Name = req.Name
	existing.TemplateID = req.TemplateID
	existing.Options = marshalOptions(req.Options)
	existing.NodeIDs = req.NodeIDs
	if err := s.Store.UpdateProfile(existing); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

// handleRotateToken issues a new public token, revoking the old link.
func (s *Server) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	if _, err := s.Store.GetProfile(id); err != nil {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	}
	token, err := newToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.SetProfileToken(id, token); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"token": token})
}

// handleDeleteProfile removes a profile.
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteProfile(pathID(r)); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func marshalOptions(o models.ProfileOptions) string {
	b, err := json.Marshal(o)
	if err != nil {
		return "{}"
	}
	return string(b)
}
