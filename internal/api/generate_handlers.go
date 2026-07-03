package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/store"
)

// previewRequest generates a config from a template + node ids without saving.
type previewRequest struct {
	TemplateID      int64                 `json:"template_id"`
	TemplateContent string                `json:"template_content"` // optional inline override
	NodeIDs         []int64               `json:"node_ids"`
	NodeGroupIDs    []int64               `json:"node_group_ids"`
	Options         models.ProfileOptions `json:"options"`
}

// handlePreview renders a config.json in-memory (requirements a, c, f).
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req previewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	req.NodeIDs = uniqueInt64s(req.NodeIDs)
	req.NodeGroupIDs = uniqueInt64s(req.NodeGroupIDs)
	normalizePreviewRequestOptions(&req)
	if !s.validateOptionSelectionAccess(w, u.ID, u.IsAdmin(), req.Options) {
		return
	}
	if !validateChainProxySelection(w, req.NodeIDs, req.NodeGroupIDs, req.Options) {
		return
	}
	content := req.TemplateContent
	if content == "" {
		t, err := s.Store.GetTemplateForUser(req.TemplateID, u.ID, u.IsAdmin())
		if err != nil {
			respondError(w, http.StatusBadRequest, "bad_request", "template not found")
			return
		}
		content = t.Content
	}
	if err := validateOptionOutboundRefs(content, req.Options); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := validateOptionGroupInputs(content, req.Options); err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if !validateAutoCountrySelection(w, req.Options) {
		return
	}
	order, err := s.countryHeatOrder()
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
		return
	}
	config, err := s.generateFromInputs(content, req.NodeIDs, req.NodeGroupIDs, req.Options, u.ID, u.IsAdmin(), order)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"config": string(config)})
}

func normalizePreviewRequestOptions(req *previewRequest) {
	profileReq := profileRequest{Options: req.Options}
	normalizeProfileRequestOptions(&profileReq)
	req.Options = profileReq.Options
}

type validateRequest struct {
	Config    string `json:"config"`
	ProfileID int64  `json:"profile_id"`
}

// handleValidate runs the sing-box kernel check (advisory).
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	config, ok := s.resolveConfig(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, s.validateWithKernelForUser(u, config))
}

// handleFormat runs the sing-box kernel format.
func (s *Server) handleFormat(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	config, ok := s.resolveConfig(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, s.formatWithKernelForUser(u, config))
}

// resolveConfig returns the config bytes from either an inline config or a
// profile_id (by regenerating it).
func (s *Server) resolveConfig(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	var req validateRequest
	if !decodeJSON(w, r, &req) {
		return nil, false
	}
	if req.Config != "" {
		return []byte(req.Config), true
	}
	if req.ProfileID != 0 {
		ownerID, allOwners := ownerScope(r)
		config, err := s.renderProfileForUser(req.ProfileID, ownerID, allOwners)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
			return nil, false
		}
		return config, true
	}
	respondError(w, http.StatusBadRequest, "bad_request", "provide config or profile_id")
	return nil, false
}

// renderProfile regenerates a profile's config.json from its stored inputs.
func (s *Server) renderProfile(profileID int64) ([]byte, error) {
	p, err := s.Store.GetProfile(profileID)
	if err != nil {
		return nil, err
	}
	t, err := s.Store.GetTemplate(p.TemplateID)
	if err != nil {
		return nil, err
	}
	opts := parseProfileOptions(p.Options)
	order, err := s.countryHeatOrder()
	if err != nil {
		return nil, err
	}
	return s.generateFromInputs(t.Content, p.NodeIDs, p.NodeGroupIDs, opts, p.OwnerUserID, false, order)
}

func (s *Server) renderProfileForUser(profileID, ownerUserID int64, allOwners bool) ([]byte, error) {
	p, err := s.Store.GetProfileForUser(profileID, ownerUserID, allOwners)
	if err != nil {
		return nil, err
	}
	t, err := s.Store.GetTemplateForUser(p.TemplateID, p.OwnerUserID, allOwners)
	if err != nil {
		return nil, err
	}
	opts := parseProfileOptions(p.Options)
	order, err := s.countryHeatOrder()
	if err != nil {
		return nil, err
	}
	return s.generateFromInputs(t.Content, p.NodeIDs, p.NodeGroupIDs, opts, p.OwnerUserID, allOwners, order)
}

// handleSubscription is the PUBLIC, unauthenticated endpoint returning a
// profile's config.json when the profile's public subscription switch is on.
// Anyone with an enabled link gets the full config including server secrets —
// tokens are 128-bit random and can be rotated.
func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request, token, profileName string) {
	if token == "" || profileName == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	u, err := s.Store.GetUserBySubscriptionToken(token)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	p, err := s.Store.GetProfileByNameForUser(profileName, u.ID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if !p.SubEnabled {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	t, err := s.Store.GetTemplate(p.TemplateID)
	if err != nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	opts := parseProfileOptions(p.Options)
	order, err := s.countryHeatOrder()
	if err != nil {
		http.Error(w, "generate error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	config, err := s.generateFromInputs(t.Content, p.NodeIDs, p.NodeGroupIDs, opts, p.OwnerUserID, false, order)
	if err != nil {
		http.Error(w, "generate error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(config)
}

func (s *Server) generateFromInputs(templateContent string, nodeIDs, groupIDs []int64, opts models.ProfileOptions, ownerUserID int64, allOwners bool, order []string) ([]byte, error) {
	if len(opts.GroupSelections) == 0 {
		nodes, err := s.resolveNodesForUser(nodeIDs, groupIDs, opts, ownerUserID, allOwners)
		if err != nil {
			return nil, err
		}
		return generateConfig(templateContent, nodes, opts, order)
	}
	groupNodes := make(map[string][]*models.Node, len(opts.GroupSelections))
	for tag, sel := range opts.GroupSelections {
		nodes, err := s.resolveNodesForUser(sel.NodeIDs, sel.NodeGroupIDs, opts, ownerUserID, allOwners)
		if err != nil {
			return nil, err
		}
		groupNodes[tag] = nodes
	}
	var chainNodes []*models.Node
	if opts.ChainProxy && opts.ChainProxySelected != nil {
		nodes, err := s.resolveNodesForUser(opts.ChainProxySelected.NodeIDs, opts.ChainProxySelected.NodeGroupIDs, opts, ownerUserID, allOwners)
		if err != nil {
			return nil, err
		}
		chainNodes = nodes
	}
	var autoCountryNodes []*models.Node
	if opts.AutoCountryGroups && opts.AutoCountrySelected != nil {
		nodes, err := s.resolveNodesForUser(opts.AutoCountrySelected.NodeIDs, opts.AutoCountrySelected.NodeGroupIDs, opts, ownerUserID, allOwners)
		if err != nil {
			return nil, err
		}
		autoCountryNodes = nodes
	}
	return generateConfigWithGroupSelections(templateContent, groupNodes, autoCountryNodes, chainNodes, opts, order)
}

func (s *Server) getNodesForUser(ids []int64, ownerUserID int64, allOwners bool) ([]*models.Node, error) {
	out := make([]*models.Node, 0, len(ids))
	for _, id := range ids {
		n, err := s.Store.GetNodeForUser(id, ownerUserID, allOwners)
		if err == store.ErrNotFound {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Server) resolveNodesForUser(nodeIDs, groupIDs []int64, opts models.ProfileOptions, ownerUserID int64, allOwners bool) ([]*models.Node, error) {
	var out []*models.Node
	seen := map[int64]bool{}
	addNode := func(id int64) error {
		if id == 0 || seen[id] {
			return nil
		}
		n, err := s.Store.GetNodeForUser(id, ownerUserID, allOwners)
		if err == store.ErrNotFound {
			return nil
		}
		if err != nil {
			return err
		}
		seen[id] = true
		out = append(out, n)
		return nil
	}
	for _, id := range nodeIDs {
		if err := addNode(id); err != nil {
			return nil, err
		}
	}
	groups, err := s.Store.GetNodeGroupsForUser(groupIDs, ownerUserID, allOwners)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		for _, id := range g.NodeIDs {
			if err := addNode(id); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
