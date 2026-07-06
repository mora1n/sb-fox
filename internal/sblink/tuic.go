package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// parseTUIC maps a tuic:// URI to a sing-box tuic outbound. The userinfo
// carries "uuid:password".
func parseTUIC(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILinkWithDefault(uri, 443)
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
	if cc := queryFirst(p.query, "congestion_control", "congestion-control", "congestion-controller"); cc != "" {
		out.Set("congestion_control", cc)
	}
	if urm := queryFirst(p.query, "udp_relay_mode", "udp-relay-mode"); urm != "" {
		out.Set("udp_relay_mode", urm)
	}
	setStringIfPresent(out, "heartbeat", queryFirst(p.query, "heartbeat", "heartbeat_interval", "heartbeat-interval"))
	setStringIfPresent(out, "network", queryFirst(p.query, "network"))

	tls := buildTLS(tlsParams{
		enabled:    true,
		serverName: queryFirst(p.query, "sni", "peer"),
		insecure:   boolQuery(p.query, "insecure", "allowInsecure", "allow-insecure", "skip-cert-verify"),
		alpn:       splitALPN(p.query.Get("alpn")),
	})
	if tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}
