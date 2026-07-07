package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/store"
)

type templateStructure struct {
	Final              string                   `json:"final"`
	Groups             []templateStructureGroup `json:"groups"`
	AvailableOutbounds []string                 `json:"available_outbounds"`
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
	if err := s.Store.UpdateTemplateForUser(t.ID, t.OwnerUserID, content, t.Description); err != nil {
		if err == store.ErrNotFound {
			respondError(w, http.StatusNotFound, "not_found", "template not found")
			return
		}
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
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return templateStructure{}, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return templateStructure{}, fmt.Errorf("outbounds must be an array")
	}
	allGroups, err := templateGroups(arr)
	if err != nil {
		return templateStructure{}, err
	}
	refs := routeGroupRefs(cfg, allGroups)
	st := templateStructure{
		Final:              routeFinal(cfg),
		Groups:             allGroups,
		AvailableOutbounds: availableTemplateOutbounds(arr, allGroups),
	}
	for i := range st.Groups {
		g := &st.Groups[i]
		g.ReferencedBy = refs[g.Tag]
		g.Deletable = false
		g.DeleteReason = "managed outbound groups cannot be deleted"
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
	allGroups, err := templateGroups(arr)
	if err != nil {
		return "", err
	}
	for _, g := range allGroups {
		staticTags[g.Tag] = true
	}
	desired, err := validateTemplateStructure(st, staticTags)
	if err != nil {
		return "", err
	}
	if err := validateTemplateGroupSet(desired, allGroups); err != nil {
		return "", err
	}

	groupObjects := make([]any, 0, len(st.Groups))
	managedTags := make(map[string]bool, len(desired))
	for _, g := range desired {
		om := existingGroups[g.Tag]
		if om == nil {
			om = merge.NewOrderedMap()
		}
		managedTags[g.Tag] = true
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
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok || !isTemplateGroup(om) || managedTags[om.GetString("tag")] {
			continue
		}
		groupObjects = append(groupObjects, om)
	}

	next := mergeTemplateGroupOrder(arr, groupObjects, firstGroupPos)
	cfg.Set("outbounds", next)
	setRouteFinal(cfg, strings.TrimSpace(st.Final))
	return orderedString(cfg)
}

func templateGroups(arr []any) ([]templateStructureGroup, error) {
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

func routeGroupRefs(cfg *merge.OrderedMap, groups []templateStructureGroup) map[string][]string {
	groupByTag := make(map[string]templateStructureGroup, len(groups))
	for _, g := range groups {
		groupByTag[g.Tag] = g
	}
	refs := map[string][]string{}
	addRouteDirectGroupRefs(cfg, groupByTag, refs)
	addNestedGroupRefs(groups, groupByTag, refs)
	return refs
}

func addRouteDirectGroupRefs(cfg *merge.OrderedMap, groupByTag map[string]templateStructureGroup, refs map[string][]string) {
	if final := routeFinal(cfg); final != "" {
		if _, ok := groupByTag[final]; ok {
			appendGroupRef(refs, final, "route.final")
		}
	}
	raw, ok := cfg.Get("route")
	if !ok {
		return
	}
	route, ok := raw.(*merge.OrderedMap)
	if !ok {
		return
	}
	rawRules, ok := route.Get("rules")
	if !ok {
		return
	}
	rules, ok := rawRules.([]any)
	if !ok {
		return
	}
	for i, item := range rules {
		rule, ok := item.(*merge.OrderedMap)
		if !ok {
			continue
		}
		outbound := rule.GetString("outbound")
		if _, ok := groupByTag[outbound]; ok && outbound != "" {
			appendGroupRef(refs, outbound, fmt.Sprintf("route.rules[%d].outbound", i))
		}
	}
}

func addNestedGroupRefs(groups []templateStructureGroup, groupByTag map[string]templateStructureGroup, refs map[string][]string) {
	visited := map[string]bool{}
	var visit func(string)
	visit = func(tag string) {
		if visited[tag] {
			return
		}
		visited[tag] = true
		g, ok := groupByTag[tag]
		if !ok {
			return
		}
		for _, outbound := range g.Outbounds {
			outbound = strings.TrimSpace(outbound)
			if outbound == "" || outbound == g.Tag {
				continue
			}
			if _, ok := groupByTag[outbound]; !ok {
				continue
			}
			appendGroupRef(refs, outbound, g.Tag)
			visit(outbound)
		}
		if def := strings.TrimSpace(g.Default); def != "" && def != g.Tag {
			if _, ok := groupByTag[def]; ok {
				appendGroupRef(refs, def, g.Tag+" default")
				visit(def)
			}
		}
	}
	for _, g := range groups {
		if len(refs[g.Tag]) > 0 {
			visit(g.Tag)
		}
	}
}

func appendGroupRef(refs map[string][]string, tag, ref string) {
	if tag == "" || ref == "" {
		return
	}
	for _, existing := range refs[tag] {
		if existing == ref {
			return
		}
	}
	refs[tag] = append(refs[tag], ref)
}

func availableTemplateOutbounds(arr []any, groups []templateStructureGroup) []string {
	seen := map[string]bool{}
	var out []string
	add := func(tag string) {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			return
		}
		seen[tag] = true
		out = append(out, tag)
	}
	for _, ob := range arr {
		om, ok := ob.(*merge.OrderedMap)
		if !ok || isTemplateGroup(om) {
			continue
		}
		add(om.GetString("tag"))
	}
	for _, g := range groups {
		add(g.Tag)
	}
	return out
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
	allowed := map[string]bool{}
	for tag := range staticTags {
		allowed[tag] = true
	}
	for tag := range groupTags {
		allowed[tag] = true
	}
	if final != "" && !allowed[final] {
		return nil, fmt.Errorf("final outbound %q does not exist", final)
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
	if err := validateTemplateGroupCycles(desired); err != nil {
		return nil, err
	}
	return desired, nil
}

func validateTemplateGroupSet(desired []templateStructureGroup, existing []templateStructureGroup) error {
	desiredTags := map[string]bool{}
	for _, g := range desired {
		desiredTags[g.Tag] = true
	}
	existingTags := map[string]bool{}
	for _, g := range existing {
		existingTags[g.Tag] = true
	}
	for tag := range existingTags {
		if !desiredTags[tag] {
			return fmt.Errorf("template outbound group %q cannot be deleted", tag)
		}
	}
	for tag := range desiredTags {
		if !existingTags[tag] {
			return fmt.Errorf("template outbound group %q does not exist", tag)
		}
	}
	return nil
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
			return nil, newGenerationError(
				fmt.Sprintf("group %q cannot reference itself", groupTag),
				generationErrorDetails{Kind: generateErrGroupCycle, Panel: "group", GroupTag: groupTag, Cycle: []string{groupTag, groupTag}},
			)
		}
		if !allowed[ref] {
			return nil, newGenerationError(
				fmt.Sprintf("group %q references unknown outbound %q", groupTag, ref),
				generationErrorDetails{Kind: generateErrUnknownOutboundRef, Panel: "group", GroupTag: groupTag, OutboundTag: ref},
			)
		}
		if seen[ref] {
			return nil, fmt.Errorf("group %q has duplicate outbound %q", groupTag, ref)
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out, nil
}

func validateTemplateGroupCycles(groups []templateStructureGroup) error {
	groupTags := make(map[string]bool, len(groups))
	for _, g := range groups {
		groupTags[g.Tag] = true
	}
	graph := make(map[string][]string, len(groups))
	for _, g := range groups {
		for _, ref := range g.Outbounds {
			if groupTags[ref] {
				graph[g.Tag] = append(graph[g.Tag], ref)
			}
		}
	}

	visiting := map[string]int{}
	visited := map[string]bool{}
	var stack []string
	var visit func(string) error
	visit = func(tag string) error {
		if i, ok := visiting[tag]; ok {
			cycle := append(append([]string{}, stack[i:]...), tag)
			return newGenerationError(
				fmt.Sprintf("group reference cycle: %s", strings.Join(cycle, " -> ")),
				generationErrorDetails{Kind: generateErrGroupCycle, Panel: "group", GroupTag: tag, Cycle: cycle},
			)
		}
		if visited[tag] {
			return nil
		}
		visiting[tag] = len(stack)
		stack = append(stack, tag)
		for _, next := range graph[tag] {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		delete(visiting, tag)
		visited[tag] = true
		return nil
	}
	for _, g := range groups {
		if err := visit(g.Tag); err != nil {
			return err
		}
	}
	return nil
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
	if final == "" {
		if ok {
			route.Delete("final")
		}
		return
	}
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
