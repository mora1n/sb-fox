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

func TestGenerateConfigWithGroupSelectionsAndChainProxy(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Rule","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Rule"}],
    "final":"Proxy"
  }
}`
	proxyNode := testNode(1, "proxy-node", "US")
	ruleNode := testNode(2, "rule-node", "JP")
	chainNode := testNode(3, "chain-node", "SG")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {proxyNode},
		"Rule":  {ruleNode},
	}, []*models.Node{chainNode}, models.ProfileOptions{ChainProxy: true}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := outbounds["chain-node"]["detour"]; got != merge.ChainProxyTag {
		t.Fatalf("chain-node detour = %v, want %q", got, merge.ChainProxyTag)
	}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"proxy-node", "chain-node"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Rule"]["outbounds"]); !sameStrings(got, []string{"rule-node", "chain-node"}) {
		t.Fatalf("Rule outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds[merge.ChainProxyTag]["outbounds"]); !sameStrings(got, []string{"proxy-node", "rule-node"}) {
		t.Fatalf("chain selector outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionOutboundRef(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Proxy","Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`
	proxyNode := testNode(1, "proxy-node", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {proxyNode},
	}, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {NodeIDs: []int64{1}},
			"Fallback": {OutboundRefs: []string{"Proxy"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"Proxy"}) {
		t.Fatalf("Fallback outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionAutoCountryGroups(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`
	proxyNode := testNode(1, "proxy-node", "US")
	fallbackNode := testNode(2, "fallback-node", "JP")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy":    {proxyNode},
		"Fallback": {fallbackNode},
	}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {NodeIDs: []int64{1}},
			"Fallback": {NodeIDs: []int64{2}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇺🇸 United States"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"🇯🇵 Japan"}) {
		t.Fatalf("Fallback outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸 United States"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionSkipCountryGroups(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`
	proxyNode := testNode(1, "proxy-node", "US")
	fallbackNode := testNode(2, "fallback-node", "JP")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy":    {proxyNode},
		"Fallback": {fallbackNode},
	}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {NodeIDs: []int64{1}, SkipCountryGroups: true},
			"Fallback": {NodeIDs: []int64{2}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"🇯🇵 Japan"}) {
		t.Fatalf("Fallback outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionRejectsInvalidOutboundRef(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`
	err := validateOptionOutboundRefs(template, models.ProfileOptions{
		GroupSelections: map[string]models.NodeSelection{
			"Fallback": {OutboundRefs: []string{"Proxy"}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid outbound ref error")
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

func generatedOutboundMap(t *testing.T, config []byte) map[string]map[string]any {
	t.Helper()
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
	return outbounds
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

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
