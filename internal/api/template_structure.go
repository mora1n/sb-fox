package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/store"
)

type templateStructure struct {
	Final  string                   `json:"final"`
	Groups []templateStructureGroup `json:"groups"`
}

type templateStructureGroup struct {
	Tag          string   `json:"tag"`
	Type         string   `json:"type"`
	Outbounds    []string `json:"outbounds"`
	Default      string   `json:"default,omitempty"`
	ReferencedBy []string `json:"referenced_by,omitempty"`
	Deletable    bool     `json:"deletable"`
	DeleteReason string   `json:"delete_reason,omitempty"`
}

func (s *Server) handleGetTemplateStructure(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	st, err := readTemplateStructure(t.Content)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, "invalid_template", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, st)
}

func (s *Server) handleUpdateTemplateStructure(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if t.Kind == "builtin" {
		respondError(w, http.StatusForbidden, "forbidden", "built-in templates are read-only")
		return
	}
	var req templateStructure
	if !decodeJSON(w, r, &req) {
		return
	}
	content, err := writeTemplateStructure(t.Content, req)
	if err != nil {
		respondError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := s.Store.UpdateTemplate(t.ID, content, t.Description); err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	st, err := readTemplateStructure(content)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, st)
}

func (s *Server) handleExportTemplate(w http.ResponseWriter, r *http.Request) {
	ownerID, allOwners := ownerScope(r)
	t, err := s.Store.GetTemplateForUser(pathID(r), ownerID, allOwners)
	if err == store.ErrNotFound {
		respondError(w, http.StatusNotFound, "not_found", "template not found")
		return
	}
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+attachmentName(t.Name)+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(t.Content))
}

func extractTemplateProxyOutbounds(content string) (string, []*merge.OrderedMap, error) {
	cfg, err := merge.ParseOrdered([]byte(content))
	if err != nil {
		return "", nil, err
	}
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return content, nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return "", nil, fmt.Errorf("outbounds must be an array")
	}

	var proxies []*merge.OrderedMap
	proxyTags := map[string]bool{}
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok {
			continue
		}
		if isProxyOutbound(om.GetString("type")) {
			proxies = append(proxies, om)
			if tag := om.GetString("tag"); tag != "" {
				proxyTags[tag] = true
			}
		}
	}
	if len(proxies) == 0 {
		return content, nil, nil
	}
	next := make([]any, 0, len(arr))
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok {
			next = append(next, ob)
			continue
		}
		if isProxyOutbound(om.GetString("type")) {
			continue
		}
		cleanTemplateGroupRefs(om, proxyTags)
		next = append(next, om)
	}
	cfg.Set("outbounds", next)
	processed, err := orderedString(cfg)
	return processed, proxies, err
}

func cleanTemplateGroupRefs(ob *merge.OrderedMap, removed map[string]bool) {
	if !isTemplateGroup(ob) || len(removed) == 0 {
		return
	}
	raw, ok := ob.Get("outbounds")
	if !ok {
		return
	}
	arr, ok := raw.([]any)
	if !ok {
		return
	}
	next := make([]any, 0, len(arr))
	for _, item := range arr {
		tag, ok := item.(string)
		if ok && removed[tag] {
			continue
		}
		next = append(next, item)
	}
	ob.Set("outbounds", next)
	if def := ob.GetString("default"); def != "" && removed[def] {
		ob.Delete("default")
	}
}

func readTemplateStructure(content string) (templateStructure, error) {
	cfg, err := merge.ParseOrdered([]byte(content))
	if err != nil {
		return templateStructure{}, err
	}
	groups, err := templateGroups(cfg)
	if err != nil {
		return templateStructure{}, err
	}
	st := templateStructure{Final: routeFinal(cfg), Groups: groups}
	refs := templateGroupRefs(st.Final, groups)
	for i := range st.Groups {
		g := &st.Groups[i]
		g.ReferencedBy = refs[g.Tag]
		g.Deletable = len(g.ReferencedBy) == 0
		if !g.Deletable {
			g.DeleteReason = "group is still referenced"
		}
	}
	return st, nil
}

func writeTemplateStructure(content string, st templateStructure) (string, error) {
	cfg, err := merge.ParseOrdered([]byte(content))
	if err != nil {
		return "", err
	}
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return "", fmt.Errorf("template has no outbounds")
	}
	arr, ok := raw.([]any)
	if !ok {
		return "", fmt.Errorf("outbounds must be an array")
	}

	existingGroups, staticTags, firstGroupPos := classifyTemplateOutbounds(arr)
	desired, err := validateTemplateStructure(st, staticTags)
	if err != nil {
		return "", err
	}

	groupObjects := make([]any, 0, len(st.Groups))
	for _, g := range desired {
		om := existingGroups[g.Tag]
		if om == nil {
			om = merge.NewOrderedMap()
		}
		om.Set("type", g.Type)
		om.Set("tag", g.Tag)
		om.Set("outbounds", stringSliceToAny(g.Outbounds))
		if g.Default == "" {
			om.Delete("default")
		} else {
			om.Set("default", g.Default)
		}
		groupObjects = append(groupObjects, om)
	}

	next := mergeTemplateGroupOrder(arr, groupObjects, firstGroupPos)
	cfg.Set("outbounds", next)
	setRouteFinal(cfg, strings.TrimSpace(st.Final))
	return orderedString(cfg)
}

func templateGroups(cfg *merge.OrderedMap) ([]templateStructureGroup, error) {
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("outbounds must be an array")
	}
	var groups []templateStructureGroup
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok || !isTemplateGroup(om) {
			continue
		}
		groups = append(groups, templateStructureGroup{
			Tag:       om.GetString("tag"),
			Type:      om.GetString("type"),
			Outbounds: outboundStringList(om),
			Default:   om.GetString("default"),
		})
	}
	return groups, nil
}

func classifyTemplateOutbounds(arr []any) (map[string]*merge.OrderedMap, map[string]bool, int) {
	groups := map[string]*merge.OrderedMap{}
	staticTags := map[string]bool{}
	firstGroupPos := -1
	nonGroupCount := 0
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok {
			nonGroupCount++
			continue
		}
		if isTemplateGroup(om) {
			if firstGroupPos == -1 {
				firstGroupPos = nonGroupCount
			}
			groups[om.GetString("tag")] = om
			continue
		}
		if tag := om.GetString("tag"); tag != "" {
			staticTags[tag] = true
		}
		nonGroupCount++
	}
	return groups, staticTags, firstGroupPos
}

func validateTemplateStructure(st templateStructure, staticTags map[string]bool) ([]templateStructureGroup, error) {
	final := strings.TrimSpace(st.Final)
	if final == "" {
		return nil, fmt.Errorf("final selector is required")
	}
	seen := map[string]bool{}
	groupTags := map[string]bool{}
	desired := make([]templateStructureGroup, 0, len(st.Groups))
	for _, g := range st.Groups {
		g.Tag = strings.TrimSpace(g.Tag)
		g.Type = strings.TrimSpace(g.Type)
		g.Default = strings.TrimSpace(g.Default)
		if g.Tag == "" {
			return nil, fmt.Errorf("group tag is required")
		}
		if g.Type != "selector" && g.Type != "urltest" {
			return nil, fmt.Errorf("group %q has unsupported type %q", g.Tag, g.Type)
		}
		if seen[g.Tag] {
			return nil, fmt.Errorf("duplicate group tag %q", g.Tag)
		}
		seen[g.Tag] = true
		groupTags[g.Tag] = true
		desired = append(desired, g)
	}
	if !groupTags[final] {
		return nil, fmt.Errorf("final selector %q does not exist", final)
	}

	allowed := map[string]bool{}
	for tag := range staticTags {
		allowed[tag] = true
	}
	for tag := range groupTags {
		allowed[tag] = true
	}
	for i := range desired {
		outs, err := normalizeOutboundRefs(desired[i].Tag, desired[i].Outbounds, allowed)
		if err != nil {
			return nil, err
		}
		desired[i].Outbounds = outs
		if desired[i].Default != "" && !containsString(outs, desired[i].Default) {
			return nil, fmt.Errorf("default %q is not in group %q outbounds", desired[i].Default, desired[i].Tag)
		}
	}
	return desired, nil
}

func normalizeOutboundRefs(groupTag string, refs []string, allowed map[string]bool) ([]string, error) {
	out := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return nil, fmt.Errorf("group %q contains an empty outbound", groupTag)
		}
		if ref == groupTag {
			return nil, fmt.Errorf("group %q cannot reference itself", groupTag)
		}
		if !allowed[ref] {
			return nil, fmt.Errorf("group %q references unknown outbound %q", groupTag, ref)
		}
		if seen[ref] {
			return nil, fmt.Errorf("group %q has duplicate outbound %q", groupTag, ref)
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out, nil
}

func mergeTemplateGroupOrder(arr []any, groupObjects []any, firstGroupPos int) []any {
	nonGroups := make([]any, 0, len(arr))
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if ok && isTemplateGroup(om) {
			continue
		}
		nonGroups = append(nonGroups, ob)
	}
	if firstGroupPos < 0 || firstGroupPos > len(nonGroups) {
		firstGroupPos = directInsertPos(nonGroups)
	}
	next := make([]any, 0, len(nonGroups)+len(groupObjects))
	next = append(next, nonGroups[:firstGroupPos]...)
	next = append(next, groupObjects...)
	next = append(next, nonGroups[firstGroupPos:]...)
	return next
}

func directInsertPos(outbounds []any) int {
	for i, ob := range outbounds {
		om, ok := ob.(*merge.OrderedMap)
		if ok && strings.EqualFold(om.GetString("tag"), "Direct") {
			return i
		}
	}
	return len(outbounds)
}

func templateGroupRefs(final string, groups []templateStructureGroup) map[string][]string {
	refs := map[string][]string{}
	if final != "" {
		refs[final] = append(refs[final], "route.final")
	}
	for _, g := range groups {
		for _, ob := range g.Outbounds {
			if ob != "" && ob != g.Tag {
				refs[ob] = append(refs[ob], g.Tag)
			}
		}
		if g.Default != "" && g.Default != g.Tag {
			refs[g.Default] = append(refs[g.Default], g.Tag+" default")
		}
	}
	return refs
}

func isTemplateGroup(ob *merge.OrderedMap) bool {
	typ := ob.GetString("type")
	return typ == "selector" || typ == "urltest"
}

func outboundStringList(ob *merge.OrderedMap) []string {
	raw, ok := ob.Get("outbounds")
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func routeFinal(cfg *merge.OrderedMap) string {
	raw, ok := cfg.Get("route")
	if !ok {
		return ""
	}
	route, ok := raw.(*merge.OrderedMap)
	if !ok {
		return ""
	}
	return route.GetString("final")
}

func setRouteFinal(cfg *merge.OrderedMap, final string) {
	raw, ok := cfg.Get("route")
	route, ok := raw.(*merge.OrderedMap)
	if !ok {
		route = merge.NewOrderedMap()
		cfg.Set("route", route)
	}
	route.Set("final", final)
}

func stringSliceToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func orderedString(cfg *merge.OrderedMap) (string, error) {
	compact, err := cfg.MarshalJSON()
	if err != nil {
		return "", err
	}
	pretty, err := merge.Indent(compact)
	if err != nil {
		return "", err
	}
	return string(pretty), nil
}
