package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

// handleListNodes returns nodes matching optional query filters.
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	q := r.URL.Query()
	filter := store.NodeFilter{
		Source:      q.Get("source"),
		CountryCode: q.Get("country"),
		Type:        q.Get("type"),
		Search:      q.Get("search"),
		OwnerUserID: ownerID,
		AllOwners:   allOwners,
	}
	if allOwners && q.Get("owner_user_id") != "" {
		filter.OwnerUserID = pathInt64(q.Get("owner_user_id"))
	}
	nodes, err := s.Store.ListNodes(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, nodes)
}

// handleGetNode returns one node.
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	n, err := s.Store.GetNodeForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, n)
}

type nodeRequest struct {
	Raw           string `json:"raw"`            // full outbound JSON
	CountryCode   string `json:"country_code"`   // optional manual override
	CountrySource string `json:"country_source"` // auto|manual|override
}

// handleCreateNode creates a node from a raw outbound object (edit form).
func (s *Server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req nodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	raw, err := merge.ParseOrdered([]byte(req.Raw))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "raw is not a valid outbound object: "+err.Error())
		return
	}
	n, err := nodeFromOutbound(raw, "manual", nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !s.checkQuota(w, u, quotaNodes, 1) {
		return
	}
	n.OwnerUserID = u.ID
	applyManualCountry(n, req)
	id, err := s.Store.CreateNode(n)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	n.ID = id
	respondJSON(w, http.StatusCreated, n)
}

// handleUpdateNode updates a node's raw blob and re-extracts metadata, honoring
// a manual country override if provided.
func (s *Server) handleUpdateNode(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	existing, err := s.Store.GetNodeForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var req nodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	raw, err := merge.ParseOrdered([]byte(req.Raw))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "raw is not a valid outbound object: "+err.Error())
		return
	}
	updated, err := nodeFromOutbound(raw, existing.Source, existing.SourceRef)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	updated.ID = existing.ID
	updated.OwnerUserID = existing.OwnerUserID
	applyManualCountry(updated, req)
	if err := s.Store.UpdateNode(updated); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

// handleDeleteNode removes a node.
func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	if _, err := s.Store.GetNodeForUser(pathID(r), ownerID, allOwners); err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.DeleteNode(pathID(r)); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// applyManualCountry lets the request override the detected country.
func applyManualCountry(n *models.Node, req nodeRequest) {
	if req.CountrySource == "manual" && req.CountryCode != "" {
		n.CountryCode = req.CountryCode
		n.CountrySource = "manual"
	}
}
