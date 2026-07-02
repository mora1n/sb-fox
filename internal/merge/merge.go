package merge

import (
	"encoding/json"
	"errors"
	"strconv"
)

// jsonInt returns an integer as a json.Number so it serializes without a
// decimal point, matching how the templates encode numeric fields.
func jsonInt(n int) json.Number {
	return json.Number(strconv.Itoa(n))
}

// Options controls a Generate run (mirrors merge.js args beyond template/nodes).
type Options struct {
	// AutoCountryGroups toggles country-selector generation (requirement c).
	// When false, nodes are appended and Relay is populated, but no country
	// selectors are created and no country tags are fanned out to groups.
	AutoCountryGroups bool
	// CountryHeatOrder ranks country selectors before the region fallback sort.
	// Empty means DefaultCountryHeatOrder().
	CountryHeatOrder []string
	// ChainProxy adds a selector grouping non-chain options. ChainProxyTag is
	// the outbound tag used as the detour target by callers.
	ChainProxy    bool
	ChainProxyTag string
}

// DefaultOptions returns options matching merge.js defaults.
func DefaultOptions() Options {
	return Options{AutoCountryGroups: true}
}

// Generate ports merge.js main(): given a parsed template config and a set of
// nodes, it injects the nodes, builds country/others selectors, wires up the
// Proxy/Auto/Relay/Mainland groups and returns the final config. The template
// is mutated in place.
func Generate(config *OrderedMap, nodes []*Node, opts Options) (*OrderedMap, error) {
	if config == nil {
		return nil, errors.New("merge: nil template config")
	}

	for _, n := range nodes {
		n.applySourceTagging()
	}

	if len(nodes) == 0 {
		finalizeConfig(config)
		return config, nil
	}

	outbounds, err := configOutbounds(config)
	if err != nil {
		return nil, err
	}

	info := collectNodeInfo(nodes, outbounds)
	for _, n := range info.validProxies {
		outbounds = append(outbounds, n.Raw)
	}
	config.Set("outbounds", outbounds)

	groups := findOutboundGroups(outbounds)
	appendUniqueTags(groups.relay, info.relayDirect)

	if !opts.AutoCountryGroups {
		if opts.ChainProxy {
			tags := nonChainProxyTags(info.validProxies, opts.ChainProxyTag)
			if chainSelector := createChainProxySelector(tags); chainSelector != nil {
				outbounds = insertAdditionalOutbounds(outbounds, groups.directIdx, []any{chainSelector})
				config.Set("outbounds", outbounds)
				appendUniqueTags(groups.proxy, []string{chainSelector.GetString("tag")})
				appendUniqueTags(groups.auto, []string{chainSelector.GetString("tag")})
			}
		}
		for _, n := range info.validProxies {
			n.cleanupInternalFields()
		}
		finalizeConfig(config)
		return config, nil
	}

	countrySelectors := createCountrySelectors(info, opts.CountryHeatOrder)
	countryTags := make([]string, len(countrySelectors))
	for i, s := range countrySelectors {
		countryTags[i] = s.GetString("tag")
	}

	var othersSelector *OrderedMap
	if len(info.unrecognizedTags) > 0 {
		othersSelector = NewOrderedMap()
		othersSelector.Set("type", "selector")
		othersSelector.Set("tag", "🏳️‍🌈 Others")
		othersSelector.Set("outbounds", toAnySlice(info.unrecognizedTags))
	}

	selectableTags := append([]string{}, countryTags...)
	if othersSelector != nil {
		selectableTags = append(selectableTags, othersSelector.GetString("tag"))
	}
	var chainSelector *OrderedMap
	if opts.ChainProxy {
		chainSelector = createChainProxySelector(selectableTags)
	}
	appendUniqueTags(groups.proxy, selectableTags)
	appendUniqueTags(groups.auto, selectableTags)
	if chainSelector != nil {
		appendUniqueTags(groups.proxy, []string{chainSelector.GetString("tag")})
		appendUniqueTags(groups.auto, []string{chainSelector.GetString("tag")})
	}

	outbounds = insertCountrySelectors(outbounds, groups.directIdx, countrySelectors, othersSelector, chainSelector)
	config.Set("outbounds", outbounds)

	chinaTag := chinaSelectorTag(countrySelectors)
	finalizeOutboundGroups(outbounds, countryTags, groups.mainland, chinaTag)

	for _, n := range info.validProxies {
		n.cleanupInternalFields()
	}
	finalizeConfig(config)
	return config, nil
}

// insertCountrySelectors ports the splice logic: insert country selectors (and
// Others) before the Direct outbound, or append when Direct is absent.
func insertCountrySelectors(outbounds []any, directIdx int, selectors []*OrderedMap, others, chain *OrderedMap) []any {
	inserts := make([]any, 0, len(selectors)+1)
	for _, s := range selectors {
		inserts = append(inserts, s)
	}
	if chain != nil {
		inserts = append(inserts, chain)
	}

	if directIdx != -1 {
		// merge.js splices Others first, then country selectors, both at
		// directIndex — producing [countrySelectors..., Others] before Direct.
		block := make([]any, 0, len(inserts)+1)
		block = append(block, inserts...)
		if others != nil {
			block = append(block, others)
		}
		result := make([]any, 0, len(outbounds)+len(block))
		result = append(result, outbounds[:directIdx]...)
		result = append(result, block...)
		result = append(result, outbounds[directIdx:]...)
		return result
	}

	outbounds = append(outbounds, inserts...)
	if others != nil {
		outbounds = append(outbounds, others)
	}
	return outbounds
}

func insertAdditionalOutbounds(outbounds []any, directIdx int, inserts []any) []any {
	if len(inserts) == 0 {
		return outbounds
	}
	if directIdx == -1 {
		return append(outbounds, inserts...)
	}
	result := make([]any, 0, len(outbounds)+len(inserts))
	result = append(result, outbounds[:directIdx]...)
	result = append(result, inserts...)
	result = append(result, outbounds[directIdx:]...)
	return result
}

func createChainProxySelector(tags []string) *OrderedMap {
	if len(tags) == 0 {
		return nil
	}
	sel := NewOrderedMap()
	sel.Set("type", "selector")
	sel.Set("tag", "🔗 Chain Proxy")
	sel.Set("outbounds", toAnySlice(tags))
	return sel
}

func nonChainProxyTags(nodes []*Node, chainTag string) []string {
	tags := make([]string, 0, len(nodes))
	for _, n := range nodes {
		tag := n.tag()
		if tag == "" || tag == chainTag {
			continue
		}
		tags = append(tags, tag)
	}
	return tags
}

func chinaSelectorTag(selectors []*OrderedMap) string {
	for _, s := range selectors {
		if matchTag(s.GetString("tag"), "China") {
			return s.GetString("tag")
		}
	}
	return ""
}

// finalizeConfig ports finalizeConfig.
func finalizeConfig(config *OrderedMap) {
	if outbounds, err := configOutbounds(config); err == nil {
		applyDefaultURLTestURL(outbounds)
	}
}

// configOutbounds returns the config's outbounds as a slice, erroring if the
// key is missing or not an array.
func configOutbounds(config *OrderedMap) ([]any, error) {
	raw, ok := config.Get("outbounds")
	if !ok {
		return nil, errors.New("merge: template has no outbounds array")
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, errors.New("merge: template outbounds is not an array")
	}
	return arr, nil
}

func toAnySlice(strs []string) []any {
	out := make([]any, len(strs))
	for i, s := range strs {
		out[i] = s
	}
	return out
}
