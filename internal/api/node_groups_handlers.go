package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

type nodeGroupRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	NodeIDs     []int64 `json:"node_ids"`
}

func (s *Server) handleListNodeGroups(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	groups, err := s.Store.ListNodeGroups(ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, groups)
}

func (s *Server) handleGetNodeGroup(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	g, err := s.Store.GetNodeGroupForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, g)
}

func (s *Server) handleCreateNodeGroup(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req nodeGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	req.NodeIDs = uniqueInt64s(req.NodeIDs)
	if !s.validateNodeAccess(w, u, req.NodeIDs) {
		return
	}
	g := &models.NodeGroup{OwnerUserID: u.ID, Name: req.Name, Description: req.Description, NodeIDs: req.NodeIDs}
	id, err := s.Store.CreateNodeGroup(g)
	if err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	g.ID = id
	respondJSON(w, http.StatusCreated, g)
}

func (s *Server) handleUpdateNodeGroup(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireCurrentUser(w, r); !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	existing, err := s.Store.GetNodeGroupForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	var req nodeGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "name is required")
		return
	}
	req.NodeIDs = uniqueInt64s(req.NodeIDs)
	refAllOwners := false
	if !s.validateNodeAccessForOwner(w, existing.OwnerUserID, refAllOwners, req.NodeIDs) {
		return
	}
	existing.Name = req.Name
	existing.Description = req.Description
	existing.NodeIDs = req.NodeIDs
	if err := s.Store.UpdateNodeGroup(existing); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleDeleteNodeGroup(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	g, err := s.Store.GetNodeGroupForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.DeleteNodeGroupForUser(g.ID, g.OwnerUserID); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
