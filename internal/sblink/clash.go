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
	var out *merge.OrderedMap
	var err error
	switch typ {
	case "ss", "shadowsocks":
		out, err = clashShadowsocks(p)
	case "vmess":
		out, err = clashVMess(p)
	case "vless":
		out, err = clashVLESS(p)
	case "trojan":
		out, err = clashTrojan(p)
	case "hysteria":
		out, err = clashHysteria(p)
	case "hysteria2", "hy2":
		out, err = clashHysteria2(p)
	case "tuic":
		out, err = clashTUIC(p)
	case "anytls":
		out, err = clashAnyTLS(p)
	case "shadowtls":
		out, err = clashShadowTLS(p)
	case "http", "https":
		out, err = clashHTTP(p, typ == "https")
	case "socks", "socks5", "socks4", "socks4a":
		out, err = clashSOCKS(p, typ)
	case "naive":
		out, err = clashNaive(p)
	case "ssr":
		return nil, fmt.Errorf("ssr is recognized but not converted to sing-box")
	case "wireguard", "wg":
		return nil, fmt.Errorf("wireguard is recognized but not converted because sing-box 1.13 uses endpoints instead of outbounds")
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", typ)
	}
	if err != nil {
		return nil, err
	}
	applyClashDialFields(out, p)
	return out, nil
}

func clashBase(p map[string]any, typ string) (*merge.OrderedMap, error) {
	return clashBaseWithDefault(p, typ, 0)
}

func clashBaseWithDefault(p map[string]any, typ string, defaultPort int) (*merge.OrderedMap, error) {
	server := cleanServer(clashString(p, "server"))
	port := clashString(p, "port")
	if port == "" && defaultPort > 0 {
		port = strconv.Itoa(defaultPort)
	}
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

func applyClashDialFields(out *merge.OrderedMap, p map[string]any) {
	setStringIfPresent(out, "detour", clashStringAny(p, "dialer-proxy", "dialer_proxy", "detour"))
	setStringIfPresent(out, "bind_interface", clashStringAny(p, "interface-name", "interface_name", "bind-interface", "bind_interface"))
	setStringIfPresent(out, "inet4_bind_address", clashStringAny(p, "inet4-bind-address", "inet4_bind_address"))
	setStringIfPresent(out, "inet6_bind_address", clashStringAny(p, "inet6-bind-address", "inet6_bind_address"))
	setStringIfPresent(out, "protect_path", clashStringAny(p, "protect-path", "protect_path"))
	setStringIfPresent(out, "routing_mark", clashStringAny(p, "routing-mark", "routing_mark"))
	setStringIfPresent(out, "netns", clashStringAny(p, "netns"))
	setStringIfPresent(out, "connect_timeout", clashStringAny(p, "connect-timeout", "connect_timeout"))
	setStringIfPresent(out, "tcp_keep_alive", clashStringAny(p, "tcp-keep-alive", "tcp_keep_alive"))
	setStringIfPresent(out, "tcp_keep_alive_interval", clashStringAny(p, "tcp-keep-alive-interval", "tcp_keep_alive_interval"))
	setStringIfPresent(out, "network_strategy", clashStringAny(p, "network-strategy", "network_strategy"))
	setStringIfPresent(out, "fallback_delay", clashStringAny(p, "fallback-delay", "fallback_delay"))
	setBoolIfPresent(out, p, "bind_address_no_port", "bind-address-no-port", "bind_address_no_port")
	setBoolIfPresent(out, p, "reuse_addr", "reuse-addr", "reuse_addr")
	setBoolIfPresent(out, p, "tcp_fast_open", "tfo", "tcp-fast-open", "tcp_fast_open")
	setBoolIfPresent(out, p, "tcp_multi_path", "mptcp", "tcp-multi-path", "tcp_multi_path")
	setBoolIfPresent(out, p, "disable_tcp_keep_alive", "disable-tcp-keep-alive", "disable_tcp_keep_alive")
	setBoolIfPresent(out, p, "udp_fragment", "udp-fragment", "udp_fragment")
	setStringListField(out, "network_type", clashAny(p, "network-type", "network_type"))
	setStringListField(out, "fallback_network_type", clashAny(p, "fallback-network-type", "fallback_network_type"))
	setDomainResolverField(out, clashAny(p, "domain-resolver", "domain_resolver"))
}

func setBoolIfPresent(out *merge.OrderedMap, p map[string]any, outKey string, keys ...string) {
	for _, key := range keys {
		if p == nil {
			return
		}
		if _, ok := p[key]; ok {
			out.Set(outKey, clashBool(p, key, false))
			return
		}
	}
}

func setStringListField(out *merge.OrderedMap, key string, value any) {
	values := clashStringSlice(value)
	if len(values) == 0 {
		return
	}
	arr := make([]any, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			arr = append(arr, value)
		}
	}
	if len(arr) > 0 {
		out.Set(key, arr)
	}
}

func setDomainResolverField(out *merge.OrderedMap, value any) {
	switch v := value.(type) {
	case string:
		setStringIfPresent(out, "domain_resolver", v)
	case map[string]any:
		if resolver := orderedFromMap(v); resolver != nil {
			out.Set("domain_resolver", resolver)
		}
	case map[any]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[anyToString(key)] = item
		}
		if resolver := orderedFromMap(m); resolver != nil {
			out.Set("domain_resolver", resolver)
		}
	}
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

func clashHysteria(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBaseWithDefault(p, "hysteria", 443)
	if err != nil {
		return nil, err
	}
	if ports := clashStringAny(p, "ports", "mport", "server_ports", "server-ports"); ports != "" {
		out.Set("server_ports", []any{ports})
	}
	setStringIfPresent(out, "hop_interval", clashStringAny(p, "hop-interval", "hop_interval"))
	setStringIfPresent(out, "up", clashString(p, "up"))
	setStringIfPresent(out, "down", clashString(p, "down"))
	if up := clashIntAny(p, 0, "up-mbps", "up_mbps", "upmbps"); up > 0 {
		out.Set("up_mbps", intNumber(up))
	}
	if down := clashIntAny(p, 0, "down-mbps", "down_mbps", "downmbps"); down > 0 {
		out.Set("down_mbps", intNumber(down))
	}
	setStringIfPresent(out, "obfs", clashStringAny(p, "obfs", "obfs-param", "obfsParam"))
	setStringIfPresent(out, "auth_str", clashStringAny(p, "auth-str", "auth_str", "auth", "password"))
	setStringIfPresent(out, "network", clashStringAny(p, "network", "protocol"))
	if recv := clashIntAny(p, 0, "recv-window-conn", "recv_window_conn"); recv > 0 {
		out.Set("recv_window_conn", intNumber(recv))
	}
	if recv := clashIntAny(p, 0, "recv-window", "recv_window"); recv > 0 {
		out.Set("recv_window", intNumber(recv))
	}
	if clashBoolAny(p, false, "disable-mtu-discovery", "disable_mtu_discovery") {
		out.Set("disable_mtu_discovery", true)
	}
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashHysteria2(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBaseWithDefault(p, "hysteria2", 443)
	if err != nil {
		return nil, err
	}
	out.Set("password", clashString(p, "password"))
	if ports := clashStringAny(p, "ports", "mport", "server_ports", "server-ports"); ports != "" {
		out.Set("server_ports", []any{ports})
	}
	setStringIfPresent(out, "hop_interval", clashStringAny(p, "hop-interval", "hop_interval"))
	if up := clashIntAny(p, 0, "up-mbps", "up_mbps", "upmbps"); up > 0 {
		out.Set("up_mbps", intNumber(up))
	}
	if down := clashIntAny(p, 0, "down-mbps", "down_mbps", "downmbps"); down > 0 {
		out.Set("down_mbps", intNumber(down))
	}
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
	out, err := clashBaseWithDefault(p, "tuic", 443)
	if err != nil {
		return nil, err
	}
	out.Set("uuid", clashString(p, "uuid"))
	out.Set("password", clashString(p, "password"))
	if cc := clashStringAny(p, "congestion-control", "congestion_control", "congestion-controller"); cc != "" {
		out.Set("congestion_control", cc)
	}
	if mode := clashStringAny(p, "udp-relay-mode", "udp_relay_mode"); mode != "" {
		out.Set("udp_relay_mode", mode)
	}
	setStringIfPresent(out, "heartbeat", clashStringAny(p, "heartbeat", "heartbeat-interval", "heartbeat_interval"))
	setStringIfPresent(out, "network", clashString(p, "network"))
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashAnyTLS(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBaseWithDefault(p, "anytls", 443)
	if err != nil {
		return nil, err
	}
	password := clashString(p, "password")
	if password == "" {
		password = clashString(p, "auth")
	}
	if password == "" {
		return nil, fmt.Errorf("anytls missing password")
	}
	out.Set("password", password)
	setStringIfPresent(out, "idle_session_check_interval", clashStringAny(p, "idle-session-check-interval", "idle_session_check_interval"))
	setStringIfPresent(out, "idle_session_timeout", clashStringAny(p, "idle-session-timeout", "idle_session_timeout"))
	if minIdle := clashIntAny(p, -1, "min-idle-session", "min_idle_session"); minIdle >= 0 {
		out.Set("min_idle_session", intNumber(minIdle))
	}
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashShadowTLS(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBaseWithDefault(p, "shadowtls", 443)
	if err != nil {
		return nil, err
	}
	if version := clashInt(p, "version", 0); version > 0 {
		out.Set("version", intNumber(version))
	}
	setStringIfPresent(out, "password", clashString(p, "password"))
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashHTTP(p map[string]any, forceTLS bool) (*merge.OrderedMap, error) {
	defaultPort := 80
	if forceTLS || clashBool(p, "tls", false) {
		defaultPort = 443
	}
	out, err := clashBaseWithDefault(p, "http", defaultPort)
	if err != nil {
		return nil, err
	}
	setStringIfPresent(out, "username", clashString(p, "username"))
	setStringIfPresent(out, "password", clashString(p, "password"))
	setStringIfPresent(out, "path", clashString(p, "path"))
	if headers := orderedFromMap(clashMap(p, "headers")); headers != nil {
		out.Set("headers", headers)
	}
	if tls := clashTLS(p, forceTLS || clashBool(p, "tls", false)); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashSOCKS(p map[string]any, typ string) (*merge.OrderedMap, error) {
	out, err := clashBaseWithDefault(p, "socks", 1080)
	if err != nil {
		return nil, err
	}
	version := "5"
	switch typ {
	case "socks4":
		version = "4"
	case "socks4a":
		version = "4a"
	}
	out.Set("version", version)
	setStringIfPresent(out, "username", clashString(p, "username"))
	setStringIfPresent(out, "password", clashString(p, "password"))
	setStringIfPresent(out, "network", clashString(p, "network"))
	if clashBoolAny(p, false, "udp-over-tcp", "udp_over_tcp", "uot") {
		out.Set("udp_over_tcp", true)
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

func clashAny(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if m == nil {
			return nil
		}
		if value, ok := m[key]; ok {
			return value
		}
	}
	return nil
}

func clashStringAny(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := clashString(m, key); value != "" {
			return value
		}
	}
	return ""
}

func clashInt(m map[string]any, key string, def int) int {
	return atoiDefault(clashString(m, key), def)
}

func clashIntAny(m map[string]any, def int, keys ...string) int {
	for _, key := range keys {
		if value := clashString(m, key); value != "" {
			return atoiDefault(value, def)
		}
	}
	return def
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

func clashBoolAny(m map[string]any, def bool, keys ...string) bool {
	for _, key := range keys {
		if m != nil {
			if _, ok := m[key]; ok {
				return clashBool(m, key, def)
			}
		}
	}
	return def
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
