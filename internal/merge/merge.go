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
	// CountryGroupSourceTags limits which injected node tags create country
	// selectors. Nil means all valid proxies; an empty non-nil slice means none.
	CountryGroupSourceTags []string
	// ChainProxy adds a selector grouping the configured upstream options.
	ChainProxy bool
	// ChainProxyOutbounds is the explicit upstream tag list for the selector.
	ChainProxyOutbounds []string
	// ChainProxyTag is kept for compatibility with older callers; when
	// ChainProxyOutbounds is empty, nodes with this tag are excluded.
	ChainProxyTag string
}

const ChainProxyTag = "📤Chain Proxy"

// DefaultOptions returns options matching merge.js defaults.
func DefaultOptions() Options {
	return Options{AutoCountryGroups: true}
}

// Generate ports merge.js main(): given a parsed template config and a set of
// nodes, it injects the nodes, builds country selectors including Custom, wires up the
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
	countryInfo := info
	if opts.CountryGroupSourceTags != nil {
		countryInfo = collectNodeInfo(filterNodesByTags(nodes, opts.CountryGroupSourceTags), outbounds)
	}
	for _, n := range info.validProxies {
		outbounds = append(outbounds, n.Raw)
	}
	config.Set("outbounds", outbounds)

	groups := findOutboundGroups(outbounds)
	appendUniqueTags(groups.relay, info.relayDirect)

	if !opts.AutoCountryGroups {
		if opts.ChainProxy {
			tags := opts.ChainProxyOutbounds
			if len(tags) == 0 {
				tags = nonChainProxyTags(info.validProxies, opts.ChainProxyTag)
			}
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

	countrySelectors := createCountrySelectors(countryInfo, opts.CountryHeatOrder)
	countryTags := make([]string, len(countrySelectors))
	for i, s := range countrySelectors {
		countryTags[i] = s.GetString("tag")
	}

	var chainSelector *OrderedMap
	if opts.ChainProxy {
		tags := opts.ChainProxyOutbounds
		if len(tags) == 0 {
			tags = countryTags
		}
		chainSelector = createChainProxySelector(tags)
	}
	appendUniqueTags(groups.proxy, countryTags)
	appendUniqueTags(groups.auto, countryTags)
	if chainSelector != nil {
		appendUniqueTags(groups.proxy, []string{chainSelector.GetString("tag")})
		appendUniqueTags(groups.auto, []string{chainSelector.GetString("tag")})
	}

	inserts := make([]any, 0, len(countrySelectors)+1)
	for _, selector := range countrySelectors {
		inserts = append(inserts, selector)
	}
	if chainSelector != nil {
		inserts = append(inserts, chainSelector)
	}
	outbounds = insertAdditionalOutbounds(outbounds, groups.directIdx, inserts)
	config.Set("outbounds", outbounds)

	chinaTag := chinaSelectorTag(countrySelectors)
	finalizeOutboundGroups(outbounds, countryTags, groups.mainland, chinaTag)

	for _, n := range info.validProxies {
		n.cleanupInternalFields()
	}
	finalizeConfig(config)
	return config, nil
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
	sel.Set("tag", ChainProxyTag)
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

func filterNodesByTags(nodes []*Node, tags []string) []*Node {
	wanted := make(map[string]bool, len(tags))
	for _, tag := range tags {
		if tag != "" {
			wanted[tag] = true
		}
	}
	out := make([]*Node, 0, len(nodes))
	for _, n := range nodes {
		if n != nil && wanted[n.tag()] {
			out = append(out, n)
		}
	}
	return out
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
