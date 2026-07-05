package api

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
)

// generateConfig renders a final config.json from a template and node set using
// the merge engine. templateContent is the raw template JSON; nodes are DB rows
// whose Raw blobs are the authoritative outbounds.
func generateConfig(templateContent string, nodes []*models.Node, opts models.ProfileOptions, countryHeatOrder []string) ([]byte, error) {
	cfg, err := merge.ParseOrdered([]byte(templateContent))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	mergeNodes, err := toMergeNodes(nodes)
	if err != nil {
		return nil, err
	}
	if err := applyChainProxy(nodes, mergeNodes, opts); err != nil {
		return nil, err
	}
	chainSelectorTags, err := chainProxySelectorTags(nodes, mergeNodes, opts)
	if err != nil {
		return nil, err
	}

	out, err := merge.Generate(cfg, mergeNodes, merge.Options{
		AutoCountryGroups:   opts.AutoCountryGroups,
		CountryHeatOrder:    countryHeatOrder,
		ChainProxy:          opts.ChainProxy,
		ChainProxyOutbounds: chainSelectorTags,
	})
	if err != nil {
		return nil, err
	}
	if err := validateRouteFinalOutbound(out); err != nil {
		return nil, err
	}
	if err := validateGeneratedGroupCycles(out); err != nil {
		return nil, err
	}

	compact, err := out.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return merge.Indent(compact)
}

func generateConfigWithGroupSelections(templateContent string, groupNodes map[string][]*models.Node, autoCountryNodes, chainNodes []*models.Node, opts models.ProfileOptions, countryHeatOrder []string) ([]byte, error) {
	st, err := readTemplateStructure(templateContent)
	if err != nil {
		return nil, fmt.Errorf("read template groups: %w", err)
	}
	opts = ensureGroupSelectionsFromNodes(opts, groupNodes)
	opts = applyDefaultOutboundRefs(st, opts)
	if err := validateGroupSelectionRefs(st, opts); err != nil {
		return nil, err
	}
	if err := validateGroupSelectionCycles(st, opts); err != nil {
		return nil, err
	}
	if err := validateRequiredGroupSelections(st, opts); err != nil {
		return nil, err
	}
	validGroups := map[string]bool{}
	for _, g := range st.Groups {
		validGroups[g.Tag] = true
	}
	for tag := range opts.GroupSelections {
		if !validGroups[tag] {
			return nil, newGenerationError(
				fmt.Sprintf("group selection references unknown outbound group %q", tag),
				generationErrorDetails{Kind: generateErrUnknownOutboundRef, Panel: "group", GroupTag: tag},
			)
		}
	}

	cfg, err := merge.ParseOrdered([]byte(templateContent))
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	templateOutbounds, err := generatedOutbounds(cfg)
	if err != nil {
		return nil, err
	}
	templateGroupTags, err := templateGroupTagSet(templateOutbounds)
	if err != nil {
		return nil, err
	}

	if opts.AutoCountryGroups && len(autoCountryNodes) == 0 {
		return nil, newGenerationError(
			"auto country group nodes are required",
			generationErrorDetails{Kind: generateErrAutoCountryEmpty, Panel: "country"},
		)
	}

	allNodes := uniqueNodes(groupNodes, autoCountryNodes, chainNodes)
	mergeNodes, err := toMergeNodes(allNodes)
	if err != nil {
		return nil, err
	}
	chainIDs := nodeIDs(chainNodes)
	if opts.ChainProxy {
		if len(chainIDs) == 0 {
			return nil, newGenerationError(
				"chain proxy nodes are required",
				generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
			)
		}
		chainOpts := opts
		chainOpts.ChainProxyNodeIDs = chainIDs
		if err := applyChainProxy(allNodes, mergeNodes, chainOpts); err != nil {
			return nil, err
		}
	}

	out, err := merge.Generate(cfg, mergeNodes, merge.Options{
		AutoCountryGroups:      opts.AutoCountryGroups,
		CountryHeatOrder:       countryHeatOrder,
		CountryGroupSourceTags: countrySourceTags(opts, autoCountryNodes),
	})
	if err != nil {
		return nil, err
	}

	outbounds, err := generatedOutbounds(out)
	if err != nil {
		return nil, err
	}

	chainTags := nodeTags(chainNodes)
	if opts.ChainProxy {
		upstreamTags := upstreamNodeTags(groupNodes, chainIDs)
		if len(upstreamTags) == 0 {
			return nil, newGenerationError(
				"chain proxy selector has no upstream nodes",
				generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
			)
		}
		outbounds = append(outbounds, chainSelector(upstreamTags))
		out.Set("outbounds", outbounds)
	}

	for _, g := range st.Groups {
		sel := opts.GroupSelections[g.Tag]
		configured := selectionHasInputs(sel) || autoCountryFillsSelection(sel, opts)
		if opts.ChainProxy && g.Tag == st.Final && len(chainTags) > 0 {
			configured = true
		}
		if !configured {
			continue
		}

		group := findOutboundByTag(outbounds, g.Tag)
		if group == nil {
			return nil, fmt.Errorf("template outbound group %q is missing", g.Tag)
		}
		tags := selectableTagsForSelection(sel, groupNodes[g.Tag], autoCountryNodes, outbounds, templateGroupTags, opts)
		if opts.ChainProxy {
			tags = appendUniqueStrings(tags, chainTags)
		}
		if g.Tag == st.Final && len(tags) == 0 {
			return nil, newGenerationError(
				fmt.Sprintf("final selector %q has no selected nodes", g.Tag),
				generationErrorDetails{Kind: generateErrInvalidFinal, Panel: "group", GroupTag: g.Tag},
			)
		}
		group.Set("outbounds", stringSliceToAny(tags))
		if def := group.GetString("default"); def != "" && !containsString(tags, def) {
			group.Delete("default")
		}
	}
	if err := validateRouteFinalOutbound(out); err != nil {
		return nil, err
	}
	if err := validateGeneratedGroupCycles(out); err != nil {
		return nil, err
	}

	compact, err := out.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return merge.Indent(compact)
}

func countrySourceTags(opts models.ProfileOptions, nodes []*models.Node) []string {
	if !opts.AutoCountryGroups {
		return nil
	}
	return nodeTags(nodes)
}

func ensureGroupSelectionsFromNodes(opts models.ProfileOptions, groupNodes map[string][]*models.Node) models.ProfileOptions {
	if len(opts.GroupSelections) > 0 {
		return opts
	}
	selections := make(map[string]models.NodeSelection, len(groupNodes))
	for tag, nodes := range groupNodes {
		if len(nodes) == 0 {
			continue
		}
		selections[tag] = models.NodeSelection{NodeIDs: nodeIDs(nodes)}
	}
	opts.GroupSelections = normalizeGroupSelections(selections)
	return opts
}

func validateOptionOutboundRefs(templateContent string, opts models.ProfileOptions) error {
	if len(opts.GroupSelections) == 0 {
		return nil
	}
	st, err := readTemplateStructure(templateContent)
	if err != nil {
		return fmt.Errorf("read template groups: %w", err)
	}
	return validateGroupSelectionRefs(st, opts)
}

func validateOptionGroupInputs(templateContent string, opts models.ProfileOptions) error {
	if len(opts.GroupSelections) == 0 {
		return nil
	}
	st, err := readTemplateStructure(templateContent)
	if err != nil {
		return fmt.Errorf("read template groups: %w", err)
	}
	opts = applyDefaultOutboundRefs(st, opts)
	if err := validateGroupSelectionCycles(st, opts); err != nil {
		return err
	}
	return validateRequiredGroupSelections(st, opts)
}

func validateGroupSelectionRefs(st templateStructure, opts models.ProfileOptions) error {
	groups := make(map[string]templateStructureGroup, len(st.Groups))
	for _, g := range st.Groups {
		groups[g.Tag] = g
	}
	for tag, sel := range opts.GroupSelections {
		g, ok := groups[tag]
		if !ok {
			return newGenerationError(
				fmt.Sprintf("group selection references unknown outbound group %q", tag),
				generationErrorDetails{Kind: generateErrUnknownOutboundRef, Panel: "group", GroupTag: tag},
			)
		}
		allowed := make(map[string]bool, len(g.Outbounds))
		for _, outbound := range g.Outbounds {
			allowed[outbound] = true
		}
		if _, err := normalizeOutboundRefs(tag, sel.OutboundRefs, allowed); err != nil {
			return err
		}
	}
	return nil
}

func validateGroupSelectionCycles(st templateStructure, opts models.ProfileOptions) error {
	structureGroups := make([]templateStructureGroup, 0, len(st.Groups))
	for _, g := range st.Groups {
		sel := opts.GroupSelections[g.Tag]
		structureGroups = append(structureGroups, templateStructureGroup{
			Tag:       g.Tag,
			Type:      g.Type,
			Outbounds: sel.OutboundRefs,
		})
	}
	return validateTemplateGroupCycles(structureGroups)
}

func applyDefaultOutboundRefs(st templateStructure, opts models.ProfileOptions) models.ProfileOptions {
	if opts.GroupSelections == nil {
		opts.GroupSelections = map[string]models.NodeSelection{}
	}
	for _, g := range st.Groups {
		sel := opts.GroupSelections[g.Tag]
		if selectionHasInputs(sel) {
			continue
		}
		sel.OutboundRefs = defaultOutboundRefs(g)
		opts.GroupSelections[g.Tag] = sel
	}
	opts.GroupSelections = normalizeGroupSelections(opts.GroupSelections)
	return opts
}

func defaultOutboundRefs(g templateStructureGroup) []string {
	out := make([]string, 0, len(g.Outbounds))
	for _, ref := range g.Outbounds {
		ref = strings.TrimSpace(ref)
		if ref != "" && ref != g.Tag {
			out = append(out, ref)
		}
	}
	return normalizeOutboundRefList(out)
}

func validateRequiredGroupSelections(st templateStructure, opts models.ProfileOptions) error {
	for _, g := range st.Groups {
		sel := opts.GroupSelections[g.Tag]
		if selectionHasInputs(sel) {
			continue
		}
		if autoCountryFillsSelection(sel, opts) {
			continue
		}
		if opts.ChainProxy && g.Tag == st.Final && opts.ChainProxySelected != nil && selectionHasInputs(*opts.ChainProxySelected) {
			continue
		}
		if g.Tag == st.Final {
			return newGenerationError(
				fmt.Sprintf("final selector %q has no selected nodes", g.Tag),
				generationErrorDetails{Kind: generateErrInvalidFinal, Panel: "group", GroupTag: g.Tag},
			)
		}
		return newGenerationError(
			fmt.Sprintf("outbound group %q has no selected nodes or references", g.Tag),
			generationErrorDetails{Kind: generateErrEmptyGroup, Panel: "group", GroupTag: g.Tag},
		)
	}
	return nil
}

func templateGroupTagSet(outbounds []any) (map[string]bool, error) {
	groups, err := templateGroups(outbounds)
	if err != nil {
		return nil, err
	}
	tags := make(map[string]bool, len(groups))
	for _, g := range groups {
		tags[g.Tag] = true
	}
	return tags, nil
}

func selectionHasInputs(sel models.NodeSelection) bool {
	return len(sel.NodeIDs) > 0 || len(sel.NodeGroupIDs) > 0 || len(sel.OutboundRefs) > 0
}

func autoCountryFillsSelection(sel models.NodeSelection, opts models.ProfileOptions) bool {
	return opts.AutoCountryGroups && opts.AutoCountrySelected != nil && selectionHasInputs(*opts.AutoCountrySelected) && !sel.SkipCountryGroups
}

func selectableTagsForSelection(sel models.NodeSelection, nodes, autoCountryNodes []*models.Node, outbounds []any, templateGroupTags map[string]bool, opts models.ProfileOptions) []string {
	tags := appendUniqueStrings(nil, sel.OutboundRefs)
	if opts.AutoCountryGroups && !sel.SkipCountryGroups {
		countryNodes := nodes
		if len(countryNodes) == 0 {
			countryNodes = autoCountryNodes
		}
		return appendUniqueStrings(tags, countryCandidateTags(outbounds, countryNodes, templateGroupTags))
	}
	return appendUniqueStrings(tags, nodeTags(nodes))
}

func countryCandidateTags(outbounds []any, nodes []*models.Node, templateGroupTags map[string]bool) []string {
	nodeTags := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		if n != nil && n.Tag != "" {
			nodeTags[n.Tag] = true
		}
	}
	if len(nodeTags) == 0 {
		return nil
	}

	covered := map[string]bool{}
	var out []string
	for _, ob := range outbounds {
		group, ok := ob.(*merge.OrderedMap)
		if !ok {
			continue
		}
		tag := group.GetString("tag")
		if tag == "" || templateGroupTags[tag] || tag == merge.ChainProxyTag {
			continue
		}
		typ := group.GetString("type")
		if typ != "selector" && typ != "urltest" {
			continue
		}
		hasSelectedNode := false
		for _, outbound := range outboundStringList(group) {
			if nodeTags[outbound] {
				hasSelectedNode = true
				covered[outbound] = true
			}
		}
		if hasSelectedNode {
			out = appendUniqueStrings(out, []string{tag})
		}
	}
	for _, n := range nodes {
		if n != nil && n.Tag != "" && !covered[n.Tag] {
			out = appendUniqueStrings(out, []string{n.Tag})
		}
	}
	return out
}

func validateRouteFinalOutbound(cfg *merge.OrderedMap) error {
	final := strings.TrimSpace(routeFinal(cfg))
	if final == "" {
		return nil
	}
	outbounds, err := generatedOutbounds(cfg)
	if err != nil {
		return err
	}
	outbound := findOutboundByTag(outbounds, final)
	if outbound == nil {
		return newGenerationError(
			fmt.Sprintf("final outbound %q is missing", final),
			generationErrorDetails{Kind: generateErrInvalidFinal, Panel: "group", GroupTag: final, OutboundTag: final},
		)
	}
	if isTemplateGroup(outbound) && len(outboundStringList(outbound)) == 0 {
		return newGenerationError(
			fmt.Sprintf("final selector %q has no selected nodes", final),
			generationErrorDetails{Kind: generateErrInvalidFinal, Panel: "group", GroupTag: final},
		)
	}
	return nil
}

func validateGeneratedGroupCycles(cfg *merge.OrderedMap) error {
	outbounds, err := generatedOutbounds(cfg)
	if err != nil {
		return err
	}
	groups, err := templateGroups(outbounds)
	if err != nil {
		return err
	}
	return validateTemplateGroupCycles(groups)
}

func generatedOutbounds(cfg *merge.OrderedMap) ([]any, error) {
	raw, ok := cfg.Get("outbounds")
	if !ok {
		return nil, fmt.Errorf("template has no outbounds")
	}
	outbounds, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("template outbounds is not an array")
	}
	return outbounds, nil
}

func uniqueNodes(groupNodes map[string][]*models.Node, extraGroups ...[]*models.Node) []*models.Node {
	var out []*models.Node
	seen := map[int64]bool{}
	add := func(nodes []*models.Node) {
		for _, n := range nodes {
			if n == nil || seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	keys := make([]string, 0, len(groupNodes))
	for key := range groupNodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		add(groupNodes[key])
	}
	for _, nodes := range extraGroups {
		add(nodes)
	}
	return out
}

func nodeIDs(nodes []*models.Node) []int64 {
	out := make([]int64, 0, len(nodes))
	seen := map[int64]bool{}
	for _, n := range nodes {
		if n == nil || seen[n.ID] {
			continue
		}
		seen[n.ID] = true
		out = append(out, n.ID)
	}
	return out
}

func nodeTags(nodes []*models.Node) []string {
	out := make([]string, 0, len(nodes))
	seen := map[string]bool{}
	for _, n := range nodes {
		if n == nil || n.Tag == "" || seen[n.Tag] {
			continue
		}
		seen[n.Tag] = true
		out = append(out, n.Tag)
	}
	return out
}

func upstreamNodeTags(groupNodes map[string][]*models.Node, chainIDs []int64) []string {
	chainSet := make(map[int64]bool, len(chainIDs))
	for _, id := range chainIDs {
		chainSet[id] = true
	}
	var out []string
	seen := map[string]bool{}
	keys := make([]string, 0, len(groupNodes))
	for key := range groupNodes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		nodes := groupNodes[key]
		for _, n := range nodes {
			if n == nil || chainSet[n.ID] || n.Tag == "" || seen[n.Tag] {
				continue
			}
			seen[n.Tag] = true
			out = append(out, n.Tag)
		}
	}
	return out
}

func chainSelector(tags []string) *merge.OrderedMap {
	sel := merge.NewOrderedMap()
	sel.Set("type", "selector")
	sel.Set("tag", merge.ChainProxyTag)
	sel.Set("outbounds", stringSliceToAny(tags))
	return sel
}

func findOutboundByTag(outbounds []any, tag string) *merge.OrderedMap {
	for _, ob := range outbounds {
		om, ok := ob.(*merge.OrderedMap)
		if ok && om.GetString("tag") == tag {
			return om
		}
	}
	return nil
}

func appendUniqueStrings(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, v := range base {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	for _, v := range extra {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func applyChainProxy(nodes []*models.Node, mergeNodes []*merge.Node, opts models.ProfileOptions) error {
	if !opts.ChainProxy {
		return nil
	}
	chainIDs := normalizedChainProxyNodeIDs(opts)
	if len(chainIDs) == 0 {
		return newGenerationError(
			"chain proxy nodes are required",
			generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
		)
	}
	chainSet := make(map[int64]bool, len(chainIDs))
	for _, id := range chainIDs {
		chainSet[id] = true
	}
	found := 0
	for i, n := range nodes {
		if chainSet[n.ID] {
			mergeNodes[i].Raw.Set("detour", merge.ChainProxyTag)
			found++
		}
	}
	if found != len(chainSet) {
		return newGenerationError(
			"chain proxy node not found",
			generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
		)
	}
	return nil
}

func chainProxySelectorTags(nodes []*models.Node, mergeNodes []*merge.Node, opts models.ProfileOptions) ([]string, error) {
	if !opts.ChainProxy {
		return nil, nil
	}
	chainIDs := normalizedChainProxyNodeIDs(opts)
	chainSet := make(map[int64]bool, len(chainIDs))
	for _, id := range chainIDs {
		chainSet[id] = true
	}
	tags := make([]string, 0, len(nodes)-len(chainSet))
	for i, n := range nodes {
		if chainSet[n.ID] {
			continue
		}
		tag := mergeNodes[i].Raw.GetString("tag")
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return nil, newGenerationError(
			"chain proxy selector has no upstream nodes",
			generationErrorDetails{Kind: generateErrChainProxyEmpty, Panel: "chain"},
		)
	}
	return tags, nil
}

func normalizedChainProxyNodeIDs(opts models.ProfileOptions) []int64 {
	ids := opts.ChainProxyNodeIDs
	if len(ids) == 0 && opts.ChainProxyNodeID != 0 {
		ids = []int64{opts.ChainProxyNodeID}
	}
	return uniqueInt64s(ids)
}

// toMergeNodes converts DB node rows into merge.Node values. A node whose
// stored metadata has a country_code carries it as an explicit override so
// manual, auto-detected and imported country attributes are honored during
// generation.
func toMergeNodes(nodes []*models.Node) ([]*merge.Node, error) {
	out := make([]*merge.Node, 0, len(nodes))
	for _, n := range nodes {
		raw, err := merge.ParseOrdered([]byte(n.Raw))
		if err != nil {
			return nil, fmt.Errorf("node %d (%s) has invalid raw JSON: %w", n.ID, n.Tag, err)
		}
		mn := &merge.Node{Raw: raw, Source: mergeSource(n.Source)}
		if code := strings.TrimSpace(n.CountryCode); code != "" {
			mn.CountryOverride = code
		}
		out = append(out, mn)
	}
	return out, nil
}

// mergeSource maps a stored node source to the merge engine's source vocabulary.
// The merge engine only distinguishes "protocol"/"subscription" (both make a
// detour-less node eligible for the Relay group); config/manual nodes map to
// "protocol" so they participate in relay grouping identically.
func mergeSource(source string) string {
	switch source {
	case "subscription":
		return "subscription"
	default:
		return "protocol"
	}
}

// parseProfileOptions parses a profile's options JSON blob, defaulting
// AutoCountryGroups to true when unset.
func parseProfileOptions(blob string) models.ProfileOptions {
	opts := models.ProfileOptions{AutoCountryGroups: true}
	if blob == "" {
		return opts
	}
	// Preserve an explicitly-false autoCountryGroups: decode into a probe with
	// pointer so we can tell "absent" from "false".
	var probe struct {
		AutoCountryGroups   *bool                           `json:"autoCountryGroups"`
		ChainProxy          bool                            `json:"chainProxy"`
		ChainProxyNodeID    int64                           `json:"chainProxyNodeId"`
		ChainProxyNodeIDs   []int64                         `json:"chainProxyNodeIds"`
		GroupSelections     map[string]models.NodeSelection `json:"groupSelections"`
		AutoCountrySelected *models.NodeSelection           `json:"autoCountrySelection"`
		ChainProxySelected  *models.NodeSelection           `json:"chainProxySelection"`
	}
	if err := json.Unmarshal([]byte(blob), &probe); err != nil {
		return opts
	}
	if probe.AutoCountryGroups != nil {
		opts.AutoCountryGroups = *probe.AutoCountryGroups
	}
	opts.ChainProxy = probe.ChainProxy
	opts.ChainProxyNodeID = probe.ChainProxyNodeID
	opts.ChainProxyNodeIDs = uniqueInt64s(probe.ChainProxyNodeIDs)
	if len(opts.ChainProxyNodeIDs) == 0 && opts.ChainProxyNodeID != 0 {
		opts.ChainProxyNodeIDs = []int64{opts.ChainProxyNodeID}
	}
	opts.GroupSelections = normalizeGroupSelections(probe.GroupSelections)
	if probe.AutoCountrySelected != nil {
		normalized := normalizeNodeSelection(*probe.AutoCountrySelected)
		opts.AutoCountrySelected = &normalized
	}
	if opts.AutoCountryGroups && opts.AutoCountrySelected == nil && len(opts.GroupSelections) > 0 {
		normalized := normalizeNodeSelection(mergeGroupSelections(opts.GroupSelections))
		if selectionHasInputs(normalized) {
			opts.AutoCountrySelected = &normalized
		}
	}
	if probe.ChainProxySelected != nil {
		normalized := normalizeNodeSelection(*probe.ChainProxySelected)
		opts.ChainProxySelected = &normalized
	}
	return opts
}
