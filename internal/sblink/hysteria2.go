package sblink

import (
	"github.com/mora1n/sb-fox/internal/merge"
)

// parseHysteria2 maps a hysteria2:// (or hy2://) body — with the scheme prefix
// already stripped — to a sing-box hysteria2 outbound. A normalized scheme is
// re-added so url.Parse handles userinfo/host/port/query uniformly.
func parseHysteria2(body string) (*merge.OrderedMap, error) {
	p, err := parseURILink("hysteria2://" + body)
	if err != nil {
		return nil, err
	}
	password := ""
	if p.user != nil {
		password = p.user.Username()
		if pw, ok := p.user.Password(); ok && pw != "" {
			// Some clients put the auth string as user:pass; join them.
			password = password + ":" + pw
		}
	}

	out := merge.NewOrderedMap()
	out.Set("type", "hysteria2")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	out.Set("password", password)

	if obfs := p.query.Get("obfs"); obfs != "" {
		o := merge.NewOrderedMap()
		o.Set("type", obfs)
		if pw := p.query.Get("obfs-password"); pw != "" {
			o.Set("password", pw)
		}
		out.Set("obfs", o)
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
