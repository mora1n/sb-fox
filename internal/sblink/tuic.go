package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// parseTUIC maps a tuic:// URI to a sing-box tuic outbound. The userinfo
// carries "uuid:password".
func parseTUIC(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILink(uri)
	if err != nil {
		return nil, err
	}
	uuid, password := "", ""
	if p.user != nil {
		uuid = p.user.Username()
		if pw, ok := p.user.Password(); ok {
			password = pw
		}
	}

	out := merge.NewOrderedMap()
	out.Set("type", "tuic")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("uuid", uuid)
	out.Set("password", password)
	if cc := p.query.Get("congestion_control"); cc != "" {
		out.Set("congestion_control", cc)
	}
	if urm := p.query.Get("udp_relay_mode"); urm != "" {
		out.Set("udp_relay_mode", urm)
	}

	tls := buildTLS(tlsParams{
		enabled:    true,
		serverName: p.query.Get("sni"),
		insecure:   boolParam(p.query.Get("insecure")),
		alpn:       splitALPN(p.query.Get("alpn")),
	})
	if tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}
