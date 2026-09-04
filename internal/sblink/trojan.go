package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// parseTrojan maps a trojan:// URI to a sing-box trojan outbound. Trojan
// implies TLS, so the tls block is always enabled.
func parseTrojan(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILinkWithDefault(uri, 443)
	if err != nil {
		return nil, err
	}
	password := ""
	if p.user != nil {
		password = p.user.Username()
	}

	out := merge.NewOrderedMap()
	out.Set("type", "trojan")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("password", password)
	setStringIfPresent(out, "network", queryFirst(p.query, "network"))

	tls := buildTLS(tlsParams{
		enabled:     true,
		serverName:  queryFirst(p.query, "sni", "peer"),
		insecure:    boolQuery(p.query, "insecure", "allowInsecure", "allow-insecure", "skip-cert-verify"),
		alpn:        splitALPN(p.query.Get("alpn")),
		fingerprint: queryFirst(p.query, "fp", "fingerprint", "client-fingerprint"),
	})
	if tls != nil {
		out.Set("tls", tls)
	}
	if t := transportFromQuery(p.query); t != nil {
		out.Set("transport", t)
	}
	return out, nil
}
