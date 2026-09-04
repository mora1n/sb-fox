package api

import (
	"encoding/json"
	"errors"
	"strings"
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
	assertNoGeneratedReference(t, outbounds, "down-a")
	assertNoGeneratedReference(t, outbounds, "down-b")
	for _, tag := range []string{"📥down-a", "📥down-b"} {
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
	assertNoGeneratedReference(t, outbounds, "chain-node")
	if got := outbounds["📥chain-node"]["detour"]; got != merge.ChainProxyTag {
		t.Fatalf("chain-node detour = %v, want %q", got, merge.ChainProxyTag)
	}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Rule"]["outbounds"]); !sameStrings(got, []string{"rule-node"}) {
		t.Fatalf("Rule outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds[merge.ChainProxyTag]["outbounds"]); !sameStrings(got, []string{"proxy-node", "rule-node"}) {
		t.Fatalf("chain selector outbounds = %v", got)
	}
}

func TestGenerateConfigChainProxyUsesAutoCountryUpstreams(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	proxyNode := testNode(1, "proxy-node", "US")
	autoNodeA := testNode(2, "auto-a", "JP")
	autoNodeB := testNode(3, "auto-b", "SG")
	chainNode := testNode(4, "chain-node", "HK")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {proxyNode},
	}, []*models.Node{proxyNode, autoNodeA, chainNode, autoNodeB}, []*models.Node{chainNode}, models.ProfileOptions{
		AutoCountryGroups: true,
		ChainProxy:        true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {NodeIDs: []int64{1}},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 2, 3, 4}},
		ChainProxySelected:  &models.NodeSelection{NodeIDs: []int64{4}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	assertNoGeneratedReference(t, outbounds, "chain-node")
	if got := outbounds["📥chain-node"]["detour"]; got != merge.ChainProxyTag {
		t.Fatalf("chain-node detour = %v, want %q", got, merge.ChainProxyTag)
	}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇺🇸United States"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇭🇰Hong Kong"]["outbounds"]); !sameStrings(got, []string{"📥chain-node"}) {
		t.Fatalf("HK selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds[merge.ChainProxyTag]["outbounds"]); !sameStrings(got, []string{"proxy-node", "auto-a", "auto-b"}) {
		t.Fatalf("chain selector outbounds = %v", got)
	}
}

func TestGenerateConfigStripsURLTestDefault(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Auto","Direct"]},
    {"type":"urltest","tag":"Auto","outbounds":["node-a"],"default":"node-a"},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	config, err := generateConfig(template, []*models.Node{testNode(1, "node-a", "US")}, models.ProfileOptions{}, nil)
	if err != nil {
		t.Fatalf("generateConfig: %v", err)
	}
	outbounds := generatedOutboundMap(t, config)
	auto := outbounds["Auto"]
	if auto == nil {
		t.Fatal("missing Auto outbound")
	}
	if _, ok := auto["default"]; ok {
		t.Fatalf("urltest outbound kept default field: %+v", auto)
	}
}

func TestGenerateConfigChainProxyReplacesAutoCountrySelectedNodeRefs(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	chainNode := testNode(1, "👼 BWH.MINIBOX", "HK")
	upstream := testNode(2, "naive-upstream", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {chainNode},
	}, []*models.Node{chainNode, upstream}, []*models.Node{chainNode}, models.ProfileOptions{
		AutoCountryGroups: true,
		ChainProxy:        true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {NodeIDs: []int64{1}},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 2}},
		ChainProxySelected:  &models.NodeSelection{NodeIDs: []int64{1}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	assertNoGeneratedReference(t, outbounds, "👼 BWH.MINIBOX")
	if got := outbounds["📥👼 BWH.MINIBOX"]["detour"]; got != merge.ChainProxyTag {
		t.Fatalf("chain node detour = %v, want %q", got, merge.ChainProxyTag)
	}
	if got := stringSliceValue(t, outbounds["🇭🇰Hong Kong"]["outbounds"]); !sameStrings(got, []string{"📥👼 BWH.MINIBOX"}) {
		t.Fatalf("HK selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇭🇰Hong Kong"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
}

func TestGenerateConfigChainProxyDoesNotFillFinalGroup(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":[]},
    {"type":"selector","tag":"Upstream","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["upstream.example"],"outbound":"Upstream"}],
    "final":"Proxy"
  }
}`
	chainNode := testNode(1, "chain-node", "HK")
	upstream := testNode(2, "upstream", "US")
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Upstream": {upstream},
	}, nil, []*models.Node{chainNode}, models.ProfileOptions{
		AutoCountryGroups: false,
		ChainProxy:        true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy":    {},
			"Upstream": {NodeIDs: []int64{2}},
		},
		ChainProxySelected: &models.NodeSelection{NodeIDs: []int64{1}},
	}, nil)
	if err == nil {
		t.Fatal("expected final selector error")
	}
	var genErr *generationError
	if !errors.As(err, &genErr) {
		t.Fatalf("err = %T %v, want generationError", err, err)
	}
	if genErr.details.Kind != generateErrInvalidFinal || genErr.details.GroupTag != "Proxy" {
		t.Fatalf("details = %+v, want invalid final for Proxy", genErr.details)
	}
}

func TestGenerateConfigChainProxyReplacesSkipCountryNodeRefs(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	chainNode := testNode(1, "👼 BWH.MINIBOX", "HK")
	upstream := testNode(2, "naive-upstream", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Proxy": {chainNode},
	}, []*models.Node{chainNode, upstream}, []*models.Node{chainNode}, models.ProfileOptions{
		AutoCountryGroups: true,
		ChainProxy:        true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {NodeIDs: []int64{1}, SkipCountryGroups: true},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1, 2}},
		ChainProxySelected:  &models.NodeSelection{NodeIDs: []int64{1}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	assertNoGeneratedReference(t, outbounds, "👼 BWH.MINIBOX")
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"📥👼 BWH.MINIBOX"}) {
		t.Fatalf("Proxy outbounds = %v", got)
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
			"Fallback": {OutboundRefs: []string{"Direct", "Proxy"}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"Direct", "Proxy"}) {
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

func TestGenerateConfigWithNestedURLTestGroupSelection(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Auto"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	autoNode := testNode(1, "auto-node", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Auto": {autoNode},
	}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Auto": {NodeIDs: []int64{1}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"Auto"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Auto"]["outbounds"]); !sameStrings(got, []string{"auto-node"}) {
		t.Fatalf("Auto outbounds = %v", got)
	}
}

func TestGenerateConfigRejectsEmptyNestedURLTest(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Auto"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
	}, nil)
	if err == nil {
		t.Fatal("expected empty nested urltest error")
	}
	var ge *generationError
	if !errors.As(err, &ge) {
		t.Fatalf("expected generation error, got %T %v", err, err)
	}
	if ge.details.Kind != generateErrEmptyGroup || ge.details.GroupTag != "Auto" {
		t.Fatalf("details = %+v", ge.details)
	}
}

func TestGenerateConfigWithUnreachableEmptyURLTestGroupSelection(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	autoNode := testNode(1, "auto-node", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{
		"Auto": {autoNode},
	}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Auto": {NodeIDs: []int64{1}},
		},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"Direct"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Auto"]["outbounds"]); !sameStrings(got, []string{"auto-node"}) {
		t.Fatalf("Auto outbounds = %v", got)
	}
}

func TestGenerateConfigRejectsUnconfiguredUnreachableEmptyURLTest(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"urltest","tag":"Auto","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
	}, nil)
	if err == nil {
		t.Fatal("expected empty unreachable urltest error")
	}
	var ge *generationError
	if !errors.As(err, &ge) {
		t.Fatalf("expected generation error, got %T %v", err, err)
	}
	if ge.details.Kind != generateErrEmptyGroup || ge.details.GroupTag != "Auto" {
		t.Fatalf("details = %+v", ge.details)
	}
}

func TestGenerateConfigWithGroupSelectionAllowsStaticFinal(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Proxy"}],
    "final":"Direct"
  }
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
	if got := outbounds["Direct"]["type"]; got != "direct" {
		t.Fatalf("Direct type = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionAllowsMissingFinal(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Proxy"}]
  }
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
	route := generatedRouteMap(t, config)
	if _, ok := route["final"]; ok {
		t.Fatalf("route.final should stay omitted: %v", route["final"])
	}
}

func TestGenerateConfigRejectsUnknownFinal(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":["Direct"]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {
    "rules": [{"domain":["example.com"],"outbound":"Proxy"}],
    "final":"Missing"
  }
}`
	_, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, nil, nil, models.ProfileOptions{
		AutoCountryGroups: false,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {},
		},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), `final outbound "Missing" is missing`) {
		t.Fatalf("expected unknown final error, got %v", err)
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
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇺🇸United States"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"fallback-node"}) {
		t.Fatalf("Fallback outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸United States"]["outbounds"]); !sameStrings(got, []string{"proxy-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇸🇬Singapore"]["outbounds"]); !sameStrings(got, []string{"source-node"}) {
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
	countryRefs := []string{"🇯🇵Japan", "🇺🇸United States"}
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, append([]string{"Direct"}, countryRefs...)) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Rule"]["outbounds"]); !sameStrings(got, countryRefs) {
		t.Fatalf("Rule outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["Skip"]["outbounds"]); !sameStrings(got, []string{"Direct"}) {
		t.Fatalf("Skip outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇯🇵Japan"]["outbounds"]); !sameStrings(got, []string{"jp-node"}) {
		t.Fatalf("JP selector outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸United States"]["outbounds"]); !sameStrings(got, []string{"us-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionAutoCountrySourceFillsFinalGroup(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"Proxy","outbounds":[]},
    {"type":"direct","tag":"Direct"}
  ],
  "route": {"final":"Proxy"}
}`
	usNode := testNode(1, "us-node", "US")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, []*models.Node{usNode}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"Proxy": {},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["Proxy"]["outbounds"]); !sameStrings(got, []string{"🇺🇸United States"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇺🇸United States"]["outbounds"]); !sameStrings(got, []string{"us-node"}) {
		t.Fatalf("US selector outbounds = %v", got)
	}
}

func TestGenerateConfigWithGroupSelectionAutoCountrySourceFillsEmojiFinalGroup(t *testing.T) {
	template := `{
  "outbounds": [
    {"type":"selector","tag":"🚀 Proxy","outbounds":[]},
    {"type":"direct","tag":"🎯 Direct"}
  ],
  "route": {"final":"🚀 Proxy"}
}`
	jpNode := testNode(1, "jp-node", "JP")
	config, err := generateConfigWithGroupSelections(template, map[string][]*models.Node{}, []*models.Node{jpNode}, nil, models.ProfileOptions{
		AutoCountryGroups: true,
		GroupSelections: map[string]models.NodeSelection{
			"🚀 Proxy": {},
		},
		AutoCountrySelected: &models.NodeSelection{NodeIDs: []int64{1}},
	}, nil)
	if err != nil {
		t.Fatalf("generateConfigWithGroupSelections: %v", err)
	}

	outbounds := generatedOutboundMap(t, config)
	if got := stringSliceValue(t, outbounds["🚀 Proxy"]["outbounds"]); !sameStrings(got, []string{"🇯🇵Japan"}) {
		t.Fatalf("Proxy outbounds = %v", got)
	}
	if got := stringSliceValue(t, outbounds["🇯🇵Japan"]["outbounds"]); !sameStrings(got, []string{"jp-node"}) {
		t.Fatalf("JP selector outbounds = %v", got)
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
	if got := stringSliceValue(t, outbounds["🇯🇵Japan"]["outbounds"]); !sameStrings(got, []string{"plain-node"}) {
		t.Fatalf("JP selector outbounds = %v", got)
	}
	if _, ok := outbounds["🏳️‍🌈Others"]; ok {
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
	if got := stringSliceValue(t, outbounds["Fallback"]["outbounds"]); !sameStrings(got, []string{"🇯🇵Japan"}) {
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

func generatedRouteMap(t *testing.T, config []byte) map[string]any {
	t.Helper()
	var out struct {
		Route map[string]any `json:"route"`
	}
	if err := json.Unmarshal(config, &out); err != nil {
		t.Fatalf("unmarshal generated config: %v", err)
	}
	return out.Route
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

func assertNoGeneratedReference(t *testing.T, outbounds map[string]map[string]any, tag string) {
	t.Helper()
	if _, ok := outbounds[tag]; ok {
		t.Fatalf("generated outbounds still contain original chain node tag %q", tag)
	}
	for groupTag, outbound := range outbounds {
		raw, ok := outbound["outbounds"].([]any)
		if !ok {
			continue
		}
		for _, item := range raw {
			if item == tag {
				t.Fatalf("outbound %q still references original chain node tag %q", groupTag, tag)
			}
		}
	}
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
