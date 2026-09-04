package sblink

import (
	"encoding/json"
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
	case "snell":
		out, err = clashSnell(p)
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
	port := clashStringAny(p, "port", "server-port", "server_port")
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

func setNetworkField(out *merge.OrderedMap, value any) {
	values := clashStringSlice(value)
	if len(values) == 0 {
		return
	}
	items := make([]any, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "tcp" || value == "udp" {
			if _, ok := seen[value]; !ok {
				items = append(items, value)
				seen[value] = struct{}{}
			}
		}
	}
	if len(items) > 0 {
		out.Set("network", items)
	}
}

func setDomainResolverField(out *merge.OrderedMap, value any) {
	switch v := value.(type) {
	case string:
		setStringIfPresent(out, "domain_resolver", v)
	case map[string]any:
		if resolver := orderedFromMap(normalizeDomainResolverMap(v)); resolver != nil {
			out.Set("domain_resolver", resolver)
		}
	case map[any]any:
		m := make(map[string]any, len(v))
		for key, item := range v {
			m[anyToString(key)] = item
		}
		if resolver := orderedFromMap(normalizeDomainResolverMap(m)); resolver != nil {
			out.Set("domain_resolver", resolver)
		}
	}
}

func normalizeDomainResolverMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	allowed := map[string]struct{}{
		"server": {}, "strategy": {}, "disable_cache": {}, "rewrite_ttl": {}, "client_subnet": {},
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		key = strings.ReplaceAll(strings.TrimSpace(key), "-", "_")
		if _, ok := allowed[key]; !ok {
			continue
		}
		result[key] = item
	}
	return result
}

func setClashJSONField(out *merge.OrderedMap, key string, value any) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return
		}
		var parsed any
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			out.Set(key, orderedAny(parsed))
			return
		}
		out.Set(key, trimmed)
	default:
		switch v := v.(type) {
		case map[string]any, map[any]any:
			if ordered := orderedAny(v); ordered != nil {
				out.Set(key, ordered)
			}
		default:
			out.Set(key, v)
		}
	}
}

func setClashUDPOverTCP(out *merge.OrderedMap, value any) {
	if value == nil {
		return
	}
	if text := strings.TrimSpace(anyToString(value)); text != "" {
		if boolParam(text) {
			out.Set("udp_over_tcp", true)
			return
		}
		if version, err := strconv.Atoi(text); err == nil && version > 0 {
			uot := merge.NewOrderedMap()
			uot.Set("enabled", true)
			uot.Set("version", intNumber(version))
			out.Set("udp_over_tcp", uot)
			return
		}
	}
	setClashJSONField(out, "udp_over_tcp", value)
}

func setClashMultiplex(out *merge.OrderedMap, value any) {
	if value == nil {
		return
	}
	switch v := value.(type) {
	case bool:
		if !v {
			return
		}
		multiplex := merge.NewOrderedMap()
		multiplex.Set("enabled", true)
		out.Set("multiplex", multiplex)
	case string:
		if boolParam(strings.TrimSpace(v)) {
			multiplex := merge.NewOrderedMap()
			multiplex.Set("enabled", true)
			out.Set("multiplex", multiplex)
			return
		}
		if strings.EqualFold(strings.TrimSpace(v), "false") {
			return
		}
		setClashJSONField(out, "multiplex", value)
	default:
		setClashJSONField(out, "multiplex", value)
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
	setStringIfPresent(out, "plugin", clashStringAny(p, "plugin"))
	setStringIfPresent(out, "plugin_opts", clashStringAny(p, "plugin-opts", "plugin_opts"))
	setNetworkField(out, clashAny(p, "network"))
	setClashUDPOverTCP(out, clashAny(p, "udp-over-tcp", "udp_over_tcp", "uot"))
	setClashMultiplex(out, clashAny(p, "multiplex"))
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
	setNetworkField(out, clashAny(p, "network"))
	setStringIfPresent(out, "packet_encoding", clashStringAny(p, "packet-encoding", "packet_encoding"))
	setBoolIfPresent(out, p, "global_padding", "global-padding", "global_padding")
	setBoolIfPresent(out, p, "authenticated_length", "authenticated-length", "authenticated_length")
	setClashMultiplex(out, clashAny(p, "multiplex"))
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
	setNetworkField(out, clashAny(p, "network"))
	setStringIfPresent(out, "packet_encoding", clashStringAny(p, "packet-encoding", "packet_encoding"))
	setClashMultiplex(out, clashAny(p, "multiplex"))
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
	setNetworkField(out, clashAny(p, "network"))
	setClashMultiplex(out, clashAny(p, "multiplex"))
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
	setNetworkField(out, clashAny(p, "network", "protocol"))
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
	setBoolIfPresent(out, p, "brutal_debug", "brutal-debug", "brutal_debug")
	setNetworkField(out, clashAny(p, "network"))
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
	setNetworkField(out, clashAny(p, "network"))
	setBoolIfPresent(out, p, "udp_over_stream", "udp-over-stream", "udp_over_stream")
	setBoolIfPresent(out, p, "zero_rtt_handshake", "zero-rtt-handshake", "zero_rtt_handshake")
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashSnell(p map[string]any) (*merge.OrderedMap, error) {
	out, err := clashBase(p, "snell")
	if err != nil {
		return nil, err
	}
	psk := clashStringAny(p, "psk", "password")
	if psk == "" {
		return nil, fmt.Errorf("snell missing psk")
	}
	version, err := snellVersion(clashString(p, "version"))
	if err != nil {
		return nil, err
	}
	out.Set("version", intNumber(version))
	out.Set("psk", psk)
	if version == 6 && (len([]byte(psk)) < 12 || len([]byte(psk)) > 255) {
		return nil, fmt.Errorf("snell v6 psk must be 12-255 bytes")
	}
	setStringIfPresent(out, "userkey", clashStringAny(p, "userkey", "user-key"))
	if _, ok := p["reuse"]; ok && clashBool(p, "reuse", false) {
		out.Set("reuse", true)
	}
	if network, err := clashSnellNetwork(clashAny(p, "network")); err != nil {
		return nil, err
	} else if network != "" {
		if err := validateSnellNetwork(network); err != nil {
			return nil, err
		}
		out.Set("network", snellNetworkValue(network))
	} else if _, ok := p["udp"]; ok {
		if clashBool(p, "udp", false) {
			out.Set("network", []any{"tcp", "udp"})
		} else {
			out.Set("network", "tcp")
		}
	}
	obfs := clashMap(p, "obfs-opts")
	obfsMode := strings.ToLower(strings.TrimSpace(clashStringAny(obfs, "mode", "obfs_mode", "obfs-mode")))
	if obfsMode == "" {
		obfsMode = strings.ToLower(strings.TrimSpace(clashStringAny(p, "obfs", "obfs_mode", "obfs-mode")))
	}
	if version == 6 {
		if obfsMode != "" {
			return nil, fmt.Errorf("snell obfs is only valid for version 4")
		}
		if mode := strings.TrimSpace(clashString(p, "mode")); mode != "" {
			if mode != "default" && mode != "unshaped" && mode != "unsafe-raw" {
				return nil, fmt.Errorf("unsupported snell v6 mode %q", mode)
			}
			out.Set("mode", mode)
		}
		return out, nil
	}
	if obfsMode != "" && obfsMode != "none" && obfsMode != "http" && obfsMode != "tls" {
		return nil, fmt.Errorf("unsupported snell v4 obfs mode %q", obfsMode)
	}
	if obfsMode == "http" || obfsMode == "tls" {
		out.Set("obfs_mode", obfsMode)
		obfsHost := clashStringAny(obfs, "host", "obfs-host", "obfs_host")
		if obfsHost == "" {
			obfsHost = clashStringAny(p, "obfs-host", "obfs_host", "obfs-hostname")
		}
		setStringIfPresent(out, "obfs_host", obfsHost)
	}
	if mode := strings.TrimSpace(clashString(p, "mode")); mode != "" {
		return nil, fmt.Errorf("snell mode is only valid for version 6")
	}
	return out, nil
}

func clashSnellNetwork(value any) (string, error) {
	switch value := value.(type) {
	case []any:
		items := clashStringSlice(value)
		return strings.Join(items, ","), nil
	case []string:
		return strings.Join(value, ","), nil
	case nil:
		return "", nil
	default:
		return strings.TrimSpace(anyToString(value)), nil
	}
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
	setStringIfPresent(out, "client_metadata", clashStringAny(p, "client-metadata", "client_metadata"))
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
		if version > 3 {
			return nil, fmt.Errorf("unsupported shadowtls version %d", version)
		}
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
	setNetworkField(out, clashAny(p, "network"))
	setClashUDPOverTCP(out, clashAny(p, "udp-over-tcp", "udp_over_tcp", "uot"))
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
	if concurrency := clashIntAny(p, -1, "insecure-concurrency", "insecure_concurrency"); concurrency >= 0 {
		out.Set("insecure_concurrency", intNumber(concurrency))
	}
	setClashJSONField(out, "extra_headers", clashAny(p, "extra-headers", "extra_headers"))
	setStringIfPresent(out, "stream_receive_window", clashStringAny(p, "stream-receive-window", "stream_receive_window"))
	if clashBool(p, "quic", false) {
		out.Set("quic", true)
	}
	setStringIfPresent(out, "quic_congestion_control", clashStringAny(p, "quic-congestion-control", "quic_congestion_control"))
	setClashUDPOverTCP(out, clashAny(p, "udp-over-tcp", "udp_over_tcp"))
	setStringIfPresent(out, "quic_session_receive_window", clashStringAny(p, "quic-session-receive-window", "quic_session_receive_window"))
	if tls := clashTLS(p, true); tls != nil {
		out.Set("tls", tls)
	}
	return out, nil
}

func clashTLS(p map[string]any, enabled bool) *merge.OrderedMap {
	reality := clashMap(p, "reality-opts")
	if reality == nil {
		reality = clashMap(p, "reality")
	}
	if reality != nil {
		enabled = true
	}
	if clashMap(p, "utls") != nil {
		enabled = true
	}
	for _, key := range []string{
		"disable-sni", "disable_sni", "min-version", "min_version", "max-version", "max_version",
		"cipher-suites", "cipher_suites", "curve-preferences", "curve_preferences", "certificate", "certificate-path", "certificate_path",
		"certificate-public-key-sha256", "certificate_public_key_sha256", "client-certificate", "client_certificate",
		"client-certificate-path", "client_certificate_path", "client-key", "client_key", "client-key-path", "client_key_path",
		"fragment", "fragment-fallback-delay", "fragment_fallback_delay", "record-fragment", "record_fragment",
		"kernel-tx", "kernel_tx", "kernel-rx", "kernel_rx", "ech",
	} {
		if _, ok := p[key]; ok {
			enabled = true
			break
		}
	}
	serverName := clashString(p, "servername")
	if serverName == "" {
		serverName = clashString(p, "sni")
	}
	fingerprint := clashString(p, "client-fingerprint")
	if fingerprint == "" {
		fingerprint = clashString(p, "fingerprint")
	}
	fingerprint = normalizeUTLSFingerprint(fingerprint)
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
	tls := buildTLS(params)
	if tls == nil {
		return nil
	}
	setBoolIfPresent(tls, p, "disable_sni", "disable-sni", "disable_sni")
	setStringIfPresent(tls, "min_version", clashStringAny(p, "min-version", "min_version"))
	setStringIfPresent(tls, "max_version", clashStringAny(p, "max-version", "max_version"))
	setStringListField(tls, "cipher_suites", clashAny(p, "cipher-suites", "cipher_suites"))
	setStringListField(tls, "curve_preferences", clashAny(p, "curve-preferences", "curve_preferences"))
	setStringListField(tls, "certificate", clashAny(p, "certificate"))
	setStringIfPresent(tls, "certificate_path", clashStringAny(p, "certificate-path", "certificate_path"))
	setStringListField(tls, "certificate_public_key_sha256", clashAny(p, "certificate-public-key-sha256", "certificate_public_key_sha256"))
	setStringListField(tls, "client_certificate", clashAny(p, "client-certificate", "client_certificate"))
	setStringIfPresent(tls, "client_certificate_path", clashStringAny(p, "client-certificate-path", "client_certificate_path"))
	setStringListField(tls, "client_key", clashAny(p, "client-key", "client_key"))
	setStringIfPresent(tls, "client_key_path", clashStringAny(p, "client-key-path", "client_key_path"))
	setBoolIfPresent(tls, p, "fragment", "fragment")
	setStringIfPresent(tls, "fragment_fallback_delay", clashStringAny(p, "fragment-fallback-delay", "fragment_fallback_delay"))
	setBoolIfPresent(tls, p, "record_fragment", "record-fragment", "record_fragment")
	setBoolIfPresent(tls, p, "kernel_tx", "kernel-tx", "kernel_tx")
	setBoolIfPresent(tls, p, "kernel_rx", "kernel-rx", "kernel_rx")
	setClashJSONField(tls, "ech", clashAny(p, "ech"))
	setClashJSONField(tls, "utls", clashAny(p, "utls"))
	setClashJSONField(tls, "reality", clashAny(p, "reality"))
	return tls
}

func normalizeUTLSFingerprint(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "chrome_psk", "chrome_psk_shuffle", "chrome_padding_psk_shuffle", "chrome_pq", "chrome_pq_psk":
		return "chrome"
	default:
		return strings.TrimSpace(value)
	}
}

func clashTransport(p map[string]any) *merge.OrderedMap {
	network := strings.ToLower(strings.TrimSpace(clashString(p, "network")))
	switch network {
	case "ws":
		options := clashMap(p, "ws-opts")
		host := clashString(options, "host")
		if host == "" {
			headers := clashMap(options, "headers")
			host = clashStringAny(headers, "Host", "host")
		}
		transport := buildTransport("ws", clashString(options, "path"), host, "")
		setStringIfPresent(transport, "early_data_header_name", clashStringAny(options, "early-data-header-name", "early_data_header_name"))
		if maxEarly := clashIntAny(options, 0, "max-early-data", "max_early_data"); maxEarly > 0 {
			transport.Set("max_early_data", intNumber(maxEarly))
		}
		setTransportHeaders(transport, options)
		return transport
	case "grpc":
		options := clashMap(p, "grpc-opts")
		transport := buildTransport("grpc", "", "", clashStringAny(options, "grpc-service-name", "service-name", "service_name"))
		setTransportKeepalive(transport, options)
		setBoolIfPresent(transport, options, "permit_without_stream", "permit-without-stream", "permit_without_stream")
		return transport
	case "h2":
		options := clashMap(p, "h2-opts")
		transport := buildTransport("http", clashString(options, "path"), firstString(clashStringSlice(options["host"])), "")
		setTransportHTTPFields(transport, options)
		return transport
	case "http":
		options := clashMap(p, "http-opts")
		if options == nil {
			options = clashMap(p, "h2-opts")
		}
		transport := buildTransport("http", clashString(options, "path"), firstString(clashStringSlice(options["host"])), "")
		setTransportHTTPFields(transport, options)
		return transport
	case "httpupgrade":
		options := clashMap(p, "http-opts")
		transport := buildTransport("httpupgrade", clashString(options, "path"), clashString(options, "host"), "")
		setTransportHeaders(transport, options)
		return transport
	case "quic":
		return buildTransport("quic", "", "", "")
	default:
		return nil
	}
}

func setTransportHeaders(transport *merge.OrderedMap, options map[string]any) {
	if headers := orderedFromMap(clashMap(options, "headers")); headers != nil {
		if existing, ok := orderedField(transport, "headers"); ok {
			for _, key := range existing.Keys() {
				if !headers.Has(key) {
					if value, ok := existing.Get(key); ok {
						headers.Set(key, value)
					}
				}
			}
		}
		transport.Set("headers", headers)
	}
}

func setTransportKeepalive(transport *merge.OrderedMap, options map[string]any) {
	setStringIfPresent(transport, "idle_timeout", clashStringAny(options, "idle-timeout", "idle_timeout"))
	setStringIfPresent(transport, "ping_timeout", clashStringAny(options, "ping-timeout", "ping_timeout"))
}

func setTransportHTTPFields(transport *merge.OrderedMap, options map[string]any) {
	setStringIfPresent(transport, "method", clashStringAny(options, "method"))
	setTransportKeepalive(transport, options)
	setTransportHeaders(transport, options)
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
