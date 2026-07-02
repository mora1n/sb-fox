package merge

import "testing"

// TestExtractCountry pins every detection branch, including the tricky ranking
// cases from the country-edge regression scenario.
func TestExtractCountry(t *testing.T) {
	cases := []struct {
		tag  string
		want string // "" means no match (unrecognized)
	}{
		// ranking edge cases (from country-edge-linux)
		{"HKHK", ""},          // no boundary match -> Others
		{"HongKong HK", "HK"}, // "hongkong" alias
		{"CN2-US", "US"},      // US matched (boundary), CN2 not a CN boundary
		{"Tokyo JP 01", "JP"}, // "jp" code with boundaries
		{"Germany-US", "DE"},  // "germany" name alias is longer/higher-ranked than "us"
		{"US-Relay", "US"},
		// exact alias on stripped tag
		{"日本", "JP"},
		{"香港", "HK"},
		{"新加坡", "SG"},
		// emoji flag detection
		{"🇭🇰 Premium", "HK"},
		{"🇺🇸", "US"},
		// code aliases with boundaries
		{"Node JP", "JP"},
		{"SG-01", "SG"},
		// name aliases
		{"Singapore Premium", "SG"},
		{"Britain 1", "GB"}, // "britain" alias; note "united kingdom" is NOT an alias
		{"UK-1", "GB"},
		// no match
		{"Mystery-1", ""},
		{"Unknown Relay", ""},
		{"", ""},
	}

	for _, c := range cases {
		got := extractCountry(c.tag)
		gotCode := ""
		if got != nil {
			gotCode = got.Code
		}
		if gotCode != c.want {
			t.Errorf("extractCountry(%q) = %q, want %q", c.tag, gotCode, c.want)
		}
	}
}

// TestServerCountryOverride covers the `server#CC` suffix (requirement h).
func TestServerCountryOverride(t *testing.T) {
	cases := []struct {
		server     string
		wantServer string
		wantCode   string
		wantNil    bool
	}{
		{"relay.example.com#CN", "relay.example.com", "CN", false},
		{"1.2.3.4#us", "1.2.3.4", "US", false},
		{"plain.example.com", "", "", true},
		{"#CN", "", "", true}, // empty server part
		{"host#ABC", "", "", true}, // 3 letters, not a 2-letter code
	}
	for _, c := range cases {
		ov := ExtractServerCountryOverride(c.server)
		if c.wantNil {
			if ov != nil {
				t.Errorf("ExtractServerCountryOverride(%q) = %+v, want nil", c.server, ov)
			}
			continue
		}
		if ov == nil {
			t.Errorf("ExtractServerCountryOverride(%q) = nil, want {%q,%q}", c.server, c.wantServer, c.wantCode)
			continue
		}
		if ov.Server != c.wantServer || ov.CountryCode != c.wantCode {
			t.Errorf("ExtractServerCountryOverride(%q) = {%q,%q}, want {%q,%q}", c.server, ov.Server, ov.CountryCode, c.wantServer, c.wantCode)
		}
	}
}

// TestManualCountryOverride verifies a pre-set CountryOverride wins over the tag.
func TestManualCountryOverride(t *testing.T) {
	n := &Node{Raw: NewOrderedMap(), CountryOverride: "JP"}
	n.Raw.Set("tag", "🇺🇸 Looks American")
	info := n.resolveCountry()
	if info == nil || info.Code != "JP" {
		t.Fatalf("manual override JP not honored, got %+v", info)
	}
}

// TestBuildFlagEmoji covers the regional-indicator math.
func TestBuildFlagEmoji(t *testing.T) {
	if got := buildFlagEmoji("US"); got != "🇺🇸" {
		t.Errorf("buildFlagEmoji(US) = %q", got)
	}
	if got := buildFlagEmoji("xx"); got != "🏳️" {
		t.Errorf("buildFlagEmoji(xx) = %q, want white flag", got)
	}
}

// TestStripEmoji covers the leading-emoji removal used by matchTag.
func TestStripEmoji(t *testing.T) {
	cases := map[string]string{
		"🚀 Proxy":       "Proxy",
		"🏠 Mainland":    "Mainland",
		"🏳️‍🌈 Others":    "Others",
		"Plain":         "Plain",
		"  Trimmed  ":   "Trimmed",
		"🇭🇰 Hong Kong": "Hong Kong",
	}
	for in, want := range cases {
		if got := stripEmoji(in); got != want {
			t.Errorf("stripEmoji(%q) = %q, want %q", in, got, want)
		}
	}
}
