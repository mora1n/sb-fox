package api

import (
	"net/http"
	"time"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/sblink"
)

// handleImportLinks parses share-links into nodes (requirement d.1).
func (s *Server) handleImportLinks(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Links string `json:"links"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	outbounds, warnings, err := sblink.ParseManyWithWarnings(req.Links)
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse_error", err.Error())
		return
	}
	result, ok := s.persistOutbounds(w, u, outbounds, "protocol", nil)
	if !ok {
		return
	}
	respondJSON(w, http.StatusCreated, importResponse(result.Nodes, 0, result.Deduped, warnings))
}

// handleImportConfig extracts outbound nodes from an uploaded config or a
// template's content (requirement d.2). Group outbounds (selector/urltest/
// direct/block/dns) are skipped — only real proxy nodes are imported.
func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Config string `json:"config"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	cfg, err := merge.ParseOrdered([]byte(req.Config))
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", "config is not valid JSON: "+err.Error())
		return
	}
	raw, ok := cfg.Get("outbounds")
	if !ok {
		respondError(w, http.StatusBadRequest, "bad_request", "config has no outbounds")
		return
	}
	arr, _ := raw.([]any)
	var proxies []*merge.OrderedMap
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok || !isProxyOutbound(om.GetString("type")) {
			continue
		}
		proxies = append(proxies, om)
	}
	result, ok := s.persistOutbounds(w, u, proxies, "config", nil)
	if !ok {
		return
	}
	respondJSON(w, http.StatusCreated, importResponse(result.Nodes, 0, result.Deduped, nil))
}

// handleImportSubscription creates a source, fetches it and imports nodes
// (requirements b, d). Nodes may be share-links or a base64 blob of links.
func (s *Server) handleImportSubscription(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "url is required")
		return
	}
	sourceID, err := s.Store.CreateSource(u.ID, req.Name, req.URL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	nodes, deduped, warnings, ferr := s.fetchSourceNodes(u, sourceID, req.URL)
	if ferr != nil {
		if ferr == errQuotaExceeded {
			_ = s.Store.DeleteSource(sourceID)
			respondError(w, http.StatusForbidden, "quota_exceeded", "nodes limit exceeded")
			return
		}
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusCreated, importResponse(nodes, sourceID, deduped, warnings))
}

// handleRefreshSource re-fetches a subscription source, replacing its nodes.
func (s *Server) handleRefreshSource(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	id := pathID(r)
	ownerID, allOwners := ownerScope(r)
	src, err := s.Store.GetSourceForUser(id, ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	nodes, deduped, _, ferr := s.refreshSourceNodes(u, src)
	if ferr != nil {
		if ferr == errQuotaExceeded {
			respondError(w, http.StatusForbidden, "quota_exceeded", "nodes limit exceeded")
			return
		}
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"imported": len(nodes), "deduped": deduped, "nodes": nodes})
}

// fetchSourceNodes fetches a subscription URL, parses links, persists nodes and
// records the fetch outcome on the source.
func (s *Server) fetchSourceNodes(user *models.User, sourceID int64, url string) ([]*models.Node, int, []string, error) {
	outbounds, warnings, err := s.fetchSourceOutbounds(sourceID, url)
	if err != nil {
		return nil, 0, warnings, err
	}
	nodes := nodesFromOutbounds(user.ID, outbounds, "subscription", &sourceID)
	nodes, deduped, err := s.dedupeNodesForUser(user.ID, nodes, nil)
	if err != nil {
		return nil, 0, warnings, err
	}
	if ok, _, err := s.quotaAllowed(user, quotaNodes, len(nodes)); err != nil {
		return nil, 0, warnings, err
	} else if !ok {
		return nil, 0, warnings, errQuotaExceeded
	}
	created, err := s.insertNodes(nodes)
	if len(warnings) > 0 {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok with warnings", len(created))
	} else {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok", len(created))
	}
	return created, deduped, warnings, err
}

func (s *Server) refreshSourceNodes(user *models.User, src *models.SubscriptionSource) ([]*models.Node, int, []string, error) {
	outbounds, warnings, err := s.fetchSourceOutbounds(src.ID, src.URL)
	if err != nil {
		return nil, 0, warnings, err
	}
	nodes := nodesFromOutbounds(src.OwnerUserID, outbounds, "subscription", &src.ID)
	nodes, deduped, err := s.dedupeNodesForUser(src.OwnerUserID, nodes, &src.ID)
	if err != nil {
		return nil, 0, warnings, err
	}
	oldCount, err := s.Store.CountNodesBySource(src.ID)
	if err != nil {
		return nil, 0, warnings, err
	}
	if ok, _, err := s.quotaAllowed(user, quotaNodes, len(nodes)-oldCount); err != nil {
		return nil, 0, warnings, err
	} else if !ok {
		return nil, 0, warnings, errQuotaExceeded
	}
	if err := s.Store.DeleteNodesBySourceForUser(src.ID, src.OwnerUserID); err != nil {
		return nil, 0, warnings, err
	}
	created, err := s.insertNodes(nodes)
	if len(warnings) > 0 {
		_ = s.Store.UpdateSourceFetch(src.ID, "ok with warnings", len(created))
	} else {
		_ = s.Store.UpdateSourceFetch(src.ID, "ok", len(created))
	}
	return created, deduped, warnings, err
}

func (s *Server) fetchSourceOutbounds(sourceID int64, url string) ([]*merge.OrderedMap, []string, error) {
	ctx, cancel := contextWithTimeout(25 * time.Second)
	defer cancel()

	body, err := s.Fetcher.Fetch(ctx, url)
	if err != nil {
		_ = s.Store.UpdateSourceFetch(sourceID, "error: "+err.Error(), 0)
		return nil, nil, err
	}
	outbounds, warnings, err := sblink.ParseManyWithWarnings(body)
	if err != nil {
		_ = s.Store.UpdateSourceFetch(sourceID, "error: "+err.Error(), 0)
		return nil, warnings, err
	}
	if len(warnings) > 0 {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok with warnings", len(outbounds))
	}
	return outbounds, warnings, nil
}

func (s *Server) insertNodes(nodes []*models.Node) ([]*models.Node, error) {
	var created []*models.Node
	sourceID := int64(0)
	if len(nodes) > 0 && nodes[0].SourceRef != nil {
		sourceID = *nodes[0].SourceRef
	}
	for _, n := range nodes {
		id, err := s.Store.CreateNode(n)
		if err != nil {
			continue
		}
		n.ID = id
		created = append(created, n)
	}
	if sourceID != 0 {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok", len(created))
	}
	return created, nil
}

// handleRefreshCountry re-runs country detection over selected node ids
// (requirement e). Nodes with a manual override are left untouched.
func (s *Server) handleRefreshCountry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeIDs []int64 `json:"node_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	updated := 0
	for _, id := range req.NodeIDs {
		ownerID, allOwners := ownerScope(r)
		n, err := s.Store.GetNodeForUser(id, ownerID, allOwners)
		if err != nil {
			continue
		}
		if n.CountrySource == "manual" {
			continue
		}
		info := merge.DetectCountry(n.Tag)
		code := ""
		if info != nil {
			code = info.Code
		}
		if code != n.CountryCode {
			n.CountryCode = code
			n.CountrySource = "auto"
			if err := s.Store.UpdateNode(n); err == nil {
				updated++
			}
		}
	}
	respondJSON(w, http.StatusOK, map[string]int{"updated": updated})
}

type nodeImportResult struct {
	Nodes   []*models.Node
	Deduped int
}

// persistOutbounds saves a batch of parsed outbounds as nodes and returns them.
// On zero valid parsed nodes it writes a 400 and returns false.
func (s *Server) persistOutbounds(w http.ResponseWriter, user *models.User, outbounds []*merge.OrderedMap, source string, sourceRef *int64) (nodeImportResult, bool) {
	nodes := nodesFromOutbounds(user.ID, outbounds, source, sourceRef)
	if len(nodes) == 0 {
		respondError(w, http.StatusBadRequest, "no_nodes", "no valid nodes were imported")
		return nodeImportResult{}, false
	}
	nodes, deduped, err := s.dedupeNodesForUser(user.ID, nodes, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return nodeImportResult{}, false
	}
	if !s.checkQuota(w, user, quotaNodes, len(nodes)) {
		return nodeImportResult{}, false
	}
	created, err := s.insertNodes(nodes)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return nodeImportResult{}, false
	}
	return nodeImportResult{Nodes: created, Deduped: deduped}, true
}

func importResponse(nodes []*models.Node, sourceID int64, deduped int, warnings []string) map[string]any {
	resp := map[string]any{"imported": len(nodes), "nodes": nodes}
	if sourceID != 0 {
		resp["source_id"] = sourceID
	}
	if deduped > 0 {
		resp["deduped"] = deduped
	}
	if len(warnings) > 0 {
		resp["warnings"] = warnings
	}
	return resp
}

func nodesFromOutbounds(ownerUserID int64, outbounds []*merge.OrderedMap, source string, sourceRef *int64) []*models.Node {
	var nodes []*models.Node
	for _, ob := range outbounds {
		n, err := nodeFromOutbound(ob, source, sourceRef)
		if err != nil {
			continue
		}
		n.OwnerUserID = ownerUserID
		nodes = append(nodes, n)
	}
	return nodes
}

// isProxyOutbound reports whether a type is a real proxy node (vs a group or
// pseudo-outbound like direct/block/dns/selector/urltest).
func isProxyOutbound(typ string) bool {
	switch typ {
	case "selector", "urltest", "direct", "block", "dns", "":
		return false
	default:
		return true
	}
}
