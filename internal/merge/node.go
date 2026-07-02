package merge

// Node is one proxy outbound plus the sidecar metadata merge.js tracks on the
// object as `_source` / `_countryOverride`. Keeping them beside the raw object
// (rather than injecting temporary keys) avoids perturbing JSON key order.
type Node struct {
	// Raw is the parsed outbound object. Generate may mutate it: stripping a
	// `#CC` suffix from `server` and deleting stray country fields on cleanup.
	Raw *OrderedMap
	// Source mirrors `_source`. "protocol" additionally triggers `#CC` server
	// override extraction (matching tagProtocolNode); any non-empty value makes
	// a detour-less node eligible for the Relay group.
	Source string
	// CountryOverride mirrors `_countryOverride`. Pre-set for manual country
	// marking (requirement e); protocol `#CC` extraction fills it automatically.
	CountryOverride string
}

func (n *Node) tag() string    { return n.Raw.GetString("tag") }
func (n *Node) server() string { return n.Raw.GetString("server") }

func (n *Node) hasDetour() bool {
	v, ok := n.Raw.Get("detour")
	if !ok || v == nil {
		return false
	}
	s, ok := v.(string)
	return ok && s != ""
}

// applySourceTagging ports tagProtocolNode/tagSubscriptionNode. Protocol nodes
// get `#CC` extraction; subscription (and others) only carry their source.
func (n *Node) applySourceTagging() {
	if n.Source == "protocol" {
		if ov := ExtractServerCountryOverride(n.server()); ov != nil {
			n.Raw.Set("server", ov.Server)
			n.CountryOverride = ov.CountryCode
		}
	}
}

// resolveCountry ports resolveCountryForProxy: explicit override wins, else
// extract from the tag.
func (n *Node) resolveCountry() *CountryInfo {
	if info := resolveCountryOverride(n.CountryOverride); info != nil {
		return info
	}
	return extractCountry(n.tag())
}

// cleanupInternalFields ports cleanupInternalFields for the raw object: the
// sidecar fields never touch Raw, but stray `#country`/`country` keys are
// removed to match merge.js output.
func (n *Node) cleanupInternalFields() {
	n.Raw.Delete("#country")
	n.Raw.Delete("country")
}
