package api

import (
	"encoding/json"
	"testing"

	"github.com/mora1n/sb-fox/internal/merge"
	"github.com/mora1n/sb-fox/internal/models"
)

func TestGenerateConfigChainProxyAddsDetourToSelectedNodes(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"🚀 Proxy","outbounds":[]},
    {"type":"urltest","tag":"⚡ Auto","outbounds":[]},
    {"type":"direct","tag":"🎯 Direct"}
  ]
}`
	nodes := []*models.Node{
		testNode(1, "down-a", "US"),
		testNode(2, "down-b", "JP"),
		testNode(3, "upstream", "SG"),
	}
	config, err := generateConfig(template, nodes, models.ProfileOptions{
		AutoCountryGroups: true,
		ChainProxy:        true,
		ChainProxyNodeIDs: []int64{1, 2},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfig: %v", err)
	}

	var out struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(config, &out); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}
	outbounds := map[string]map[string]any{}
	for _, ob := range out.Outbounds {
		if tag, ok := ob["tag"].(string); ok {
			outbounds[tag] = ob
		}
	}
	for _, tag := range []string{"down-a", "down-b"} {
		if got := outbounds[tag]["detour"]; got != merge.ChainProxyTag {
			t.Fatalf("%s detour = %v, want %q", tag, got, merge.ChainProxyTag)
		}
	}
	if got := outbounds["upstream"]["detour"]; got != nil {
		t.Fatalf("upstream detour = %v", got)
	}
	chain := outbounds[merge.ChainProxyTag]
	if chain == nil {
		t.Fatal("missing chain proxy selector")
	}
	tags := stringSliceValue(t, chain["outbounds"])
	if len(tags) != 1 || tags[0] != "upstream" {
		t.Fatalf("chain selector outbounds = %v", tags)
	}
}

func TestParseProfileOptionsMigratesLegacyChainProxyNodeID(t *testing.T) {
	opts := parseProfileOptions(`{"autoCountryGroups":false,"chainProxy":true,"chainProxyNodeId":42}`)
	if opts.AutoCountryGroups {
		t.Fatal("autoCountryGroups should remain false")
	}
	if !opts.ChainProxy || len(opts.ChainProxyNodeIDs) != 1 || opts.ChainProxyNodeIDs[0] != 42 {
		t.Fatalf("opts = %+v", opts)
	}
}

func testNode(id int64, tag, country string) *models.Node {
	raw := `{"type":"shadowsocks","tag":"` + tag + `","server":"example.com","server_port":443}`
	return &models.Node{
		ID:            id,
		OwnerUserID:   1,
		Tag:           tag,
		Type:          "shadowsocks",
		CountryCode:   country,
		CountrySource: "manual",
		Source:        "manual",
		Raw:           raw,
	}
}

func stringSliceValue(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value is not array: %T", value)
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("array contains non-string: %T", v)
		}
		out = append(out, s)
	}
	return out
}
