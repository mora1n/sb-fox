package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

type bulkIDsRequest struct {
	IDs []int64 `json:"ids"`
}

type bulkDeleteResponse struct {
	Deleted int `json:"deleted"`
}

type bulkNodeUsageResponse struct {
	Usage []bulkNodeUsage `json:"usage"`
}

type bulkNodeUsage struct {
	NodeID       int64  `json:"node_id"`
	ProfileID    int64  `json:"profile_id"`
	ProfileName  string `json:"profile_name"`
	ViaGroupID   int64  `json:"via_group_id,omitempty"`
	ViaGroupName string `json:"via_group_name,omitempty"`
}

func decodeBulkIDs(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	var req bulkIDsRequest
	if !decodeJSON(w, r, &req) {
		return nil, false
	}
	ids := normalizeBulkIDs(req.IDs)
	if len(ids) == 0 {
		respondError(w, http.StatusBadRequest, "no_ids", "no items selected")
		return nil, false
	}
	return ids, true
}

func normalizeBulkIDs(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func (s *Server) handlePreviewBulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	usage := []bulkNodeUsage{}
	for _, id := range ids {
		if _, err := s.Store.GetNodeForUser(id, ownerID, allOwners); err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "node not found")
			return
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		nodeUsage, err := s.bulkNodeUsage(id, ownerID, allOwners)
		if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		usage = append(usage, nodeUsage...)
	}
	respondJSON(w, http.StatusOK, bulkNodeUsageResponse{Usage: usage})
}

func (s *Server) bulkNodeUsage(nodeID, ownerID int64, allOwners bool) ([]bulkNodeUsage, error) {
	usage, err := s.Store.ListNodeUsage(nodeID, ownerID, allOwners)
	if err != nil {
		return nil, err
	}
	optionUsage, err := s.profileOptionNodeUsage(nodeID, ownerID, allOwners)
	if err != nil {
		return nil, err
	}
	usage = mergeNodeUsage(usage, optionUsage)
	return withNodeID(nodeID, usage), nil
}

func withNodeID(nodeID int64, usage []*models.NodeUsage) []bulkNodeUsage {
	out := make([]bulkNodeUsage, 0, len(usage))
	for _, item := range usage {
		out = append(out, bulkNodeUsage{
			NodeID:       nodeID,
			ProfileID:    item.ProfileID,
			ProfileName:  item.ProfileName,
			ViaGroupID:   item.ViaGroupID,
			ViaGroupName: item.ViaGroupName,
		})
	}
	return out
}

func (s *Server) handleBulkDeleteNodes(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	for _, id := range ids {
		if _, err := s.Store.GetNodeForUser(id, ownerID, allOwners); err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "node not found")
			return
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	deleted, err := s.Store.DeleteNodesByIDs(ids)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bulkDeleteResponse{Deleted: deleted})
}

func (s *Server) handleBulkDeleteNodeGroups(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	for _, id := range ids {
		if _, err := s.Store.GetNodeGroupForUser(id, ownerID, allOwners); err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "node group not found")
			return
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	deleted, err := s.Store.DeleteNodeGroupsByIDs(ids)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node group not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bulkDeleteResponse{Deleted: deleted})
}

func (s *Server) handleBulkDeleteTemplates(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	for _, id := range ids {
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
	}
	deleted, err := s.Store.DeleteTemplatesByIDs(ids)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bulkDeleteResponse{Deleted: deleted})
}

func (s *Server) handleBulkDeleteProfiles(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	for _, id := range ids {
		if _, err := s.Store.GetProfileForUser(id, ownerID, allOwners); err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	deleted, err := s.Store.DeleteProfilesByIDs(ids)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bulkDeleteResponse{Deleted: deleted})
}
