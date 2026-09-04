package merge

import "sort"

// Group tag names (emoji-stripped, lowercased for matching) — mirrors GROUP_NAMES.
const (
	groupProxy    = "Proxy"
	groupRelay    = "Relay"
	groupAuto     = "Auto"
	groupDirect   = "Direct"
	groupMainland = "Mainland"
	groupReject   = "Reject"
	groupOthers   = "Others"
	groupChain    = "Chain Proxy"
)

// countryOptionExcludedGroups mirrors COUNTRY_OPTION_EXCLUDED_GROUPS.
var countryOptionExcludedGroups = []string{
	groupDirect, groupReject, groupOthers, groupAuto, groupRelay, groupProxy, groupMainland,
	groupChain,
}

const defaultURLTestURL = "https://cp.cloudflare.com/generate_204"

// outboundGroups holds references to the well-known template groups.
type outboundGroups struct {
	proxy     *OrderedMap
	relay     *OrderedMap
	auto      *OrderedMap
	mainland  *OrderedMap
	directIdx int
}

// findOutboundGroups ports findOutboundGroups: locate Proxy/Relay/Auto/Mainland
// groups and the index of the Direct outbound by emoji-stripped tag match.
func findOutboundGroups(outbounds []any) outboundGroups {
	groups := outboundGroups{directIdx: -1}
	for i, ob := range outbounds {
		om, ok := ob.(*OrderedMap)
		if !ok {
			continue
		}
		tag := om.GetString("tag")
		if tag == "" {
			continue
		}
		if groups.proxy == nil && matchTag(tag, groupProxy) {
			groups.proxy = om
		}
		if groups.relay == nil && matchTag(tag, groupRelay) {
			groups.relay = om
		}
		if groups.auto == nil && matchTag(tag, groupAuto) {
			groups.auto = om
		}
		if groups.mainland == nil && matchTag(tag, groupMainland) {
			groups.mainland = om
		}
		if groups.directIdx == -1 && matchTag(tag, groupDirect) {
			groups.directIdx = i
		}
	}
	return groups
}

// appendUniqueTags ports appendUniqueTags: coerce null outbounds to [], then
// append tags not already present. No-op when group is nil or its outbounds is
// not an array (after the null coercion).
func appendUniqueTags(group *OrderedMap, tags []string) {
	if group == nil || len(tags) == 0 {
		return
	}
	raw, ok := group.Get("outbounds")
	if ok && raw == nil {
		group.Set("outbounds", []any{})
		raw, _ = group.Get("outbounds")
	}
	arr, ok := raw.([]any)
	if !ok {
		return
	}
	seen := make(map[string]struct{}, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			seen[s] = struct{}{}
		}
	}
	for _, tag := range tags {
		if _, exists := seen[tag]; !exists {
			seen[tag] = struct{}{}
			arr = append(arr, tag)
		}
	}
	group.Set("outbounds", arr)
}

// createCountrySelectors sorts country groups by the hot/europe/region ranking
// and builds a selector outbound for each.
func createCountrySelectors(info *nodeInfo, countryHeatOrder []string) []*OrderedMap {
	codes := make([]string, len(info.countryOrder))
	copy(codes, info.countryOrder)
	sortCache := buildCountrySortCache(countryHeatOrder)

	sort.SliceStable(codes, func(i, j int) bool {
		a, b := getCountrySortInfo(sortCache, codes[i]), getCountrySortInfo(sortCache, codes[j])
		if a.bucket != b.bucket {
			return a.bucket < b.bucket
		}
		if a.priority != b.priority {
			return a.priority < b.priority
		}
		return codes[i] < codes[j]
	})

	var selectors []*OrderedMap
	for _, code := range codes {
		group := info.countryGroups[code]
		if len(group.tags) == 0 {
			continue
		}
		sel := NewOrderedMap()
		sel.Set("type", "selector")
		sel.Set("tag", group.emoji+group.name)
		tags := make([]any, len(group.tags))
		for i, t := range group.tags {
			tags[i] = t
		}
		sel.Set("outbounds", tags)
		selectors = append(selectors, sel)
	}
	return selectors
}

// shouldSkipCountryOptions ports shouldSkipCountryOptions.
func shouldSkipCountryOptions(tag string, countryTags map[string]struct{}) bool {
	if _, ok := countryTags[tag]; ok {
		return true
	}
	for _, name := range countryOptionExcludedGroups {
		if matchTag(tag, name) {
			return true
		}
	}
	return false
}
