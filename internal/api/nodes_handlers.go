package api

import (
	"fmt"
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

func (s *Server) handleNodeUsage(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	id := pathID(r)
	if _, err := s.Store.GetNodeForUser(id, ownerID, allOwners); err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "node not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	usage, err := s.Store.ListNodeUsage(id, ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	optionUsage, err := s.profileOptionNodeUsage(id, ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	usage = mergeNodeUsage(usage, optionUsage)
	respondJSON(w, http.StatusOK, usage)
}

func (s *Server) profileOptionNodeUsage(nodeID, ownerID int64, allOwners bool) ([]*models.NodeUsage, error) {
	profiles, err := s.Store.ListProfiles(ownerID, allOwners)
	if err != nil {
		return nil, err
	}
	var out []*models.NodeUsage
	for _, p := range profiles {
		opts := parseProfileOptions(p.Options)
		selections := make([]models.NodeSelection, 0, len(opts.GroupSelections)+1)
		for _, sel := range opts.GroupSelections {
			selections = append(selections, sel)
		}
		if opts.ChainProxySelected != nil {
			selections = append(selections, *opts.ChainProxySelected)
		}
		for _, sel := range selections {
			if containsInt64(sel.NodeIDs, nodeID) {
				out = append(out, &models.NodeUsage{ProfileID: p.ID, ProfileName: p.Name})
			}
			for _, gid := range sel.NodeGroupIDs {
				group, err := s.Store.GetNodeGroupForUser(gid, p.OwnerUserID, false)
				if err == store.ErrNotFound {
					continue
				}
				if err != nil {
					return nil, err
				}
				if containsInt64(group.NodeIDs, nodeID) {
					out = append(out, &models.NodeUsage{
						ProfileID:    p.ID,
						ProfileName:  p.Name,
						ViaGroupID:   group.ID,
						ViaGroupName: group.Name,
					})
				}
			}
		}
	}
	return out, nil
}

func mergeNodeUsage(a, b []*models.NodeUsage) []*models.NodeUsage {
	out := make([]*models.NodeUsage, 0, len(a)+len(b))
	seen := map[string]bool{}
	add := func(u *models.NodeUsage) {
		key := fmt.Sprintf("%d/%d", u.ProfileID, u.ViaGroupID)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, u)
	}
	for _, u := range a {
		add(u)
	}
	for _, u := range b {
		add(u)
	}
	return out
}

func containsInt64(values []int64, needle int64) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

// applyManualCountry lets the request override the detected country.
func applyManualCountry(n *models.Node, req nodeRequest) {
	if req.CountrySource == "manual" && req.CountryCode != "" {
		n.CountryCode = req.CountryCode
		n.CountrySource = "manual"
	}
}
