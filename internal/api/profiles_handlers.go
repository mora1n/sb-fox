package api

import (
	"encoding/json"
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

// handleListProfiles returns all profiles.
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	list, err := s.Store.ListProfiles(ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleGetProfile returns one profile with its node ids.
func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	p, err := s.Store.GetProfileForUser(pathID(r), ownerID, allOwners)
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
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req profileRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.TemplateID == 0 {
		respondError(w, http.StatusBadRequest, "bad_request", "name and template_id are required")
		return
	}
	if _, err := s.Store.GetTemplateForUser(req.TemplateID, u.ID, u.IsAdmin()); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "template not found")
		return
	}
	if !s.validateNodeAccess(w, u, req.NodeIDs) {
		return
	}
	if !s.checkQuota(w, u, quotaProfiles, 1) {
		return
	}
	token, err := newToken()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	p := &models.Profile{
		OwnerUserID: u.ID,
		Name:        req.Name,
		TemplateID:  req.TemplateID,
		Options:     marshalOptions(req.Options),
		Token:       token,
		NodeIDs:     req.NodeIDs,
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
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	existing, err := s.Store.GetProfileForUser(pathID(r), ownerID, allOwners)
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
	refOwner := existing.OwnerUserID
	refAllOwners := u.IsAdmin()
	if _, err := s.Store.GetTemplateForUser(req.TemplateID, refOwner, refAllOwners); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "template not found")
		return
	}
	if !s.validateNodeAccessForOwner(w, refOwner, refAllOwners, req.NodeIDs) {
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
	ownerID, allOwners := ownerScope(r)
	if _, err := s.Store.GetProfileForUser(id, ownerID, allOwners); err != nil {
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
	ownerID, allOwners := ownerScope(r)
	if _, err := s.Store.GetProfileForUser(pathID(r), ownerID, allOwners); err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
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

func (s *Server) validateNodeAccess(w http.ResponseWriter, u *models.User, nodeIDs []int64) bool {
	return s.validateNodeAccessForOwner(w, u.ID, u.IsAdmin(), nodeIDs)
}

func (s *Server) validateNodeAccessForOwner(w http.ResponseWriter, ownerUserID int64, allOwners bool, nodeIDs []int64) bool {
	for _, id := range nodeIDs {
		if _, err := s.Store.GetNodeForUser(id, ownerUserID, allOwners); err == store.ErrNotFound {
			respondError(w, http.StatusBadRequest, "bad_request", "node not found")
			return false
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return false
		}
	}
	return true
}
