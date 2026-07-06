package sblink

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

func parseNaive(uri string) (*merge.OrderedMap, error) {
	raw := strings.TrimSpace(uri)
	raw = strings.TrimPrefix(raw, "naive+")
	if strings.HasPrefix(raw, "naive://") {
		raw = "https://" + strings.TrimPrefix(raw, "naive://")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("sblink: bad naive URI %q: %w", uri, err)
	}
	server := cleanServer(u.Hostname())
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		default:
			port = strconv.Itoa(443)
		}
	}
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}
	username := ""
	password := ""
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	q := u.Query()

	out := merge.NewOrderedMap()
	out.Set("type", "naive")
	out.Set("tag", tagOrDefault(u.Fragment, server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	if username != "" {
		out.Set("username", username)
	}
	if password != "" {
		out.Set("password", password)
	}
	if q.Get("quic") == "1" || q.Get("quic") == "true" || u.Scheme == "quic" {
		out.Set("quic", true)
	}
	if cc := q.Get("quic_congestion_control"); cc != "" {
		out.Set("quic_congestion_control", cc)
	}
	tls := buildTLS(tlsParams{
		enabled:    u.Scheme == "https" || u.Scheme == "quic" || q.Get("security") == "tls",
		serverName: queryFirst(q, "sni", "peer"),
		insecure:   boolQuery(q, "insecure", "allowInsecure", "allow-insecure", "skip-cert-verify"),
		alpn:       splitALPN(q.Get("alpn")),
	})
	if tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}
