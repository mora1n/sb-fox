package api

import (
	"net/http"

	"github.com/mora1n/sb-fox/internal/models"
)

// previewRequest generates a config from a template + node ids without saving.
type previewRequest struct {
	TemplateID      int64                 `json:"template_id"`
	TemplateContent string                `json:"template_content"` // optional inline override
	NodeIDs         []int64               `json:"node_ids"`
	Options         models.ProfileOptions `json:"options"`
}

// handlePreview renders a config.json in-memory (requirements a, c, f).
func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	var req previewRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	content := req.TemplateContent
	if content == "" {
		t, err := s.Store.GetTemplate(req.TemplateID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "bad_request", "template not found")
			return
		}
		content = t.Content
	}
	nodes, err := s.Store.GetNodes(req.NodeIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	order, err := s.countryHeatOrder()
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
		return
	}
	config, err := generateConfig(content, nodes, req.Options, order)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "generate_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"config": string(config)})
}

type validateRequest struct {
	Config    string `json:"config"`
	ProfileID int64  `json:"profile_id"`
}

// handleValidate runs the sing-box kernel check (advisory).
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	config, ok := s.resolveConfig(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, s.validateWithKernel(config))
}

// handleFormat runs the sing-box kernel format.
func (s *Server) handleFormat(w http.ResponseWriter, r *http.Request) {
	config, ok := s.resolveConfig(w, r)
	if !ok {
		return
	}
	respondJSON(w, http.StatusOK, s.Kernel.Format(config))
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
		config, err := s.renderProfile(req.ProfileID)
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
	nodes, err := s.Store.GetNodes(p.NodeIDs)
	if err != nil {
		return nil, err
	}
	order, err := s.countryHeatOrder()
	if err != nil {
		return nil, err
	}
	return generateConfig(t.Content, nodes, parseProfileOptions(p.Options), order)
}

// handleSubscription is the PUBLIC, unauthenticated endpoint returning a
// profile's config.json by token (requirement f). Anyone with the token gets
// the full config including server secrets — tokens are 128-bit random and can
// be rotated.
func (s *Server) handleSubscription(w http.ResponseWriter, r *http.Request, token string) {
	p, err := s.Store.GetProfileByToken(token)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	t, err := s.Store.GetTemplate(p.TemplateID)
	if err != nil {
		http.Error(w, "template missing", http.StatusInternalServerError)
		return
	}
	nodes, err := s.Store.GetNodes(p.NodeIDs)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	order, err := s.countryHeatOrder()
	if err != nil {
		http.Error(w, "generate error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	config, err := generateConfig(t.Content, nodes, parseProfileOptions(p.Options), order)
	if err != nil {
		http.Error(w, "generate error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="config.json"`)
	_, _ = w.Write(config)
}
