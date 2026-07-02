package merge

// countryGroup accumulates node tags detected for one country, preserving
// first-seen order (mirrors the JS Map value {name, emoji, tags}).
type countryGroup struct {
	name  string
	emoji string
	tags  []string
}

// nodeInfo is the result of collectNodeInfo.
type nodeInfo struct {
	validProxies     []*Node
	countryOrder     []string // country codes in first-seen order
	countryGroups    map[string]*countryGroup
	unrecognizedTags []string
	relayDirect      []string
}

// collectNodeInfo ports collectNodeInfo: skip nodes whose tag already exists in
// the template outbounds; classify the rest into country groups / unrecognized,
// and collect detour-less sourced nodes into relayDirect.
func collectNodeInfo(nodes []*Node, existingOutbounds []any) *nodeInfo {
	existingTags := make(map[string]struct{})
	for _, ob := range existingOutbounds {
		if om, ok := ob.(*OrderedMap); ok {
			if tag := om.GetString("tag"); tag != "" {
				existingTags[tag] = struct{}{}
			}
		}
	}

	info := &nodeInfo{countryGroups: make(map[string]*countryGroup)}

	for _, node := range nodes {
		tag := node.tag()
		if _, exists := existingTags[tag]; exists {
			continue
		}

		info.validProxies = append(info.validProxies, node)
		if !node.hasDetour() && (node.Source == "protocol" || node.Source == "subscription") {
			info.relayDirect = append(info.relayDirect, tag)
		}

		country := node.resolveCountry()
		if country != nil {
			group, ok := info.countryGroups[country.Code]
			if !ok {
				group = &countryGroup{name: country.Name, emoji: country.Emoji}
				info.countryGroups[country.Code] = group
				info.countryOrder = append(info.countryOrder, country.Code)
			}
			group.tags = append(group.tags, tag)
			continue
		}

		info.unrecognizedTags = append(info.unrecognizedTags, tag)
	}

	return info
}
