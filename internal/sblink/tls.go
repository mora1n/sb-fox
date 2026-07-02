package sblink

import (
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// tlsParams collects the TLS-related inputs gathered from a link's query.
type tlsParams struct {
	enabled     bool
	serverName  string
	insecure    bool
	alpn        []string
	fingerprint string
	realityPbk  string
	realitySid  string
}

// splitALPN parses a comma-separated alpn value into individual protocols.
func splitALPN(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildTLS renders a sing-box tls block, or nil when TLS is not enabled.
// Field order: enabled, server_name, insecure, alpn, utls, reality.
func buildTLS(p tlsParams) *merge.OrderedMap {
	if !p.enabled {
		return nil
	}
	tls := merge.NewOrderedMap()
	tls.Set("enabled", true)
	if p.serverName != "" {
		tls.Set("server_name", p.serverName)
	}
	if p.insecure {
		tls.Set("insecure", true)
	}
	if len(p.alpn) > 0 {
		arr := make([]any, 0, len(p.alpn))
		for _, a := range p.alpn {
			arr = append(arr, a)
		}
		tls.Set("alpn", arr)
	}
	if p.fingerprint != "" {
		utls := merge.NewOrderedMap()
		utls.Set("enabled", true)
		utls.Set("fingerprint", p.fingerprint)
		tls.Set("utls", utls)
	}
	if p.realityPbk != "" {
		reality := merge.NewOrderedMap()
		reality.Set("enabled", true)
		reality.Set("public_key", p.realityPbk)
		if p.realitySid != "" {
			reality.Set("short_id", p.realitySid)
		}
		tls.Set("reality", reality)
	}
	return tls
}
