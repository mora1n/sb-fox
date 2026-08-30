package api

import (
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/mora1n/sb-fox/internal/models"
)

func (s *Server) handleExportRuleSet(w http.ResponseWriter, r *http.Request) {
	item, ok := s.ruleSetForRequest(w, r)
	if !ok {
		return
	}
	format := chi.URLParam(r, "format")
	if format == "source" {
		serveRuleSetArtifact(w, r, item, "json")
		return
	}
	if format == "binary" {
		serveRuleSetArtifact(w, r, item, "srs")
		return
	}
	respondError(w, http.StatusBadRequest, "bad_request", "format must be source or binary")
}

func (s *Server) handlePublicRuleSet(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(chi.URLParam(r, "*"), "/")
	extension, name := ruleSetPath(path)
	if name == "" {
		http.NotFound(w, r)
		return
	}
	item, err := s.Store.GetRuleSetByNameAndToken(name, chi.URLParam(r, "token"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	serveRuleSetArtifact(w, r, item, extension)
}

func ruleSetPath(path string) (string, string) {
	for _, extension := range []string{"json", "srs"} {
		suffix := "." + extension
		if strings.HasSuffix(path, suffix) {
			return extension, strings.TrimSpace(strings.TrimSuffix(path, suffix))
		}
	}
	return "", ""
}

func serveRuleSetArtifact(w http.ResponseWriter, r *http.Request, item *models.RuleSet, extension string) {
	content, hash, contentType := item.PublishedJSON, item.JSONSHA256, "application/json; charset=utf-8"
	if extension == "srs" {
		content, hash, contentType = item.PublishedSRS, item.SRSSHA256, "application/octet-stream"
	}
	etag := `"` + hash + `"`
	w.Header().Set("ETag", etag)
	w.Header().Set("Last-Modified", item.PublishedAt.UTC().Format(http.TimeFormat))
	w.Header().Set("Cache-Control", "public, no-cache")
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", artifactDisposition(item.Name+"."+extension))
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func artifactDisposition(filename string) string {
	filename = strings.NewReplacer("\\", "-", "/", "-", `"`, "").Replace(strings.TrimSpace(filename))
	if filename == "" {
		filename = "rule-set"
	}
	if value := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); value != "" {
		return value
	}
	return "attachment"
}
