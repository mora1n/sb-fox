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
	}, nil, []*models.Node{chainNode}, models.ProfileOptions{ChainProxy: true}, nil)
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
	}, nil, nil, models.ProfileOptions{
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

func TestGenerateConfigWithGroupSelectionUsesTemplateDefaultRefs(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}
	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"Direct"}) {
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
	countrySourceNode := testNode(3, "source-node", "SG")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy":    {proxyNode},
		"Fallback": {fallbackNode},
	}, []*models.Node{proxyNode, countrySourceNode}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {NodeIDs: []int64{1}},
			"Fallback": {NodeIDs: []int64{2}},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 3}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇺🇸 United States"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"fallback-node"}) {
		t.Fatalf("Fallback outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸 United States"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇸🇬 Singapore"]["outbounds"]); !sameStrings(got, []string{"source-node"}) {
		t.Fatalf("SG selector outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionAutoCountrySourceFillsUnselectedGroups(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"selector","tag":"Rule","outbounds":[]},
    {"type":"selector","tag":"Skip","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [
      {"domain":["rule.example"],"outbound":"Rule"},
      {"domain":["skip.example"],"outbound":"Skip"}
    ],
    "final":"Proxy"
  }
}`
	usNode := testNode(1, "us-node", "US")
	jpNode := testNode(2, "jp-node", "JP")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, []*models.Node{usNode, jpNode}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {},
			"Rule":  {},
			"Skip":  {SkipCountryGroups: true},
		},
		AutoCountrySelected: &models.NodeSelection{NodeGroupIDs: []int64{10}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	countryRefs := []string{"🇯🇵 Japan", "🇺🇸 United States"}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, append([]string{"Direct"}, countryRefs...)) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Rule"]["outbounds"]); !sameStrings(got, countryRefs) {
		t.Fatalf("Rule outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Skip"]["outbounds"]); !sameStrings(got, []string{"Direct"}) {
		t.Fatalf("Skip outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇯🇵 Japan"]["outbounds"]); !sameStrings(got, []string{"jp-node"}) {
		t.Fatalf("JP selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸 United States"]["outbounds"]); !sameStrings(got, []string{"us-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
}

func TestGenerateConfigUsesStoredAutoCountryForGrouping(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ]
}`
	node := testNode(1, "plain-node", "JP")
	node.CountrySource = "auto"
	config, err := generateConfig(template, []*models.Node{node}, models.ProfileOptions{
		AutoCountryGroups: true,
	}, nil)
	if err != nil {
		t.Fatalf("generateConfig: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["🇯🇵 Japan"]["outbounds"]); !sameStrings(got, []string{"plain-node"}) {
		t.Fatalf("JP selector outbounds = %v", got)
	}
	if _, ok := outbounds["🏳️‍🌈 Others"]; ok {
		t.Fatalf("plain-node with stored country should not be placed in Others")
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
	}, []*models.Node{proxyNode, fallbackNode}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {NodeIDs: []int64{1}, SkipCountryGroups: true},
			"Fallback": {NodeIDs: []int64{2}},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 2}},
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

func TestGenerateConfigWithGroupSelectionRejectsCycles(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Fallback","Direct"]},
    {"type":"selector","tag":"Fallback","outbounds":["Proxy","Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Fallback"}],
    "final":"Proxy"
  }
}`
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {OutboundRefs: []string{"Fallback"}},
			"Fallback": {OutboundRefs: []string{"Proxy"}},
		},
	}, nil)
	if err == nil {
		t.Fatal("expected group cycle error")
	}
}

func TestGenerateConfigWithGroupSelectionRequiresAutoCountrySource(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	proxyNode := testNode(1, "proxy-node", "US")
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {proxyNode},
	}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {NodeIDs: []int64{1}},
		},
		AutoCountrySelected: &models.NodeSelection{},
	}, nil)
	if err == nil {
		t.Fatal("expected auto country source error")
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
