package sblink

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
)

func applyVMessTransport(v *vmessLink, transport *merge.OrderedMap) error {
	switch typ := transport.GetString("type"); typ {
	case "", "tcp":
		return nil
	case "ws":
		v.Net = "ws"
		v.Path = transport.GetString("path")
		v.Host = transportHost(transport)
	case "grpc":
		v.Net = "grpc"
		v.Path = transport.GetString("service_name")
	case "http", "h2":
		v.Net = "h2"
		v.Path = transport.GetString("path")
		v.Host = firstStringField(transport, "host")
	default:
		return fmt.Errorf("sblink: vmess transport %q cannot be exported", typ)
	}
	return nil
}

func addTLSQuery(q url.Values, out *merge.OrderedMap, allowReality, includeSecurity bool) error {
	tls, ok := orderedField(out, "tls")
	if !ok || !tlsEnabled(tls) {
		return nil
	}
	security := "tls"
	if reality, ok := orderedField(tls, "reality"); ok && reality.GetString("public_key") != "" {
		if !allowReality {
			return fmt.Errorf("sblink: reality TLS cannot be exported for %s", out.GetString("type"))
		}
		security = "reality"
		q.Set("pbk", reality.GetString("public_key"))
		if sid := reality.GetString("short_id"); sid != "" {
			q.Set("sid", sid)
		}
	}
	if includeSecurity {
		q.Set("security", security)
	}
	if sni := tls.GetString("server_name"); sni != "" {
		q.Set("sni", sni)
	}
	if truthyField(tls, "insecure") {
		q.Set("insecure", "1")
	}
	if alpn := strings.Join(stringListField(tls, "alpn"), ","); alpn != "" {
		q.Set("alpn", alpn)
	}
	if utls, ok := orderedField(tls, "utls"); ok {
		if fp := utls.GetString("fingerprint"); fp != "" && includeSecurity {
			q.Set("fp", fp)
		}
	}
	return nil
}

func addTransportQuery(q url.Values, out *merge.OrderedMap) error {
	transport, ok := orderedField(out, "transport")
	if !ok {
		return nil
	}
	switch typ := transport.GetString("type"); typ {
	case "", "tcp":
		return nil
	case "ws":
		q.Set("type", "ws")
		if path := transport.GetString("path"); path != "" {
			q.Set("path", path)
		}
		if host := transportHost(transport); host != "" {
			q.Set("host", host)
		}
	case "grpc":
		q.Set("type", "grpc")
		if service := transport.GetString("service_name"); service != "" {
			q.Set("serviceName", service)
		}
	case "http", "h2":
		q.Set("type", "http")
		if path := transport.GetString("path"); path != "" {
			q.Set("path", path)
		}
		if host := firstStringField(transport, "host"); host != "" {
			q.Set("host", host)
		}
	case "httpupgrade":
		q.Set("type", "httpupgrade")
		if path := transport.GetString("path"); path != "" {
			q.Set("path", path)
		}
		if host := transport.GetString("host"); host != "" {
			q.Set("host", host)
		}
	default:
		return fmt.Errorf("sblink: transport %q cannot be exported", typ)
	}
	return nil
}

func requiredEndpoint(m *merge.OrderedMap) (string, string, error) {
	server, err := requiredString(m, "server")
	if err != nil {
		return "", "", err
	}
	server = cleanServer(server)
	port, err := requiredPort(m)
	if err != nil {
		return "", "", err
	}
	return server, port, nil
}

func requiredString(m *merge.OrderedMap, key string) (string, error) {
	value := m.GetString(key)
	if value == "" {
		return "", fmt.Errorf("sblink: missing %s", key)
	}
	return value, nil
}

func requiredPort(m *merge.OrderedMap) (string, error) {
	value, ok := m.Get("server_port")
	if !ok {
		return "", fmt.Errorf("sblink: missing server_port")
	}
	port := scalarString(value)
	if port == "" {
		return "", fmt.Errorf("sblink: missing server_port")
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		return "", fmt.Errorf("sblink: invalid server_port %q: %w", port, err)
	}
	return strconv.Itoa(n), nil
}

func linkURL(scheme string, user *url.Userinfo, server, port string, q url.Values, tag string) string {
	return linkURLWithPath(scheme, user, server, port, "", q, tag)
}

func linkURLWithPath(scheme string, user *url.Userinfo, server, port, path string, q url.Values, tag string) string {
	u := &url.URL{
		Scheme:   scheme,
		User:     user,
		Host:     net.JoinHostPort(server, port),
		Path:     path,
		RawQuery: q.Encode(),
		Fragment: tag,
	}
	return u.String()
}

func orderedField(m *merge.OrderedMap, key string) (*merge.OrderedMap, bool) {
	value, ok := m.Get(key)
	if !ok {
		return nil, false
	}
	om, ok := value.(*merge.OrderedMap)
	return om, ok
}

func tlsEnabled(tls *merge.OrderedMap) bool {
	if value, ok := tls.Get("enabled"); ok {
		return truthy(value)
	}
	return true
}

func truthyField(m *merge.OrderedMap, key string) bool {
	value, ok := m.Get(key)
	return ok && truthy(value)
}

func truthy(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return boolParam(v)
	case json.Number:
		return v.String() != "0"
	case float64:
		return v != 0
	default:
		return false
	}
}

func scalarString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func stringListField(m *merge.OrderedMap, key string) []string {
	value, ok := m.Get(key)
	if !ok {
		return nil
	}
	switch v := value.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s := scalarString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

func firstStringField(m *merge.OrderedMap, key string) string {
	values := stringListField(m, key)
	if len(values) > 0 {
		return values[0]
	}
	return m.GetString(key)
}

func transportHost(transport *merge.OrderedMap) string {
	headers, ok := orderedField(transport, "headers")
	if !ok {
		return ""
	}
	if host := headers.GetString("Host"); host != "" {
		return host
	}
	return headers.GetString("host")
}
