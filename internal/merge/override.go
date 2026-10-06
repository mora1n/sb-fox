package merge

import (
	"regexp"
	"strings"
)

// ServerCountryOverride is the parsed result of a `server#CC` suffix.
type ServerCountryOverride struct {
	Server      string
	CountryCode string
}

// CustomCountryCode is an explicit category, never inferred from a node name.
const CustomCountryCode = "CUSTOM"

var reServerOverride = regexp.MustCompile(`(?i)^(.*)#([a-z]{2}|CUSTOM)$`)

// ExtractServerCountryOverride ports extractServerCountryOverride: a server
// value like "relay.example.com#CN" yields {Server:"relay.example.com",
// CountryCode:"CN"}. The explicit #CUSTOM category is also supported.
// Returns nil when there is no valid suffix.
//
// Exported so the sblink import path can apply the same tag-identification
// precedence (requirement h: `server#CC` overrides name-based detection).
func ExtractServerCountryOverride(server string) *ServerCountryOverride {
	m := reServerOverride.FindStringSubmatch(strings.TrimSpace(server))
	if m == nil {
		return nil
	}
	srv := strings.TrimSpace(m[1])
	if srv == "" {
		return nil
	}
	return &ServerCountryOverride{Server: srv, CountryCode: strings.ToUpper(m[2])}
}

// resolveCountryOverride ports resolveCountryOverride for a string override
// (the only form produced internally): try ISO code first, else alias extract.
func resolveCountryOverride(code string) *CountryInfo {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	if strings.EqualFold(code, CustomCountryCode) {
		return &CountryInfo{Code: CustomCountryCode, Name: "Custom", Emoji: "🏳️‍🌈"}
	}
	if info := getCountryInfoByCode(code); info != nil {
		return info
	}
	return extractCountry(code)
}
