package sblink

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

// parseShadowsocks handles both legacy (whole tail base64) and SIP002 forms.
func parseShadowsocks(uri string) (*merge.OrderedMap, error) {
	body := strings.TrimPrefix(uri, "ss://")

	fragment := ""
	if i := strings.Index(body, "#"); i >= 0 {
		fragment = body[i+1:]
		body = body[:i]
	}
	query := url.Values{}
	if i := strings.Index(body, "?"); i >= 0 {
		parsed, err := url.ParseQuery(body[i+1:])
		if err != nil {
			return nil, fmt.Errorf("sblink: ss query: %w", err)
		}
		query = parsed
	}

	// Legacy form: the entire tail (before #) is base64 of
	// "method:password@host:port". Detect by absence of '@'.
	if !strings.Contains(body, "@") {
		if dec, ok := decodeBase64(body); ok {
			body = string(dec)
		}
	}

	method, password, hostport, plugin, pluginOpts, err := splitSSBody(body)
	if err != nil {
		return nil, err
	}

	server, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	// Hosts may be percent-encoded (e.g. a literal `#CC` override written as
	// %23CC); decode before cleaning so ExtractServerCountryOverride can match.
	if unescaped, err := url.QueryUnescape(server); err == nil {
		server = unescaped
	}
	server = cleanServer(server)
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}

	out := merge.NewOrderedMap()
	out.Set("type", "shadowsocks")
	out.Set("tag", tagOrDefault(fragment, server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	out.Set("method", method)
	out.Set("password", password)
	if plugin != "" {
		out.Set("plugin", plugin)
		if pluginOpts != "" {
			out.Set("plugin_opts", pluginOpts)
		}
	}
	setStringIfPresent(out, "network", queryFirst(query, "network"))
	if value := queryFirst(query, "uot", "udp_over_tcp", "udp-over-tcp"); value != "" || boolQuery(query, "uot", "udp_over_tcp", "udp-over-tcp") {
		setUDPOverTCP(out, value)
	}
	if rawMultiplex := query.Get("multiplex"); rawMultiplex != "" {
		multiplex, err := merge.ParseOrdered([]byte(rawMultiplex))
		if err != nil {
			return nil, fmt.Errorf("sblink: shadowsocks multiplex JSON: %w", err)
		}
		out.Set("multiplex", multiplex)
	}
	return out, nil
}

// splitSSBody extracts method, password, host:port and plugin options from the
// (already fragment-stripped, possibly legacy-decoded) ss body.
func splitSSBody(body string) (method, password, hostport, plugin, pluginOpts string, err error) {
	// Separate optional query (SIP002 plugin) from the "?..." tail.
	query := ""
	if i := strings.Index(body, "?"); i >= 0 {
		query = body[i+1:]
		body = body[:i]
	}
	// A trailing "/" before the query is allowed by SIP002.
	body = strings.TrimSuffix(body, "/")

	at := strings.LastIndex(body, "@")
	if at < 0 {
		return "", "", "", "", "", fmt.Errorf("sblink: ss missing '@' in %q", body)
	}
	userinfo := body[:at]
	hostport = body[at+1:]

	method, password, err = decodeUserinfo(userinfo)
	if err != nil {
		return "", "", "", "", "", err
	}

	plugin, pluginOpts = parseSSPlugin(query)
	return method, password, hostport, plugin, pluginOpts, nil
}

// decodeUserinfo returns method:password from the userinfo segment, which may
// be base64url-encoded or plain "method:password".
func decodeUserinfo(userinfo string) (method, password string, err error) {
	creds := userinfo
	if !strings.Contains(userinfo, ":") {
		if dec, ok := decodeBase64(userinfo); ok {
			creds = string(dec)
		}
	}
	i := strings.Index(creds, ":")
	if i < 0 {
		return "", "", fmt.Errorf("sblink: ss userinfo missing ':' in %q", userinfo)
	}
	return creds[:i], creds[i+1:], nil
}

// parseSSPlugin extracts sing-box plugin and plugin_opts from a SIP002 query.
// The plugin value is like "obfs-local;obfs=http;obfs-host=example.com".
func parseSSPlugin(query string) (plugin, pluginOpts string) {
	if query == "" {
		return "", ""
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return "", ""
	}
	raw := values.Get("plugin")
	if raw == "" {
		return "", ""
	}
	if i := strings.Index(raw, ";"); i >= 0 {
		return raw[:i], raw[i+1:]
	}
	return raw, ""
}

// splitHostPort splits "host:port", tolerating IPv6 bracketed hosts.
func splitHostPort(hostport string) (host, port string, err error) {
	hostport = strings.TrimSpace(hostport)
	if strings.HasPrefix(hostport, "[") {
		end := strings.Index(hostport, "]")
		if end < 0 {
			return "", "", fmt.Errorf("sblink: bad IPv6 host %q", hostport)
		}
		host = hostport[1:end]
		rest := hostport[end+1:]
		if strings.HasPrefix(rest, ":") {
			port = rest[1:]
		}
		return host, port, nil
	}
	i := strings.LastIndex(hostport, ":")
	if i < 0 {
		return "", "", fmt.Errorf("sblink: missing port in %q", hostport)
	}
	return hostport[:i], hostport[i+1:], nil
}
