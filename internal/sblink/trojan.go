package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// parseTrojan maps a trojan:// URI to a sing-box trojan outbound. Trojan
// implies TLS, so the tls block is always enabled.
func parseTrojan(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILink(uri)
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

	tls := buildTLS(tlsParams{
		enabled:     true,
		serverName:  p.query.Get("sni"),
		insecure:    boolParam(p.query.Get("insecure")),
		alpn:        splitALPN(p.query.Get("alpn")),
		fingerprint: p.query.Get("fp"),
	})
	if tls != nil {
		out.Set("tls", tls)
	}
	if t := transportFromQuery(p.query); t != nil {
		out.Set("transport", t)
	}
	return out, nil
}
