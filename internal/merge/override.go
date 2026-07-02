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

var reServerOverride = regexp.MustCompile(`^(.*)#([A-Za-z]{2})$`)

// ExtractServerCountryOverride ports extractServerCountryOverride: a server
// value like "relay.example.com#CN" yields {Server:"relay.example.com",
// CountryCode:"CN"}. Returns nil when there is no valid suffix.
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
	if info := getCountryInfoByCode(code); info != nil {
		return info
	}
	return extractCountry(code)
}
