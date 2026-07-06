package sblink

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mora1n/sb-fox/internal/merge"
	"gopkg.in/yaml.v3"
)

type clashDocument struct {
	Proxies []map[string]any `yaml:"proxies"`
}

func parseClashYAML(text string) ([]*merge.OrderedMap, []string, error) {
	var doc clashDocument
	text = normalizeClashYAML(text)
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, nil, err
	}
	if len(doc.Proxies) == 0 {
		return nil, nil, fmt.Errorf("sblink: clash/mihomo YAML has no proxies")
	}
	var out []*merge.OrderedMap
	var warnings []string
	for i, proxy := range doc.Proxies {
		parsed, err := clashProxyOutbound(proxy)
		if err != nil {
			name := clashString(proxy, "name")
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		out = append(out, parsed)
	}
	if len(out) == 0 {
		return nil, warnings, fmt.Errorf("sblink: no supported clash/mihomo proxies parsed")
	}
	return out, warnings, nil
}

var clashShortIDLine = regexp.MustCompile(`(?m)^(\s*short-id:\s*)([^#\n]*?)(\s*(?:#.*)?)$`)

func normalizeClashYAML(text string) string {
	if !strings.Contains(text, "short-id:") {
		return text
	}
	return clashShortIDLine.ReplaceAllStringFunc(text, func(line string) string {
		parts := clashShortIDLine.FindStringSubmatch(line)
		if len(parts) != 4 {
			return line
		}
		value := strings.TrimSpace(parts[2])
		if value == "" {
			return parts[1] + `""` + parts[3]
		}
		if strings.HasPrefix(value, `"`) || strings.HasPrefix(value, `'`) || value == "null" {
			return line
		}
		return parts[1] + strconv.Quote(value) + parts[3]
	})
}

func clashProxyOutbound(p map[string]any) (*merge.OrderedMap, error) {
	typ := strings.ToLower(clashString(p, "type"))
	switch typ {
	case "ss", "shadowsocks":
		return clashShadowsocks(p)
	case "vmess":
		return clashVMess(p)
	case "vless":
		return clashVLESS(p)
	case "trojan":
		return clashTrojan(p)
	case "hysteria2", "hy2", "hysteria":
		return clashHysteria2(p)
	case "tuic":
		return clashTUIC(p)
	case "naive":
		return clashNaive(p)
	case "ssr":
		return nil, fmt.Errorf("ssr is recognized but not converted to sing-box")
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", typ)
	}
}

func clashBase(p map[string]any, typ string) (*merge.OrderedMap, error) {
	server := cleanServer(clashString(p, "server"))
	port := clashString(p, "port")
	pn, err := portNumber(port)
	if err != nil {
		return nil, err
	}
	out := merge.NewOrderedMap()
	out.Set("type", typ)
	out.Set("tag", tagOrDefault(clashString(p, "name"), server, port))
	out.Set("server", server)
	out.Set("server_port", pn)
	return out, nil
}

func clashShadowsocks(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "shadowsocks")
	if err != nil {
		return nil, err
	}
	method := clashString(p, "cipher")
	if method == "" {
		method = clashString(p, "method")
	}
	out.Set("method", method)
	out.Set("password", clashString(p, "password"))
	return out, nil
}

func clashVMess(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "vmess")
	if err != nil {
		return nil, err
	}
	out.Set("uuid", clashString(p, "uuid"))
	out.Set("alter_id", intNumber(clashInt(p, "alterId", clashInt(p, "alter-id", 0))))
	if security := clashString(p, "cipher"); security != "" {
		out.Set("security", security)
	}
	if tls := clashTLS(p, clashBool(p, "tls", false)); tls != nil {
		out.Set("tls", tls)
	}
	if tr := clashTransport(p); tr != nil {
		out.Set("transport", tr)
	}
	return out, nil
}

func clashVLESS(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "vless")
	if err != nil {
		return nil, err
	}
	out.Set("uuid", clashString(p, "uuid"))
	if flow := clashString(p, "flow"); flow != "" {
		out.Set("flow", flow)
	}
	if tls := clashTLS(p, clashBool(p, "tls", false) || clashMap(p, "reality-opts") != nil); tls != nil {
		out.Set("tls", tls)
	}
	if tr := clashTransport(p); tr != nil {
		out.Set("transport", tr)
	}
	return out, nil
}

func clashTrojan(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "trojan")
	if err != nil {
		return nil, err
	}
	out.Set("password", clashString(p, "password"))
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	if tr := clashTransport(p); tr != nil {
		out.Set("transport", tr)
	}
	return out, nil
}

func clashHysteria2(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "hysteria2")
	if err != nil {
		return nil, err
	}
	out.Set("password", clashString(p, "password"))
	if obfs := clashString(p, "obfs"); obfs != "" {
		o := merge.NewOrderedMap()
		o.Set("type", obfs)
		if pw := clashString(p, "obfs-password"); pw != "" {
			o.Set("password", pw)
		}
		out.Set("obfs", o)
	}
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashTUIC(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "tuic")
	if err != nil {
		return nil, err
	}
	out.Set("uuid", clashString(p, "uuid"))
	out.Set("password", clashString(p, "password"))
	if cc := clashString(p, "congestion-control"); cc != "" {
		out.Set("congestion_control", cc)
	}
	if mode := clashString(p, "udp-relay-mode"); mode != "" {
		out.Set("udp_relay_mode", mode)
	}
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashNaive(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "naive")
	if err != nil {
		return nil, err
	}
	if username := clashString(p, "username"); username != "" {
		out.Set("username", username)
	}
	if password := clashString(p, "password"); password != "" {
		out.Set("password", password)
	}
	if clashBool(p, "quic", false) {
		out.Set("quic", true)
	}
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashTLS(p map[string]any, enabled bool) *merge.OrderedMap {
	reality := clashMap(p, "reality-opts")
	if reality != nil {
		enabled = true
	}
	serverName := clashString(p, "servername")
	if serverName == "" {
		serverName = clashString(p, "sni")
	}
	fingerprint := clashString(p, "client-fingerprint")
	if fingerprint == "" {
		fingerprint = clashString(p, "fingerprint")
	}
	params := tlsParams{
		enabled:     enabled,
		serverName:  serverName,
		insecure:    clashBool(p, "skip-cert-verify", false),
		alpn:        clashStringSlice(p["alpn"]),
		fingerprint: fingerprint,
	}
	if reality != nil {
		params.realityPbk = clashString(reality, "public-key")
		params.realitySid = clashString(reality, "short-id")
	}
	return buildTLS(params)
}

func clashTransport(p map[string]any) *merge.OrderedMap {
	network := strings.ToLower(clashString(p, "network"))
	switch network {
	case "ws":
		opts := clashMap(p, "ws-opts")
		path := clashString(opts, "path")
		host := clashString(opts, "host")
		if host == "" {
			headers := clashMap(opts, "headers")
			host = clashString(headers, "Host")
			if host == "" {
				host = clashString(headers, "host")
			}
		}
		return buildTransport("ws", path, host, "")
	case "grpc":
		opts := clashMap(p, "grpc-opts")
		return buildTransport("grpc", "", "", clashString(opts, "grpc-service-name"))
	case "h2", "http":
		opts := clashMap(p, "h2-opts")
		return buildTransport("http", clashString(opts, "path"), firstString(clashStringSlice(opts["host"])), "")
	case "httpupgrade":
		opts := clashMap(p, "http-opts")
		return buildTransport("httpupgrade", clashString(opts, "path"), clashString(opts, "host"), "")
	default:
		return nil
	}
}

func clashString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return anyToString(m[key])
}

func clashInt(m map[string]any, key string, def int) int {
	return atoiDefault(clashString(m, key), def)
}

func clashBool(m map[string]any, key string, def bool) bool {
	if m == nil {
		return def
	}
	v, ok := m[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return boolParam(val)
	default:
		return boolParam(anyToString(val))
	}
}

func clashMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	switch val := m[key].(type) {
	case map[string]any:
		return val
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[anyToString(k)] = v
		}
		return out
	default:
		return nil
	}
}

func clashStringSlice(v any) []string {
	switch val := v.(type) {
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s := anyToString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return val
	case string:
		return splitALPN(val)
	default:
		return nil
	}
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
