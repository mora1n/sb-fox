package merge

// finalizeOutboundGroups ports finalizeOutboundGroups: for every selector/
// urltest group that isn't excluded or a country group itself, append all
// country tags; set default urltest url where missing; append China to Mainland.
func finalizeOutboundGroups(outbounds []any, countryTags []string, mainland *OrderedMap, chinaTag string) {
	countryTagSet := make(map[string]struct{}, len(countryTags))
	for _, t := range countryTags {
		countryTagSet[t] = struct{}{}
	}

	for _, ob := range outbounds {
		group, ok := ob.(*OrderedMap)
		if !ok {
			continue
		}
		typ := group.GetString("type")

		if typ == "urltest" && group.GetString("url") == "" {
			group.Set("url", defaultURLTestURL)
		}

		if len(countryTags) == 0 || (typ != "selector" && typ != "urltest") || !group.Has("outbounds") {
			continue
		}

		raw, _ := group.Get("outbounds")
		if raw == nil {
			group.Set("outbounds", []any{})
		}
		if _, isArr := group.mustOutbounds(); !isArr {
			continue
		}
		if shouldSkipCountryOptions(group.GetString("tag"), countryTagSet) {
			continue
		}
		appendUniqueTags(group, countryTags)
	}

	if chinaTag != "" {
		appendUniqueTags(mainland, []string{chinaTag})
	}
}

// mustOutbounds reports whether the group's outbounds is currently an array.
func (m *OrderedMap) mustOutbounds() ([]any, bool) {
	raw, _ := m.Get("outbounds")
	arr, ok := raw.([]any)
	return arr, ok
}

// applyDefaultURLTestURL ports applyDefaultURLTestURL.
func applyDefaultURLTestURL(outbounds []any) {
	for _, ob := range outbounds {
		if group, ok := ob.(*OrderedMap); ok {
			if group.GetString("type") == "urltest" && group.GetString("url") == "" {
				group.Set("url", defaultURLTestURL)
			}
		}
	}
}
