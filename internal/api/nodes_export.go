package api

import (
	"context"
	"net/http"
	"time"

	"github.com/mora1n/sb-fox/internal/merge"
)

// contextWithTimeout returns a background context with the given timeout.
func contextWithTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// handleExportNodeTemplate exports selected nodes as a node-template JSON
// (requirement h). Each outbound's server is annotated with a `#CC` suffix from
// the node's country_code so re-import identifies the country with precedence
// over the name. The result is an {"outbounds":[...]} document.
func (s *Server) handleExportNodeTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeIDs    []int64 `json:"node_ids"`
		TagCountry bool    `json:"tag_country"` // annotate server with #CC (default true)
	}
	// default TagCountry true
	req.TagCountry = true
	if !decodeJSON(w, r, &req) {
		return
	}

	ownerID, allOwners := ownerScope(r)
	nodes, err := s.getNodesForUser(req.NodeIDs, ownerID, allOwners)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if len(nodes) == 0 {
		respondError(w, http.StatusBadRequest, "no_nodes", "no nodes selected")
		return
	}

	outbounds := make([]any, 0, len(nodes))
	for _, n := range nodes {
		raw, err := merge.ParseOrdered([]byte(n.Raw))
		if err != nil {
			continue
		}
		if req.TagCountry && n.CountryCode != "" {
			server := raw.GetString("server")
			if server != "" && merge.ExtractServerCountryOverride(server) == nil {
				raw.Set("server", server+"#"+n.CountryCode)
			}
		}
		outbounds = append(outbounds, raw)
	}

	doc := merge.NewOrderedMap()
	doc.Set("outbounds", outbounds)
	compact, err := doc.MarshalJSON()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	pretty, _ := merge.Indent(compact)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="nodes-template.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pretty)
}
