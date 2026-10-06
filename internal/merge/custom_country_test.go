package merge

import (
	"reflect"
	"testing"
)

func customCountryTestNode(tag, country string) *Node {
	raw := NewOrderedMap()
	raw.Set("type", "socks")
	raw.Set("tag", tag)
	raw.Set("server", "example.com")
	raw.Set("server_port", jsonInt(1080))
	return &Node{Raw: raw, Source: "manual", CountryOverride: country}
}

func TestCustomCountryGroupMembershipAndReferences(t *testing.T) {
	cfg, err := ParseOrdered([]byte(`{"outbounds":[
		{"type":"selector","tag":"Proxy","outbounds":[]},
		{"type":"urltest","tag":"Auto","outbounds":[]},
		{"type":"selector","tag":"Fallback","outbounds":[]},
		{"type":"urltest","tag":"Checks","outbounds":[]},
		{"type":"selector","tag":"Others","outbounds":["Direct"]},
		{"type":"selector","tag":"Mainland","outbounds":["Direct"]},
		{"type":"direct","tag":"Direct"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	nodes := []*Node{customCountryTestNode("🇺🇸 mystery-one", "CUSTOM"), customCountryTestNode("plain-node", "JP"), customCountryTestNode("mystery-two", "CUSTOM"), customCountryTestNode("unknown-node", "")}
	out, err := Generate(cfg, nodes, Options{AutoCountryGroups: true, ChainProxy: true})
	if err != nil {
		t.Fatal(err)
	}
	outbounds, err := configOutbounds(out)
	if err != nil {
		t.Fatal(err)
	}
	assertCustomCountryTags(t, outbounds, "🏳️‍🌈Custom", []string{"🇺🇸 mystery-one", "mystery-two"})
	assertCustomCountryTags(t, outbounds, "🏴‍☠️Others", []string{"unknown-node"})
	assertCustomCountryTags(t, outbounds, "🇯🇵Japan", []string{"plain-node"})
	for _, tag := range []string{"Fallback", "Checks", ChainProxyTag} {
		assertCustomCountryTags(t, outbounds, tag, []string{"🇯🇵Japan", "🏳️‍🌈Custom", "🏴‍☠️Others"})
	}
	for _, tag := range []string{"Proxy", "Auto"} {
		assertCustomCountryTags(t, outbounds, tag, []string{"🇯🇵Japan", "🏳️‍🌈Custom", "🏴‍☠️Others", ChainProxyTag})
	}
	for _, tag := range []string{"Others", "Mainland"} {
		assertCustomCountryTags(t, outbounds, tag, []string{"Direct"})
	}
	var inserted []string
	for _, ob := range outbounds {
		tag := ob.(*OrderedMap).GetString("tag")
		if tag == "🇯🇵Japan" || tag == "🏳️‍🌈Custom" || tag == ChainProxyTag || tag == "Direct" {
			inserted = append(inserted, tag)
		}
	}
	if !reflect.DeepEqual(inserted, []string{"🇯🇵Japan", "🏳️‍🌈Custom", ChainProxyTag, "Direct"}) {
		t.Fatalf("inserted group order = %v", inserted)
	}
}

func TestCustomCountryGroupSourceAndToggle(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts Options
		want []string
	}{
		{"custom-only", Options{AutoCountryGroups: true}, []string{"mystery-one", "mystery-two"}},
		{"unknown-only", Options{AutoCountryGroups: true}, nil},
		{"known-only", Options{AutoCountryGroups: true}, nil},
		{"selected-source", Options{AutoCountryGroups: true, CountryGroupSourceTags: []string{"mystery-two"}}, []string{"mystery-two"}},
		{"empty-source", Options{AutoCountryGroups: true, CountryGroupSourceTags: []string{}}, nil},
		{"known-source", Options{AutoCountryGroups: true, CountryGroupSourceTags: []string{"plain-node"}}, nil},
		{"disabled", Options{AutoCountryGroups: false}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseOrdered([]byte(`{"outbounds":[{"type":"selector","tag":"Fallback","outbounds":[]}]}`))
			if err != nil {
				t.Fatal(err)
			}
			nodes := []*Node{customCountryTestNode("mystery-one", "CUSTOM"), customCountryTestNode("mystery-two", "CUSTOM")}
			if tc.name == "unknown-only" {
				nodes = []*Node{customCountryTestNode("mystery-one", ""), customCountryTestNode("🏳️‍🌈Custom node", "")}
			}
			if tc.name == "known-only" {
				nodes = []*Node{customCountryTestNode("plain-node", "JP")}
			}
			if tc.name == "known-source" {
				nodes = append(nodes, customCountryTestNode("plain-node", "JP"))
			}
			out, err := Generate(cfg, nodes, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			outbounds, err := configOutbounds(out)
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.want) == 0 {
				if findTestOutbound(outbounds, "🏳️‍🌈Custom") != nil {
					t.Fatal("unexpected Custom selector")
				}
				return
			}
			assertCustomCountryTags(t, outbounds, "🏳️‍🌈Custom", tc.want)
			assertCustomCountryTags(t, outbounds, "Fallback", []string{"🏳️‍🌈Custom"})
			if findTestOutbound(outbounds, "🏴‍☠️Others") != nil {
				t.Fatal("unexpected unrecognized-node selector for explicitly assigned countries")
			}
		})
	}
}

func TestCustomCountryRequiresExplicitSelection(t *testing.T) {
	for _, tag := range []string{"Custom", "🏳️‍🌈Custom node", "自定义"} {
		if info := DetectCountry(tag); info != nil {
			t.Fatalf("custom category inferred from name %q: %+v", tag, info)
		}
	}
	node := customCountryTestNode("🇯🇵 Japan", " custom ")
	node.Source = "protocol"
	node.Raw.Set("server", "example.com#US")
	node.applySourceTagging()
	info := node.resolveCountry()
	if info == nil || info.Code != CustomCountryCode || info.Emoji != "🏳️‍🌈" || info.Name != "Custom" {
		t.Fatalf("manual custom override not honored: %+v", info)
	}
	if node.server() != "example.com" {
		t.Fatalf("country annotation remains in server: %q", node.server())
	}
}

func assertCustomCountryTags(t *testing.T, outbounds []any, tag string, want []string) {
	t.Helper()
	group := findTestOutbound(outbounds, tag)
	if group == nil {
		t.Fatalf("missing group %q", tag)
	}
	if got := outboundTagsForTest(group); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s outbounds = %v, want %v", tag, got, want)
	}
}
