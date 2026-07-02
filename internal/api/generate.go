package api

import (
	"encoding/json"
	"fmt"

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

	out, err := merge.Generate(cfg, mergeNodes, merge.Options{
		AutoCountryGroups: opts.AutoCountryGroups,
		CountryHeatOrder:  countryHeatOrder,
	})
	if err != nil {
		return nil, err
	}

	compact, err := out.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return merge.Indent(compact)
}

// toMergeNodes converts DB node rows into merge.Node values. A node whose
// stored country_source is "manual" carries its country_code as an explicit
// override so manual grouping (requirement e) is honored during generation.
func toMergeNodes(nodes []*models.Node) ([]*merge.Node, error) {
	out := make([]*merge.Node, 0, len(nodes))
	for _, n := range nodes {
		raw, err := merge.ParseOrdered([]byte(n.Raw))
		if err != nil {
			return nil, fmt.Errorf("node %d (%s) has invalid raw JSON: %w", n.ID, n.Tag, err)
		}
		mn := &merge.Node{Raw: raw, Source: mergeSource(n.Source)}
		if n.CountrySource == "manual" && n.CountryCode != "" {
			mn.CountryOverride = n.CountryCode
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
		AutoCountryGroups *bool `json:"autoCountryGroups"`
		ChainProxy        bool  `json:"chainProxy"`
	}
	if err := json.Unmarshal([]byte(blob), &probe); err != nil {
		return opts
	}
	if probe.AutoCountryGroups != nil {
		opts.AutoCountryGroups = *probe.AutoCountryGroups
	}
	opts.ChainProxy = probe.ChainProxy
	return opts
}
