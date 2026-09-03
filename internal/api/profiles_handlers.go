package api

import (
	"encoding/json"
	"net/http"
	"strings"

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
	if err := s.decorateProfilesValidation(list, ownerID, allOwners); err != nil {
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
	if err := s.decorateProfilesValidation([]*models.Profile{p}, ownerID, allOwners); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, p)
}

type profileRequest struct {
	Name                string                `json:"name"`
	TemplateID          int64                 `json:"template_id"`
	NodeIDs             []int64               `json:"node_ids"`
	NodeGroupIDs        []int64               `json:"node_group_ids"`
	Options             models.ProfileOptions `json:"options"`
	SubscriptionEnabled *bool                 `json:"subscription_enabled"`
}

type subscriptionEnabledRequest struct {
	SubscriptionEnabled bool `json:"subscription_enabled"`
}

// handleCreateProfile creates a profile. Public subscription access uses the
// owner's shared subscription token plus the profile name.
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
	req.NodeIDs = uniqueInt64s(req.NodeIDs)
	req.NodeGroupIDs = uniqueInt64s(req.NodeGroupIDs)
	normalizeProfileRequestOptions(&req)
	t, err := s.Store.GetTemplateForUser(req.TemplateID, u.ID, false)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "template not found")
		return
	}
	if err := validateOptionOutboundRefs(t.Content, req.Options); err != nil {
		respondBadGenerationRequest(w, err)
		return
	}
	if err := validateOptionGroupInputs(t.Content, req.Options); err != nil {
		respondBadGenerationRequest(w, err)
		return
	}
	if !validateAutoCountrySelection(w, req.Options) {
		return
	}
	if !s.validateNodeAccess(w, u, req.NodeIDs) {
		return
	}
	if !s.validateNodeGroupAccess(w, u, req.NodeGroupIDs) {
		return
	}
	if req.Options.ChainProxy && !s.validateNodeAccess(w, u, req.Options.ChainProxyNodeIDs) {
		return
	}
	if !s.validateOptionSelectionAccess(w, u.ID, false, req.Options) {
		return
	}
	if !validateChainProxySelection(w, req.NodeIDs, req.NodeGroupIDs, req.Options) {
		return
	}
	if !s.checkQuota(w, u, quotaProfiles, 1) {
		return
	}
	p := &models.Profile{
		OwnerUserID:   u.ID,
		Name:          req.Name,
		TemplateID:    req.TemplateID,
		Options:       marshalOptions(req.Options),
		SubEnabled:    requestSubscriptionEnabled(req, true),
		SubEnabledSet: true,
		NodeIDs:       req.NodeIDs,
		NodeGroupIDs:  req.NodeGroupIDs,
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
	if _, ok := requireCurrentUser(w, r); !ok {
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
	if req.Name == "" || req.TemplateID == 0 {
		respondError(w, http.StatusBadRequest, "bad_request", "name and template_id are required")
		return
	}
	req.NodeIDs = uniqueInt64s(req.NodeIDs)
	req.NodeGroupIDs = uniqueInt64s(req.NodeGroupIDs)
	normalizeProfileRequestOptions(&req)
	refOwner := existing.OwnerUserID
	refAllOwners := false
	t, err := s.Store.GetTemplateForUser(req.TemplateID, refOwner, refAllOwners)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "template not found")
		return
	}
	if err := validateOptionOutboundRefs(t.Content, req.Options); err != nil {
		respondBadGenerationRequest(w, err)
		return
	}
	if err := validateOptionGroupInputs(t.Content, req.Options); err != nil {
		respondBadGenerationRequest(w, err)
		return
	}
	if !validateAutoCountrySelection(w, req.Options) {
		return
	}
	if !s.validateNodeAccessForOwner(w, refOwner, refAllOwners, req.NodeIDs) {
		return
	}
	if !s.validateNodeGroupAccessForOwner(w, refOwner, refAllOwners, req.NodeGroupIDs) {
		return
	}
	if req.Options.ChainProxy && !s.validateNodeAccessForOwner(w, refOwner, refAllOwners, req.Options.ChainProxyNodeIDs) {
		return
	}
	if !s.validateOptionSelectionAccess(w, refOwner, refAllOwners, req.Options) {
		return
	}
	if !validateChainProxySelection(w, req.NodeIDs, req.NodeGroupIDs, req.Options) {
		return
	}
	existing.Name = req.Name
	existing.TemplateID = req.TemplateID
	existing.Options = marshalOptions(req.Options)
	existing.SubEnabled = requestSubscriptionEnabled(req, existing.SubEnabled)
	existing.NodeIDs = req.NodeIDs
	existing.NodeGroupIDs = req.NodeGroupIDs
	if err := s.Store.UpdateProfile(existing); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

func (s *Server) handleSetProfileSubscriptionEnabled(w http.ResponseWriter, r *http.Request) {
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
	var req subscriptionEnabledRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.Store.SetProfileSubscriptionEnabled(p.ID, p.OwnerUserID, req.SubscriptionEnabled); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	p.SubEnabled = req.SubscriptionEnabled
	respondJSON(w, http.StatusOK, p)
}

// handleDeleteProfile removes a profile.
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	p, err := s.Store.GetProfileForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "profile not found")
		return
	} else if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if err := s.Store.DeleteProfileForUser(p.ID, p.OwnerUserID); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "profile not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func marshalOptions(o models.ProfileOptions) string {
	o.ChainProxyNodeIDs = uniqueInt64s(o.ChainProxyNodeIDs)
	o.GroupSelections = normalizeGroupSelections(o.GroupSelections)
	if o.AutoCountrySelected != nil {
		normalized := normalizeNodeSelection(*o.AutoCountrySelected)
		o.AutoCountrySelected = &normalized
	}
	if o.AutoCountryGroups && o.AutoCountrySelected == nil && len(o.GroupSelections) > 0 {
		normalized := normalizeNodeSelection(mergeGroupSelections(o.GroupSelections))
		if selectionHasInputs(normalized) {
			o.AutoCountrySelected = &normalized
		}
	}
	if !o.AutoCountryGroups {
		o.AutoCountrySelected = nil
	}
	if o.ChainProxySelected != nil {
		normalized := normalizeNodeSelection(*o.ChainProxySelected)
		o.ChainProxySelected = &normalized
	}
	if !o.ChainProxy {
		o.ChainProxyNodeID = 0
		o.ChainProxyNodeIDs = nil
		o.ChainProxySelected = nil
	} else if len(o.ChainProxyNodeIDs) > 0 {
		o.ChainProxyNodeID = 0
	}
	b, err := json.Marshal(o)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func requestSubscriptionEnabled(req profileRequest, fallback bool) bool {
	if req.SubscriptionEnabled == nil {
		return fallback
	}
	return *req.SubscriptionEnabled
}

func normalizeProfileRequestOptions(req *profileRequest) {
	req.Options.GroupSelections = normalizeGroupSelections(req.Options.GroupSelections)
	if req.Options.AutoCountrySelected != nil {
		normalized := normalizeNodeSelection(*req.Options.AutoCountrySelected)
		req.Options.AutoCountrySelected = &normalized
	}
	if req.Options.AutoCountryGroups && req.Options.AutoCountrySelected == nil && len(req.Options.GroupSelections) > 0 {
		normalized := normalizeNodeSelection(mergeGroupSelections(req.Options.GroupSelections))
		if selectionHasInputs(normalized) {
			req.Options.AutoCountrySelected = &normalized
		}
	}
	if !req.Options.AutoCountryGroups {
		req.Options.AutoCountrySelected = nil
	}
	if req.Options.ChainProxySelected != nil {
		normalized := normalizeNodeSelection(*req.Options.ChainProxySelected)
		req.Options.ChainProxySelected = &normalized
	}
	if !req.Options.ChainProxy {
		req.Options.ChainProxyNodeID = 0
		req.Options.ChainProxyNodeIDs = nil
		req.Options.ChainProxySelected = nil
		return
	}
	req.Options.ChainProxyNodeIDs = uniqueInt64s(req.Options.ChainProxyNodeIDs)
	if len(req.Options.ChainProxyNodeIDs) == 0 && req.Options.ChainProxyNodeID != 0 {
		req.Options.ChainProxyNodeIDs = []int64{req.Options.ChainProxyNodeID}
	}
}

func validateAutoCountrySelection(w http.ResponseWriter, opts models.ProfileOptions) bool {
	if !opts.AutoCountryGroups || len(opts.GroupSelections) == 0 {
		return true
	}
	if opts.AutoCountrySelected == nil || !selectionHasInputs(*opts.AutoCountrySelected) {
		respondErrorWithDetails(w, http.StatusBadRequest, "bad_request", "auto country group nodes are required",
			generationErrorDetails{Kind: generateErrAutoCountryEmpty, Panel: "country"})
		return false
	}
	return true
}

func mergeGroupSelections(selections map[string]models.NodeSelection) models.NodeSelection {
	var out models.NodeSelection
	for _, sel := range selections {
		out.NodeIDs = append(out.NodeIDs, sel.NodeIDs...)
		out.NodeGroupIDs = append(out.NodeGroupIDs, sel.NodeGroupIDs...)
	}
	return normalizeNodeSelection(out)
}

func validateChainProxySelection(w http.ResponseWriter, nodeIDs, nodeGroupIDs []int64, opts models.ProfileOptions) bool {
	if !opts.ChainProxy {
		return true
	}
	if len(opts.GroupSelections) > 0 {
		if opts.ChainProxySelected == nil || (len(opts.ChainProxySelected.NodeIDs) == 0 && len(opts.ChainProxySelected.NodeGroupIDs) == 0) {
			respondErrorWithDetails(w, http.StatusBadRequest, "bad_request", "chain proxy nodes are required",
				generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"})
			return false
		}
		return true
	}
	if len(opts.ChainProxyNodeIDs) == 0 {
		respondErrorWithDetails(w, http.StatusBadRequest, "bad_request", "chain proxy nodes are required",
			generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"})
		return false
	}
	selected := make(map[int64]bool, len(nodeIDs))
	for _, id := range nodeIDs {
		selected[id] = true
	}
	for _, id := range opts.ChainProxyNodeIDs {
		if !selected[id] {
			respondErrorWithDetails(w, http.StatusBadRequest, "bad_request", "chain proxy nodes must be selected as single nodes",
				generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"})
			return false
		}
	}
	if len(opts.ChainProxyNodeIDs) >= len(nodeIDs) && len(nodeGroupIDs) == 0 {
		respondErrorWithDetails(w, http.StatusBadRequest, "bad_request", "chain proxy needs at least one upstream node",
			generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"})
		return false
	}
	return true
}

func normalizeGroupSelections(in map[string]models.NodeSelection) map[string]models.NodeSelection {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]models.NodeSelection, len(in))
	for tag, sel := range in {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		out[tag] = normalizeNodeSelection(sel)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeNodeSelection(sel models.NodeSelection) models.NodeSelection {
	return models.NodeSelection{
		NodeIDs:           uniqueInt64s(sel.NodeIDs),
		NodeGroupIDs:      uniqueInt64s(sel.NodeGroupIDs),
		OutboundRefs:      normalizeOutboundRefList(sel.OutboundRefs),
		SkipCountryGroups: sel.SkipCountryGroups,
	}
}

func normalizeOutboundRefList(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s *Server) validateNodeAccess(w http.ResponseWriter, u *models.User, nodeIDs []int64) bool {
	return s.validateNodeAccessForOwner(w, u.ID, false, nodeIDs)
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

func (s *Server) validateNodeGroupAccess(w http.ResponseWriter, u *models.User, groupIDs []int64) bool {
	return s.validateNodeGroupAccessForOwner(w, u.ID, false, groupIDs)
}

func (s *Server) validateNodeGroupAccessForOwner(w http.ResponseWriter, ownerUserID int64, allOwners bool, groupIDs []int64) bool {
	for _, id := range groupIDs {
		group, err := s.Store.GetNodeGroupForUser(id, ownerUserID, allOwners)
		if err == store.ErrNotFound {
			respondError(w, http.StatusBadRequest, "bad_request", "node group not found")
			return false
		} else if err != nil {
			respondError(w, http.StatusInternalServerError, "internal", err.Error())
			return false
		}
		if len(group.NodeIDs) == 0 {
			respondError(w, http.StatusBadRequest, "bad_request", "node group is empty")
			return false
		}
	}
	return true
}

func (s *Server) validateOptionSelectionAccess(w http.ResponseWriter, ownerUserID int64, allOwners bool, opts models.ProfileOptions) bool {
	for _, sel := range opts.GroupSelections {
		if !s.validateNodeAccessForOwner(w, ownerUserID, allOwners, sel.NodeIDs) {
			return false
		}
		if !s.validateNodeGroupAccessForOwner(w, ownerUserID, allOwners, sel.NodeGroupIDs) {
			return false
		}
	}
	if opts.ChainProxySelected != nil {
		if !s.validateNodeAccessForOwner(w, ownerUserID, allOwners, opts.ChainProxySelected.NodeIDs) {
			return false
		}
		if !s.validateNodeGroupAccessForOwner(w, ownerUserID, allOwners, opts.ChainProxySelected.NodeGroupIDs) {
			return false
		}
	}
	if opts.AutoCountrySelected != nil {
		if !s.validateNodeAccessForOwner(w, ownerUserID, allOwners, opts.AutoCountrySelected.NodeIDs) {
			return false
		}
		if !s.validateNodeGroupAccessForOwner(w, ownerUserID, allOwners, opts.AutoCountrySelected.NodeGroupIDs) {
			return false
		}
	}
	return true
}
