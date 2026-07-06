package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
	"github.com/mora1n/sb-fox/internal/sblink"
	"github.com/mora1n/sb-fox/internal/subfetch"
)

// handlePreviewImportLinks parses share-links and reports what would be
// imported without writing nodes.
func (s *Server) handlePreviewImportLinks(w http.ResponseWriter, r *http.Request) {
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
	s.respondImportPreview(w, u, outbounds, "protocol", nil, warnings, nil)
}

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
	respondJSON(w, http.StatusCreated, importResponse(result.Nodes, 0, result.Deduped, warnings, nil))
}

// handlePreviewImportConfig extracts importable outbound nodes without writing
// nodes.
func (s *Server) handlePreviewImportConfig(w http.ResponseWriter, r *http.Request) {
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
	proxies, err := proxyOutboundsFromConfig(req.Config)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.respondImportPreview(w, u, proxies, "config", nil, nil, nil)
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
	proxies, err := proxyOutboundsFromConfig(req.Config)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	result, ok := s.persistOutbounds(w, u, proxies, "config", nil)
	if !ok {
		return
	}
	respondJSON(w, http.StatusCreated, importResponse(result.Nodes, 0, result.Deduped, nil, nil))
}

// handlePreviewImportSubscription fetches and parses a subscription URL without
// creating a source or writing nodes.
func (s *Server) handlePreviewImportSubscription(w http.ResponseWriter, r *http.Request) {
	u, ok := requireCurrentUser(w, r)
	if !ok {
		return
	}
	var req struct {
		URL string `json:"url"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.URL == "" {
		respondError(w, http.StatusBadRequest, "bad_request", "url is required")
		return
	}
	fetched, err := s.previewSourceOutbounds(req.URL)
	if err != nil {
		respondError(w, http.StatusBadGateway, "fetch_error", err.Error())
		return
	}
	s.respondImportPreview(w, u, fetched.Outbounds, "subscription", nil, fetched.Warnings, fetched.Fetches)
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
	nodes, deduped, warnings, fetches, ferr := s.fetchSourceNodes(u, sourceID, req.URL)
	if ferr != nil {
		if ferr == errQuotaExceeded {
			_ = s.Store.DeleteSource(sourceID)
			respondError(w, http.StatusForbidden, "quota_exceeded", "nodes limit exceeded")
			return
		}
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusCreated, importResponse(nodes, sourceID, deduped, warnings, fetches))
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
	nodes, deduped, warnings, fetches, ferr := s.refreshSourceNodes(u, src)
	if ferr != nil {
		if ferr == errQuotaExceeded {
			respondError(w, http.StatusForbidden, "quota_exceeded", "nodes limit exceeded")
			return
		}
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusOK, importResponse(nodes, src.ID, deduped, warnings, fetches))
}

// fetchSourceNodes fetches a subscription URL, parses links, persists nodes and
// records the fetch outcome on the source.
func (s *Server) fetchSourceNodes(user *models.User, sourceID int64, url string) ([]*models.Node, int, []string, []subfetch.BatchItem, error) {
	fetched, err := s.fetchSourceOutbounds(sourceID, url, subfetch.Options{})
	if err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	nodes := nodesFromOutbounds(user.ID, fetched.Outbounds, "subscription", &sourceID)
	nodes, deduped, err := s.dedupeNodesForUser(user.ID, nodes, nil)
	if err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	if ok, _, err := s.quotaAllowed(user, quotaNodes, len(nodes)); err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	} else if !ok {
		return nil, 0, fetched.Warnings, fetched.Fetches, errQuotaExceeded
	}
	created, err := s.insertNodes(nodes)
	if len(fetched.Warnings) > 0 {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok with warnings", len(created))
	} else {
		_ = s.Store.UpdateSourceFetch(sourceID, "ok", len(created))
	}
	return created, deduped, fetched.Warnings, fetched.Fetches, err
}

func (s *Server) refreshSourceNodes(user *models.User, src *models.SubscriptionSource) ([]*models.Node, int, []string, []subfetch.BatchItem, error) {
	fetched, err := s.fetchSourceOutbounds(src.ID, src.URL, subfetch.Options{NoCache: true})
	if err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	nodes := nodesFromOutbounds(src.OwnerUserID, fetched.Outbounds, "subscription", &src.ID)
	nodes, deduped, err := s.dedupeNodesForUser(src.OwnerUserID, nodes, &src.ID)
	if err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	oldCount, err := s.Store.CountNodesBySource(src.ID)
	if err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	if ok, _, err := s.quotaAllowed(user, quotaNodes, len(nodes)-oldCount); err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	} else if !ok {
		return nil, 0, fetched.Warnings, fetched.Fetches, errQuotaExceeded
	}
	if err := s.Store.DeleteNodesBySourceForUser(src.ID, src.OwnerUserID); err != nil {
		return nil, 0, fetched.Warnings, fetched.Fetches, err
	}
	created, err := s.insertNodes(nodes)
	if len(fetched.Warnings) > 0 {
		_ = s.Store.UpdateSourceFetch(src.ID, "ok with warnings", len(created))
	} else {
		_ = s.Store.UpdateSourceFetch(src.ID, "ok", len(created))
	}
	return created, deduped, fetched.Warnings, fetched.Fetches, err
}

type sourceOutboundsResult struct {
	Outbounds []*merge.OrderedMap
	Warnings  []string
	Fetches   []subfetch.BatchItem
}

func (s *Server) fetchSourceOutbounds(sourceID int64, url string, opts subfetch.Options) (sourceOutboundsResult, error) {
	result, err := s.loadSourceOutbounds(url, opts)
	if err != nil {
		_ = s.Store.UpdateSourceFetch(sourceID, "error: "+err.Error(), 0)
	}
	return result, err
}

func (s *Server) previewSourceOutbounds(url string) (sourceOutboundsResult, error) {
	return s.loadSourceOutbounds(url, subfetch.Options{})
}

func (s *Server) loadSourceOutbounds(url string, opts subfetch.Options) (sourceOutboundsResult, error) {
	ctx, cancel := contextWithTimeout(25 * time.Second)
	defer cancel()

	batch, fetchErr := s.Fetcher.FetchMany(ctx, url, opts)
	result := sourceOutboundsResult{Fetches: batch.Items}
	for i := range result.Fetches {
		item := &result.Fetches[i]
		if !item.OK {
			if item.Error != "" {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", item.URL, item.Error))
			}
			continue
		}
		outbounds, warnings, err := sblink.ParseManyWithWarnings(item.Body)
		if err != nil {
			item.OK = false
			item.Error = err.Error()
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", item.URL, err.Error()))
			continue
		}
		item.Nodes = len(outbounds)
		result.Outbounds = append(result.Outbounds, outbounds...)
		for _, warning := range warnings {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %s", item.URL, warning))
		}
	}
	if len(result.Outbounds) == 0 {
		if len(result.Warnings) == 0 && fetchErr != nil {
			return result, fetchErr
		}
		if len(result.Warnings) > 0 {
			return result, errors.New(strings.Join(result.Warnings, "; "))
		}
		return result, errors.New("no valid nodes were imported")
	}
	return result, nil
}

func (s *Server) insertNodes(nodes []*models.Node) ([]*models.Node, error) {
	var created []*models.Node
	for _, n := range nodes {
		id, err := s.Store.CreateNode(n)
		if err != nil {
			continue
		}
		n.ID = id
		created = append(created, n)
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

type importPreviewResponse struct {
	Parsed     int                  `json:"parsed"`
	Importable int                  `json:"importable"`
	Deduped    int                  `json:"deduped"`
	Nodes      []importPreviewNode  `json:"nodes"`
	Warnings   []string             `json:"warnings,omitempty"`
	Fetches    []subfetch.BatchItem `json:"fetches,omitempty"`
}

type importPreviewNode struct {
	Tag           string `json:"tag"`
	Type          string `json:"type"`
	Server        string `json:"server"`
	ServerPort    int    `json:"server_port"`
	CountryCode   string `json:"country_code"`
	CountrySource string `json:"country_source"`
	Source        string `json:"source"`
	HasDetour     bool   `json:"has_detour"`
	Detour        string `json:"detour,omitempty"`
}

func (s *Server) respondImportPreview(w http.ResponseWriter, user *models.User, outbounds []*merge.OrderedMap, source string, sourceRef *int64, warnings []string, fetches []subfetch.BatchItem) {
	nodes := nodesFromOutbounds(user.ID, outbounds, source, sourceRef)
	if len(nodes) == 0 {
		respondError(w, http.StatusBadRequest, "no_nodes", "no valid nodes were imported")
		return
	}
	parsed := len(nodes)
	nodes, deduped, err := s.dedupeNodesForUser(user.ID, nodes, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if !s.checkQuota(w, user, quotaNodes, len(nodes)) {
		return
	}
	respondJSON(w, http.StatusOK, importPreviewResponse{
		Parsed:     parsed,
		Importable: len(nodes),
		Deduped:    deduped,
		Nodes:      importPreviewNodes(nodes),
		Warnings:   warnings,
		Fetches:    fetches,
	})
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

func importPreviewNodes(nodes []*models.Node) []importPreviewNode {
	out := make([]importPreviewNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, importPreviewNode{
			Tag:           n.Tag,
			Type:          n.Type,
			Server:        n.Server,
			ServerPort:    n.ServerPort,
			CountryCode:   n.CountryCode,
			CountrySource: n.CountrySource,
			Source:        n.Source,
			HasDetour:     n.HasDetour,
			Detour:        n.Detour,
		})
	}
	return out
}

func importResponse(nodes []*models.Node, sourceID int64, deduped int, warnings []string, fetches []subfetch.BatchItem) map[string]any {
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
	if len(fetches) > 0 {
		resp["fetches"] = fetches
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

func proxyOutboundsFromConfig(config string) ([]*merge.OrderedMap, error) {
	cfg, err := merge.ParseOrdered([]byte(config))
	if err != nil {
		return nil, fmt.Errorf("config is not valid JSON: %w", err)
	}
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return nil, errors.New("config has no outbounds")
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
	return proxies, nil
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
