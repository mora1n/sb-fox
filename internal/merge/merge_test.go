package merge

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate golden files from Go output")

// scenario mirrors one DEFAULT_SCENARIOS entry in tests/merge_regression.js.
type scenario struct {
	name         string
	template     string
	protocols    []string // protocol fixture names (source="protocol")
	subscription []string // subscription fixture names (source="subscription")
}

var scenarios = []scenario{
	{name: "template-only-default", template: "fakeip"},
	{name: "protocol", template: "fakeip", protocols: []string{"proto-a"}},
	{name: "subscription", template: "fakeip", subscription: []string{"subA"}},
	{name: "mixed", template: "fakeip", protocols: []string{"proto-a", "proto-b"}, subscription: []string{"subA"}},
	{name: "country-edge", template: "fakeip", protocols: []string{"proto-c"}},
}

// loadFixtureNodes loads a testdata proxy fixture ({"outbounds":[...]}) and
// tags each node with the given source.
func loadFixtureNodes(t *testing.T, name, source string) []*Node {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	doc, err := ParseOrdered(data)
	if err != nil {
		t.Fatalf("parse fixture %s: %v", name, err)
	}
	raw, _ := doc.Get("outbounds")
	arr, ok := raw.([]any)
	if !ok {
		t.Fatalf("fixture %s has no outbounds array", name)
	}
	var nodes []*Node
	for _, ob := range arr {
		om, ok := ob.(*OrderedMap)
		if !ok {
			t.Fatalf("fixture %s outbound not an object", name)
		}
		nodes = append(nodes, &Node{Raw: om, Source: source})
	}
	return nodes
}

func loadTemplate(t *testing.T, name string) *OrderedMap {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "data", "templates", name+".json"))
	if err != nil {
		t.Fatalf("read template %s: %v", name, err)
	}
	cfg, err := ParseOrdered(data)
	if err != nil {
		t.Fatalf("parse template %s: %v", name, err)
	}
	return cfg
}

func runScenario(t *testing.T, s scenario) *OrderedMap {
	t.Helper()
	cfg := loadTemplate(t, s.template)
	var nodes []*Node
	for _, p := range s.protocols {
		nodes = append(nodes, loadFixtureNodes(t, p, "protocol")...)
	}
	for _, sub := range s.subscription {
		nodes = append(nodes, loadFixtureNodes(t, sub, "subscription")...)
	}
	out, err := Generate(cfg, nodes, Options{AutoCountryGroups: true})
	if err != nil {
		t.Fatalf("Generate(%s): %v", s.name, err)
	}
	return out
}

// TestRegression asserts Go output matches the JS-oracle goldens exactly
// (structural equality via canonical JSON), proving cross-language parity.
func TestRegression(t *testing.T) {
	for _, s := range scenarios {
		s := s
		t.Run(s.name, func(t *testing.T) {
			out := runScenario(t, s)
			got, err := json.MarshalIndent(json.RawMessage(mustMarshal(t, out)), "", "  ")
			if err != nil {
				t.Fatalf("marshal output: %v", err)
			}
			goldenPath := filepath.Join("testdata", "golden", s.name+".json")

			if *update {
				if err := os.WriteFile(goldenPath, append(got, '\n'), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}

			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden %s: %v", goldenPath, err)
			}
			// Compare canonically (parse both, re-marshal) to ignore trailing
			// whitespace and key-ordering differences from the JS pretty-print.
			if !canonicalEqual(t, got, want) {
				t.Errorf("scenario %s: Go output differs from JS golden.\nRun `node tests/generate_goldens.js` and inspect %s", s.name, goldenPath)
			}
		})
	}
}

func TestCountryHeatOrderOption(t *testing.T) {
	cfg := loadTemplate(t, "fakeip")
	nodes := loadFixtureNodes(t, "proto-a", "protocol")
	nodes = append(nodes, loadFixtureNodes(t, "proto-b", "protocol")...)
	nodes = append(nodes, loadFixtureNodes(t, "subA", "subscription")...)

	out, err := Generate(cfg, nodes, Options{
		AutoCountryGroups: true,
		CountryHeatOrder:  []string{"US", "SG", "CN"},
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	outbounds, err := configOutbounds(out)
	if err != nil {
		t.Fatalf("outbounds: %v", err)
	}
	indexes := map[string]int{}
	for i, ob := range outbounds {
		group, ok := ob.(*OrderedMap)
		if !ok || group.GetString("type") != "selector" {
			continue
		}
		indexes[group.GetString("tag")] = i
	}
	want := []string{"🇺🇸 United States", "🇸🇬 Singapore", "🇨🇳 China"}
	for _, tag := range want {
		if _, ok := indexes[tag]; !ok {
			t.Fatalf("missing selector %s in %v", tag, indexes)
		}
	}
	for i := 1; i < len(want); i++ {
		if indexes[want[i-1]] >= indexes[want[i]] {
			t.Fatalf("custom order not applied: %v", indexes)
		}
	}
}

func TestChainProxySelector(t *testing.T) {
	cfg := loadTemplate(t, "fakeip")
	nodes := loadFixtureNodes(t, "proto-a", "protocol")
	nodes = append(nodes, loadFixtureNodes(t, "proto-b", "protocol")...)
	chainTag := nodes[0].tag()
	for _, n := range nodes[1:] {
		n.Raw.Set("detour", chainTag)
	}

	out, err := Generate(cfg, nodes, Options{
		AutoCountryGroups: true,
		ChainProxy:        true,
		ChainProxyTag:     chainTag,
	})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	outbounds, err := configOutbounds(out)
	if err != nil {
		t.Fatalf("outbounds: %v", err)
	}
	chainSelector := findTestOutbound(outbounds, "🔗 Chain Proxy")
	if chainSelector == nil {
		t.Fatal("missing chain proxy selector")
	}
	for _, tag := range outboundTagsForTest(chainSelector) {
		if tag == chainTag {
			t.Fatalf("chain selector contains relay node tag %q", chainTag)
		}
	}
	proxy := findTestOutboundMatch(outbounds, "Proxy")
	if proxy == nil || !containsStringForTest(outboundTagsForTest(proxy), "🔗 Chain Proxy") {
		t.Fatalf("Proxy selector does not include chain selector: %+v", proxy)
	}
}

func mustMarshal(t *testing.T, m *OrderedMap) []byte {
	t.Helper()
	b, err := m.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal ordered map: %v", err)
	}
	return b
}

// canonicalEqual compares two JSON documents by structure (order-insensitive),
// matching the JS regression test's assert.deepStrictEqual semantics.
func canonicalEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("unmarshal go output: %v", err)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	ca, _ := json.Marshal(av)
	cb, _ := json.Marshal(bv)
	return bytes.Equal(ca, cb)
}

func findTestOutbound(outbounds []any, tag string) *OrderedMap {
	for _, ob := range outbounds {
		om, ok := ob.(*OrderedMap)
		if ok && om.GetString("tag") == tag {
			return om
		}
	}
	return nil
}

func findTestOutboundMatch(outbounds []any, pattern string) *OrderedMap {
	for _, ob := range outbounds {
		om, ok := ob.(*OrderedMap)
		if ok && matchTag(om.GetString("tag"), pattern) {
			return om
		}
	}
	return nil
}

func outboundTagsForTest(ob *OrderedMap) []string {
	raw, ok := ob.Get("outbounds")
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func containsStringForTest(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
