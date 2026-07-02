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
	var req struct {
		Links string `json:"links"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	outbounds, err := sblink.ParseMany(req.Links)
	if err != nil {
		respondError(w, http.StatusBadRequest, "parse_error", err.Error())
		return
	}
	created := s.persistOutbounds(w, outbounds, "protocol", nil)
	if created == nil {
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"imported": len(created), "nodes": created})
}

// handleImportConfig extracts outbound nodes from an uploaded config or a
// template's content (requirement d.2). Group outbounds (selector/urltest/
// direct/block/dns) are skipped — only real proxy nodes are imported.
func (s *Server) handleImportConfig(w http.ResponseWriter, r *http.Request) {
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
	created := s.persistOutbounds(w, proxies, "config", nil)
	if created == nil {
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"imported": len(created), "nodes": created})
}

// handleImportSubscription creates a source, fetches it and imports nodes
// (requirements b, d). Nodes may be share-links or a base64 blob of links.
func (s *Server) handleImportSubscription(w http.ResponseWriter, r *http.Request) {
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
	sourceID, err := s.Store.CreateSource(req.Name, req.URL)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	nodes, ferr := s.fetchSourceNodes(sourceID, req.URL)
	if ferr != nil {
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"source_id": sourceID, "imported": len(nodes), "nodes": nodes})
}

// handleRefreshSource re-fetches a subscription source, replacing its nodes.
func (s *Server) handleRefreshSource(w http.ResponseWriter, r *http.Request) {
	id := pathID(r)
	src, err := s.Store.GetSource(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "not_found", "source not found")
		return
	}
	if err := s.Store.DeleteNodesBySource(id); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	nodes, ferr := s.fetchSourceNodes(id, src.URL)
	if ferr != nil {
		respondError(w, http.StatusBadGateway, "fetch_error", ferr.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"imported": len(nodes), "nodes": nodes})
}

// fetchSourceNodes fetches a subscription URL, parses links, persists nodes and
// records the fetch outcome on the source.
func (s *Server) fetchSourceNodes(sourceID int64, url string) ([]*models.Node, error) {
	ctx, cancel := contextWithTimeout(25 * time.Second)
	defer cancel()

	body, err := s.Fetcher.Fetch(ctx, url)
	if err != nil {
		_ = s.Store.UpdateSourceFetch(sourceID, "error: "+err.Error(), 0)
		return nil, err
	}
	outbounds, err := sblink.ParseMany(body)
	if err != nil {
		_ = s.Store.UpdateSourceFetch(sourceID, "error: "+err.Error(), 0)
		return nil, err
	}
	var created []*models.Node
	for _, ob := range outbounds {
		n, err := nodeFromOutbound(ob, "subscription", &sourceID)
		if err != nil {
			continue
		}
		id, err := s.Store.CreateNode(n)
		if err != nil {
			continue
		}
		n.ID = id
		created = append(created, n)
	}
	_ = s.Store.UpdateSourceFetch(sourceID, "ok", len(created))
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
		n, err := s.Store.GetNode(id)
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

// persistOutbounds saves a batch of parsed outbounds as nodes and returns them.
// On zero results (all failed) it writes a 400 and returns nil.
func (s *Server) persistOutbounds(w http.ResponseWriter, outbounds []*merge.OrderedMap, source string, sourceRef *int64) []*models.Node {
	var created []*models.Node
	for _, ob := range outbounds {
		n, err := nodeFromOutbound(ob, source, sourceRef)
		if err != nil {
			continue
		}
		id, err := s.Store.CreateNode(n)
		if err != nil {
			continue
		}
		n.ID = id
		created = append(created, n)
	}
	if len(created) == 0 {
		respondError(w, http.StatusBadRequest, "no_nodes", "no valid nodes were imported")
		return nil
	}
	return created
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
