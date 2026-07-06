package sblink

import (
	"net/url"

	"github.com/mora1n/sb-fox/internal/merge"
)

func parseHysteria(uri string) (*merge.OrderedMap, error) {
	p, err := parseURILinkWithDefault(uri, 443)
	if err != nil {
		return nil, err
	}

	out := merge.NewOrderedMap()
	out.Set("type", "hysteria")
	out.Set("tag", p.tag)
	out.Set("server", p.server)
	out.Set("server_port", p.portN)
	if ports := queryFirst(p.query, "mport", "server_ports", "server-ports"); ports != "" {
		out.Set("server_ports", []any{ports})
	}
	setStringIfPresent(out, "hop_interval", queryFirst(p.query, "hop_interval", "hop-interval"))
	setStringIfPresent(out, "up", queryFirst(p.query, "up"))
	setStringIfPresent(out, "down", queryFirst(p.query, "down"))
	if err := setIntIfPresent(out, "up_mbps", queryFirst(p.query, "up_mbps", "up-mbps", "upmbps")); err != nil {
		return nil, err
	}
	if err := setIntIfPresent(out, "down_mbps", queryFirst(p.query, "down_mbps", "down-mbps", "downmbps")); err != nil {
		return nil, err
	}
	setStringIfPresent(out, "obfs", queryFirst(p.query, "obfs", "obfsParam", "obfs-param"))
	setStringIfPresent(out, "auth_str", queryFirst(p.query, "auth_str", "auth-str", "auth"))
	setStringIfPresent(out, "network", queryFirst(p.query, "protocol", "network"))
	if err := setIntIfPresent(out, "recv_window_conn", queryFirst(p.query, "recv_window_conn", "recv-window-conn")); err != nil {
		return nil, err
	}
	if err := setIntIfPresent(out, "recv_window", queryFirst(p.query, "recv_window", "recv-window")); err != nil {
		return nil, err
	}
	if boolQuery(p.query, "disable_mtu_discovery", "disable-mtu-discovery") {
		out.Set("disable_mtu_discovery", true)
	}
	tls, err := tlsFromQuery(p.query, true, false)
	if err != nil {
		return nil, err
	}
	out.Set("tls", tls)
	return out, nil
}

func encodeHysteria(out *merge.OrderedMap) (string, error) {
	server, port, err := requiredEndpoint(out)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	if ports := firstStringField(out, "server_ports"); ports != "" {
		q.Set("mport", ports)
	}
	for _, item := range []struct {
		field string
		query string
	}{
		{"hop_interval", "hop_interval"},
		{"up", "up"},
		{"down", "down"},
		{"up_mbps", "up_mbps"},
		{"down_mbps", "down_mbps"},
		{"obfs", "obfs"},
		{"auth_str", "auth"},
		{"network", "protocol"},
		{"recv_window_conn", "recv-window-conn"},
		{"recv_window", "recv-window"},
	} {
		if value, ok := out.Get(item.field); ok {
			q.Set(item.query, scalarString(value))
		}
	}
	if truthyField(out, "disable_mtu_discovery") {
		q.Set("disable-mtu-discovery", "1")
	}
	if err := addTLSQuery(q, out, false, false); err != nil {
		return "", err
	}
	return linkURL("hysteria", nil, server, port, q, out.GetString("tag")), nil
}
