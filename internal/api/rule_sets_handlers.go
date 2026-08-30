package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/ruleset"
	"github.com/mora1n/sb-fox/internal/store"
)

const ruleSetRequestOverhead = int64(1 << 20)

type ruleSetRequest struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Sources     []ruleSetSourceRequest `json:"sources"`
}

type ruleSetSourceRequest struct {
	Kind    string `json:"kind"`
	Format  string `json:"format"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

type ruleSetErrorDetails struct {
	Kind         string `json:"kind"`
	Stage        string `json:"stage"`
	SourceIndex  *int   `json:"source_index,omitempty"`
	SourceKind   string `json:"source_kind,omitempty"`
	SourceFormat string `json:"source_format,omitempty"`
	URL          string `json:"url,omitempty"`
	Message      string `json:"message"`
}

func (s *Server) handleListRuleSets(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	items, err := s.Store.ListRuleSets(ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, items)
}

func (s *Server) handleGetRuleSet(w http.ResponseWriter, r *http.Request) {
	item, ok := s.ruleSetForRequest(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateRuleSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	req, ok := decodeRuleSetRequest(w, r)
	if !ok {
		return
	}
	item := requestRuleSet(user.ID, 0, req)
	if !s.publishRuleSet(w, r, user, item) {
		return
	}
	id, err := s.Store.CreateRuleSet(item)
	if err != nil {
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	created, err := s.Store.GetRuleSetForUser(id, user.ID, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateRuleSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	existing, ok := s.ruleSetForRequest(w, r)
	if !ok {
		return
	}
	req, ok := decodeRuleSetRequest(w, r)
	if !ok {
		return
	}
	item := requestRuleSet(existing.OwnerUserID, existing.ID, req)
	if !s.publishRuleSet(w, r, user, item) {
		return
	}
	if err := s.Store.UpdateRuleSet(item); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "rule-set not found")
			return
		}
		respondError(w, http.StatusConflict, "conflict", err.Error())
		return
	}
	updated, err := s.Store.GetRuleSetForUser(item.ID, item.OwnerUserID, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRefreshRuleSet(w http.ResponseWriter, r *http.Request) {
	user, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	item, ok := s.ruleSetForRequest(w, r)
	if !ok {
		return
	}
	artifact, err := s.buildRuleSet(r, user, item.Sources)
	if err != nil {
		_ = s.Store.RecordRuleSetFailure(item.ID, item.OwnerUserID, err.Error())
		respondRuleSetPublishError(w, err)
		return
	}
	applyRuleSetArtifact(item, artifact)
	if err := s.Store.RefreshRuleSetArtifact(item); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	refreshed, err := s.Store.GetRuleSetForUser(item.ID, item.OwnerUserID, false)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, refreshed)
}

func (s *Server) handleDeleteRuleSet(w http.ResponseWriter, r *http.Request) {
	item, ok := s.ruleSetForRequest(w, r)
	if !ok {
		return
	}
	if err := s.Store.DeleteRuleSetForUser(item.ID, item.OwnerUserID); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "rule-set not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleBulkDeleteRuleSets(w http.ResponseWriter, r *http.Request) {
	ids, ok := decodeBulkIDs(w, r)
	if !ok {
		return
	}
	ownerID, allOwners := ownerScope(r)
	if allOwners {
		respondError(w, http.StatusForbidden, "forbidden", "cross-owner bulk delete is not supported")
		return
	}
	deleted, err := s.Store.DeleteRuleSetsForUser(ids, ownerID)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "rule-set not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, bulkDeleteResponse{Deleted: deleted})
}

func (s *Server) ruleSetForRequest(w http.ResponseWriter, r *http.Request) (*models.RuleSet, bool) {
	ownerID, allOwners := ownerScope(r)
	item, err := s.Store.GetRuleSetForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "rule-set not found")
		return nil, false
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return nil, false
	}
	return item, true
}

func decodeRuleSetRequest(w http.ResponseWriter, r *http.Request) (ruleSetRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, ruleset.MaxTotalBytes+ruleSetRequestOverhead)
	var req ruleSetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(w, http.StatusRequestEntityTooLarge, "too_large", err.Error())
			return ruleSetRequest{}, false
		}
		respondError(w, http.StatusBadRequest, "bad_request", "invalid JSON body: "+err.Error())
		return ruleSetRequest{}, false
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "name is required")
		return ruleSetRequest{}, false
	}
	if len(req.Sources) == 0 {
		respondError(w, http.StatusBadRequest, "bad_request", "at least one source is required")
		return ruleSetRequest{}, false
	}
	return req, true
}

func requestRuleSet(ownerID, id int64, req ruleSetRequest) *models.RuleSet {
	item := &models.RuleSet{ID: id, OwnerUserID: ownerID, Name: req.Name, Description: req.Description}
	item.Sources = make([]*models.RuleSetSource, 0, len(req.Sources))
	for position, input := range req.Sources {
		item.Sources = append(item.Sources, &models.RuleSetSource{
			RuleSetID: id,
			Kind:      strings.TrimSpace(input.Kind),
			Format:    strings.TrimSpace(input.Format),
			URL:       strings.TrimSpace(input.URL),
			Content:   input.Content,
			Position:  position,
		})
	}
	item.SourceCount = len(item.Sources)
	return item
}

func (s *Server) publishRuleSet(w http.ResponseWriter, r *http.Request, user *models.User, item *models.RuleSet) bool {
	artifact, err := s.buildRuleSet(r, user, item.Sources)
	if err != nil {
		respondRuleSetPublishError(w, err)
		return false
	}
	applyRuleSetArtifact(item, artifact)
	return true
}

func (s *Server) buildRuleSet(r *http.Request, user *models.User, sources []*models.RuleSetSource) (ruleset.Artifact, error) {
	if s.Fetcher == nil {
		return ruleset.Artifact{}, &ruleset.Error{Stage: "fetch", Err: errors.New("rule-set fetcher is unavailable")}
	}
	runtime, err := s.activeKernelForUser(user)
	if err != nil {
		return ruleset.Artifact{}, &ruleset.Error{Stage: "kernel", Err: err}
	}
	publisher := &ruleset.Publisher{Fetcher: s.Fetcher}
	return publisher.Publish(r.Context(), sources, runtime)
}

func applyRuleSetArtifact(item *models.RuleSet, artifact ruleset.Artifact) {
	item.PublishedJSON = artifact.JSON
	item.PublishedSRS = artifact.SRS
	item.RuleCount = artifact.RuleCount
	item.JSONSize = int64(len(artifact.JSON))
	item.SRSSize = int64(len(artifact.SRS))
	item.JSONSHA256 = artifact.JSONSHA256
	item.SRSSHA256 = artifact.SRSSHA256
	item.KernelVersion = artifact.KernelVersion
}

func respondRuleSetPublishError(w http.ResponseWriter, err error) {
	status := http.StatusUnprocessableEntity
	var limit *ruleset.LimitError
	if errors.As(err, &limit) {
		status = http.StatusRequestEntityTooLarge
	}
	details := ruleSetErrorDetails{Kind: "rule_set_publish_error", Stage: "publish", Message: err.Error()}
	var publishErr *ruleset.Error
	if errors.As(err, &publishErr) {
		details.Stage = publishErr.Stage
		details.SourceIndex = publishErr.SourceIndex
		details.SourceKind = publishErr.SourceKind
		details.SourceFormat = publishErr.SourceFormat
		details.URL = publishErr.URL
		details.Message = publishErr.Err.Error()
	}
	respondErrorWithDetails(w, status, "ruleset_publish_failed", "rule-set publish failed", details)
}
