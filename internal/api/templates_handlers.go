package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

// handleListTemplates returns all templates.
func (s *Server) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	list, err := s.Store.ListTemplates(ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, list)
}

// handleGetTemplate returns one template's full content.
func (s *Server) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(id, ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, t)
}

type templateRequest struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	Description string `json:"description"`
}

// handleCreateTemplate saves a user template (imported or from panel edits).
func (s *Server) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req templateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" || req.Content == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "name and content are required")
		return
	}
	if !validJSON(req.Content) {
		respondError(w, http.StatusBadRequest, "bad_request", "content is not valid JSON")
		return
	}
	if !s.checkQuota(w, u, quotaTemplates, 1) {
		return
	}
	t := &models.Template{OwnerUserID: u.ID, Name: req.Name, Kind: "user", Content: req.Content, Description: req.Description}
	id, err := s.Store.CreateTemplate(t)
	if err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	t.ID = id
	respondJSON(w, http.StatusCreated, t)
}

// handleUpdateTemplate updates a user template (builtins are read-only).
func (s *Server) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(id, ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if t.Kind == "builtin" {
		respondError(w, http.StatusForbidden, "forbidden", "built-in templates are read-only")
		return
	}
	var req templateRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validJSON(req.Content) {
		respondError(w, http.StatusBadRequest, "bad_request", "content is not valid JSON")
		return
	}
	if err := s.Store.UpdateTemplate(id, req.Content, req.Description); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleDeleteTemplate removes a user template.
func (s *Server) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(id, ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if t.Kind == "builtin" {
		respondError(w, http.StatusForbidden, "forbidden", "built-in templates cannot be deleted")
		return
	}
	if err := s.Store.DeleteTemplate(id); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// groupInfo describes a selector/urltest group detected in a template.
type groupInfo struct {
	Tag       string   `json:"tag"`
	Type      string   `json:"type"`
	Outbounds []string `json:"outbounds"`
}

// handleInspectTemplate parses a template's outbounds and reports its
// selector/urltest groups (requirement c).
func (s *Server) handleInspectTemplate(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(id, ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	groups, err := inspectGroups(t.Content)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_template", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

// inspectGroups extracts selector/urltest groups from a template's outbounds.
func inspectGroups(content string) ([]groupInfo, error) {
	cfg, err := merge.ParseOrdered([]byte(content))
	if err != nil {
		return nil, err
	}
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return []groupInfo{}, nil
	}
	arr, _ := raw.([]any)
	var groups []groupInfo
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok {
			continue
		}
		typ := om.GetString("type")
		if typ != "selector" && typ != "urltest" {
			continue
		}
		g := groupInfo{Tag: om.GetString("tag"), Type: typ, Outbounds: []string{}}
		if outs, ok := om.Get("outbounds"); ok {
			if list, ok := outs.([]any); ok {
				for _, v := range list {
					if s, ok := v.(string); ok {
						g.Outbounds = append(g.Outbounds, s)
					}
				}
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func pathID(r *http.Request) int64 {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	return id
}

func pathInt64(raw string) int64 {
	id, _ := strconv.ParseInt(raw, 10, 64)
	return id
}

func validJSON(s string) bool {
	var v any
	return json.Unmarshal([]byte(s), &v) == nil
}
