package sblink

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mora1n/sb-fox/internal/merge"
)

// field returns a string field from an outbound, failing the test if absent.
func field(t *testing.T, m *merge.OrderedMap, key string) string {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	s, _ := v.(string)
	return s
}

func nested(t *testing.T, m *merge.OrderedMap, key string) *merge.OrderedMap {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing nested %q", key)
	}
	om, ok := v.(*merge.OrderedMap)
	if !ok {
		t.Fatalf("field %q is not an object", key)
	}
	return om
}

func scalarField(t *testing.T, m *merge.OrderedMap, key string) string {
	t.Helper()
	v, ok := m.Get(key)
	if !ok {
		t.Fatalf("missing field %q", key)
	}
	return scalarString(v)
}

func TestParseShadowsocksLegacy(t *testing.T) {
	// legacy: ss://base64(method:password)@host:port#tag
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:secretpass"))
	uri := "ss://" + userinfo + "@1.2.3.4:8388#HK-Node"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "shadowsocks" {
		t.Errorf("type = %s", field(t, out, "type"))
	}
	if field(t, out, "method") != "aes-256-gcm" {
		t.Errorf("method = %s", field(t, out, "method"))
	}
	if field(t, out, "password") != "secretpass" {
		t.Errorf("password = %s", field(t, out, "password"))
	}
	if field(t, out, "server") != "1.2.3.4" {
		t.Errorf("server = %s", field(t, out, "server"))
	}
	if field(t, out, "tag") != "HK-Node" {
		t.Errorf("tag = %s", field(t, out, "tag"))
	}
}

func TestParseShadowsocksSIP002(t *testing.T) {
	// SIP002: userinfo is base64url of method:password, plugin in query
	userinfo := base64.RawURLEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:pw123"))
	uri := "ss://" + userinfo + "@example.com:443?plugin=obfs-local%3Bobfs%3Dhttp#SG"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "method") != "chacha20-ietf-poly1305" {
		t.Errorf("method = %s", field(t, out, "method"))
	}
	if got := field(t, out, "server"); got != "example.com" {
		t.Errorf("server = %s", got)
	}
}

func TestParseVMessWSTLS(t *testing.T) {
	vmessJSON := map[string]any{
		"v": "2", "ps": "JP-WS", "add": "jp.example.com", "port": "443",
		"id": "b831381d-6324-4d53-ad4f-8cda48b30811", "aid": "0", "scy": "auto",
		"net": "ws", "type": "none", "host": "jp.example.com", "path": "/path", "tls": "tls",
	}
	raw, _ := json.Marshal(vmessJSON)
	uri := "vmess://" + base64.StdEncoding.EncodeToString(raw)
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "vmess" {
		t.Errorf("type = %s", field(t, out, "type"))
	}
	if field(t, out, "uuid") != "b831381d-6324-4d53-ad4f-8cda48b30811" {
		t.Errorf("uuid = %s", field(t, out, "uuid"))
	}
	if field(t, out, "server") != "jp.example.com" {
		t.Errorf("server = %s", field(t, out, "server"))
	}
	tls := nested(t, out, "tls")
	if field(t, tls, "server_name") != "jp.example.com" {
		t.Errorf("tls server_name = %s", field(t, tls, "server_name"))
	}
	tr := nested(t, out, "transport")
	if field(t, tr, "type") != "ws" || field(t, tr, "path") != "/path" {
		t.Errorf("transport = %+v", tr)
	}
}

func TestParseVLESSReality(t *testing.T) {
	uri := "vless://uuid-1234@vless.example.com:443?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome&pbk=publickeyxyz&sid=abcd&flow=xtls-rprx-vision&type=tcp#Reality"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "flow") != "xtls-rprx-vision" {
		t.Errorf("flow = %s", field(t, out, "flow"))
	}
	tls := nested(t, out, "tls")
	if field(t, tls, "server_name") != "www.microsoft.com" {
		t.Errorf("sni = %s", field(t, tls, "server_name"))
	}
	reality := nested(t, tls, "reality")
	if field(t, reality, "public_key") != "publickeyxyz" {
		t.Errorf("pbk = %s", field(t, reality, "public_key"))
	}
}

func TestParseVLESSPacketEncoding(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "none is omitted",
			uri:  "vless://uuid-1234@vless.example.com:443?packetEncoding=none#VLESS",
			want: "",
		},
		{
			name: "xudp is preserved",
			uri:  "vless://uuid-1234@vless.example.com:443?packet_encoding=xudp#VLESS",
			want: "xudp",
		},
		{
			name: "packetaddr is preserved",
			uri:  "vless://uuid-1234@vless.example.com:443?packet-encoding=packetaddr#VLESS",
			want: "packetaddr",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Parse(tt.uri)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := out.Get("packet_encoding")
			if tt.want == "" {
				if ok {
					t.Fatalf("packet_encoding = %v, want absent", got)
				}
				return
			}
			if !ok || got != tt.want {
				t.Fatalf("packet_encoding = %v, want %q", got, tt.want)
			}
		})
	}

	_, err := Parse("vless://uuid-1234@vless.example.com:443?packetEncoding=bad#VLESS")
	if err == nil || !strings.Contains(err.Error(), "packetEncoding") {
		t.Fatalf("invalid packetEncoding err = %v", err)
	}
}

func TestParseTrojanWS(t *testing.T) {
	uri := "trojan://mypassword@trojan.example.com:443?security=tls&sni=trojan.example.com&type=ws&path=%2Fws#Trojan"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "trojan" || field(t, out, "password") != "mypassword" {
		t.Errorf("trojan fields wrong: %+v", out)
	}
	tr := nested(t, out, "transport")
	if field(t, tr, "type") != "ws" || field(t, tr, "path") != "/ws" {
		t.Errorf("transport = %+v", tr)
	}
}

func TestParseHysteria2(t *testing.T) {
	uri := "hysteria2://pass123@hy2.example.com:8443?sni=hy2.example.com&obfs=salamander&obfs-password=obfspw#HY2"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "hysteria2" || field(t, out, "password") != "pass123" {
		t.Errorf("hy2 fields wrong: %+v", out)
	}
	obfs := nested(t, out, "obfs")
	if field(t, obfs, "type") != "salamander" || field(t, obfs, "password") != "obfspw" {
		t.Errorf("obfs = %+v", obfs)
	}
}

func TestParseTUIC(t *testing.T) {
	uri := "tuic://uuid-9:password9@tuic.example.com:443?congestion_control=bbr&sni=tuic.example.com#TUIC"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "uuid") != "uuid-9" || field(t, out, "password") != "password9" {
		t.Errorf("tuic fields wrong: %+v", out)
	}
	if field(t, out, "congestion_control") != "bbr" {
		t.Errorf("cc = %s", field(t, out, "congestion_control"))
	}
}

func TestParseNaive(t *testing.T) {
	uri := "naive+https://user:pass@naive.example.com:443?quic=true&sni=naive.example.com#Naive"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if field(t, out, "type") != "naive" || field(t, out, "username") != "user" || field(t, out, "password") != "pass" {
		t.Errorf("naive fields wrong: %+v", out)
	}
	if got, _ := out.Get("quic"); got != true {
		t.Errorf("quic = %v", got)
	}
	tls := nested(t, out, "tls")
	if field(t, tls, "server_name") != "naive.example.com" {
		t.Errorf("tls server_name = %s", field(t, tls, "server_name"))
	}
}

func TestParseAdditionalProtocols(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		typ  string
		port string
	}{
		{
			name: "anytls",
			uri:  "anytls://secret@any.example.com?security=tls&sni=any.example.com&fp=chrome&idle_session_check_interval=20s&idle_session_timeout=40s&min_idle_session=2#Any",
			typ:  "anytls",
			port: "443",
		},
		{
			name: "shadowtls",
			uri:  "shadowtls://secret@st.example.com?version=3&security=tls&sni=st.example.com#ST",
			typ:  "shadowtls",
			port: "443",
		},
		{
			name: "hysteria",
			uri:  "hysteria://hy.example.com?auth=secret&upmbps=20&downmbps=30&sni=hy.example.com#HY1",
			typ:  "hysteria",
			port: "443",
		},
		{
			name: "https",
			uri:  "https://user:pass@http.example.com/proxy?headers=%7B%22Host%22%3A%22edge.example.com%22%7D#HTTP",
			typ:  "http",
			port: "443",
		},
		{
			name: "socks5",
			uri:  "socks5://user:pass@socks.example.com#Socks",
			typ:  "socks",
			port: "1080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Parse(tt.uri)
			if err != nil {
				t.Fatal(err)
			}
			if field(t, out, "type") != tt.typ {
				t.Fatalf("type = %s, want %s", field(t, out, "type"), tt.typ)
			}
			if scalarField(t, out, "server_port") != tt.port {
				t.Fatalf("server_port = %s, want %s", scalarField(t, out, "server_port"), tt.port)
			}
		})
	}
}

func TestDefaultPortsForCompatibleLinks(t *testing.T) {
	tests := map[string]string{
		"trojan://pw@tr.example.com#TR":             "443",
		"hysteria2://pw@hy.example.com#HY":          "443",
		"hy2://pw@hy.example.com#HY":                "443",
		"hy://hy1.example.com?auth=secret#HY1":      "443",
		"tuic://uuid-9:password9@tu.example.com#TU": "443",
		"naive://user:pass@naive.example.com#Naive": "443",
		"naive+http://user:pass@naive.example.com":  "80",
	}
	for uri, wantPort := range tests {
		t.Run(uri, func(t *testing.T) {
			out, err := Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			if got := scalarField(t, out, "server_port"); got != wantPort {
				t.Fatalf("server_port = %s, want %s", got, wantPort)
			}
		})
	}
}

func TestParseManyBase64Blob(t *testing.T) {
	links := "ss://" + base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@1.1.1.1:8388#A\n" +
		"trojan://pw@2.2.2.2:443?security=tls#B\n" +
		"anytls://pw@3.3.3.3?security=tls#C"
	blob := base64.StdEncoding.EncodeToString([]byte(links))
	out, err := ParseMany(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("parsed %d links, want 3", len(out))
	}
}

func TestParseManySIP008(t *testing.T) {
	text := `{
  "servers": [
    {"id":"1","remarks":"SIP","server":"sip.example.com","server_port":8388,"method":"2022-blake3-aes-128-gcm","password":"pw"}
  ]
}`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 1 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	if field(t, out[0], "type") != "shadowsocks" || field(t, out[0], "tag") != "SIP" {
		t.Fatalf("sip008 outbound = %+v", out[0])
	}
}

func TestParseManyClashMihomoYAML(t *testing.T) {
	text := `
proxies:
  - name: Reality
    type: vless
    server: reality.example.com
    port: 443
    uuid: b831381d-6324-4d53-ad4f-8cda48b30811
    flow: xtls-rprx-vision
    client-fingerprint: chrome
    reality-opts:
      public-key: publickeyxyz
      short-id: abcd
    servername: www.microsoft.com
  - name: Naive
    type: naive
    server: naive.example.com
    port: 443
    username: user
    password: pass
    quic: true
    sni: naive.example.com
  - name: AnyTLS
    type: anytls
    server: any.example.com
    password: anypass
    sni: any.example.com
    idle-session-check-interval: 20s
  - name: HTTP
    type: http
    server: http.example.com
    username: user
    password: pass
    headers:
      Host: edge.example.com
  - name: Socks
    type: socks5
    server: socks.example.com
    username: user
    password: pass
  - name: OldSSR
    type: ssr
    server: ssr.example.com
    port: 8388
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 5 {
		t.Fatalf("parsed %d outbounds, want 5", len(out))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "ssr") {
		t.Fatalf("warnings = %+v", warnings)
	}
	reality := out[0]
	if field(t, reality, "type") != "vless" || field(t, reality, "flow") != "xtls-rprx-vision" {
		t.Fatalf("reality outbound = %+v", reality)
	}
	tls := nested(t, reality, "tls")
	realityTLS := nested(t, tls, "reality")
	if field(t, realityTLS, "public_key") != "publickeyxyz" {
		t.Fatalf("reality tls = %+v", realityTLS)
	}
	if field(t, out[1], "type") != "naive" || field(t, out[1], "username") != "user" {
		t.Fatalf("naive outbound = %+v", out[1])
	}
	if field(t, out[2], "type") != "anytls" || scalarField(t, out[2], "server_port") != "443" {
		t.Fatalf("anytls outbound = %+v", out[2])
	}
	if field(t, out[3], "type") != "http" || scalarField(t, out[3], "server_port") != "80" {
		t.Fatalf("http outbound = %+v", out[3])
	}
	if field(t, out[4], "type") != "socks" || scalarField(t, out[4], "server_port") != "1080" {
		t.Fatalf("socks outbound = %+v", out[4])
	}
}

func TestParseClashYAMLMapsDialFields(t *testing.T) {
	text := `
proxies:
  - name: Dialed
    type: http
    server: dial.example.com
    port: 443
    tls: true
    dialer-proxy: Upstream
    interface-name: eth0
    routing-mark: "0x1234"
    tfo: true
    mptcp: true
    udp-fragment: false
    domain-resolver:
      server: local
      timeout: 1s
      strategy: prefer_ipv4
    network-strategy: fallback
    network-type:
      - wifi
      - ethernet
    fallback-network-type: cellular
    fallback-delay: 1s
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 1 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	node := out[0]
	for key, want := range map[string]string{
		"detour":           "Upstream",
		"bind_interface":   "eth0",
		"routing_mark":     "0x1234",
		"network_strategy": "fallback",
		"fallback_delay":   "1s",
	} {
		if got := field(t, node, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	for key, want := range map[string]string{
		"tcp_fast_open":  "true",
		"tcp_multi_path": "true",
		"udp_fragment":   "false",
	} {
		if got := scalarField(t, node, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	resolver := nested(t, node, "domain_resolver")
	if got := field(t, resolver, "server"); got != "local" {
		t.Fatalf("domain_resolver.server = %q", got)
	}
	if got := field(t, resolver, "timeout"); got != "1s" {
		t.Fatalf("domain_resolver.timeout = %q", got)
	}
	if got := field(t, resolver, "strategy"); got != "prefer_ipv4" {
		t.Fatalf("domain_resolver.strategy = %q", got)
	}
	if got := scalarField(t, node, "network_type"); got != "[wifi ethernet]" {
		t.Fatalf("network_type = %q", got)
	}
	if got := scalarField(t, node, "fallback_network_type"); got != "[cellular]" {
		t.Fatalf("fallback_network_type = %q", got)
	}
}

func TestParseClashYAMLDoesNotConvertDomainStrategy(t *testing.T) {
	text := `
proxies:
  - name: LegacyStrategy
    type: socks5
    server: socks.example.com
    port: 1080
    domain-strategy: prefer_ipv4
    ip-version: ipv4
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 1 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	if _, ok := out[0].Get("domain_resolver"); ok {
		t.Fatalf("unexpected domain_resolver = %+v", out[0])
	}
	if _, ok := out[0].Get("domain_strategy"); ok {
		t.Fatalf("unexpected domain_strategy = %+v", out[0])
	}
}

func TestParseManyRejectsHTML(t *testing.T) {
	_, _, err := ParseManyWithWarnings(`<!DOCTYPE html><html><body>login</body></html>`)
	if err == nil || !strings.Contains(err.Error(), "html") {
		t.Fatalf("html parse err = %v", err)
	}
}

func TestParseManyDoesNotTreatPlainHTTPSURLAsProxyLink(t *testing.T) {
	out, warnings, err := ParseManyWithWarnings(`https://example.com/subscription`)
	if err == nil {
		t.Fatalf("expected plain https URL to fail, got out=%d warnings=%v", len(out), warnings)
	}
	if len(out) != 0 {
		t.Fatalf("parsed %d links from plain https URL", len(out))
	}
}

func TestParseClashYAMLKeepsRealityShortIDLeadingZero(t *testing.T) {
	text := `
proxies:
  - name: Reality08
    type: vless
    server: reality.example.com
    port: 443
    uuid: b831381d-6324-4d53-ad4f-8cda48b30811
    reality-opts:
      public-key: publickeyxyz
      short-id: 08
    servername: www.microsoft.com
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 1 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	reality := nested(t, nested(t, out[0], "tls"), "reality")
	if got := field(t, reality, "short_id"); got != "08" {
		t.Fatalf("short_id = %q, want 08", got)
	}
}

func TestServerHashCCStripped(t *testing.T) {
	// server carrying a #CC suffix (requirement h) is cleaned
	userinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw"))
	uri := "ss://" + userinfo + "@relay.example.com%23CN:8388#Relay"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, out, "server"); got != "relay.example.com" {
		t.Errorf("server not cleaned of #CN: %q", got)
	}
}

func TestEncodeRoundTripLinks(t *testing.T) {
	ssUserinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw"))
	vmessJSON, _ := json.Marshal(map[string]any{
		"v": "2", "ps": "JP-WS", "add": "jp.example.com", "port": "443",
		"id": "b831381d-6324-4d53-ad4f-8cda48b30811", "aid": "0", "scy": "auto",
		"net": "ws", "type": "none", "host": "jp.example.com", "path": "/path", "tls": "tls",
	})
	uris := []string{
		"ss://" + ssUserinfo + "@1.1.1.1:8388#A",
		"vmess://" + base64.StdEncoding.EncodeToString(vmessJSON),
		"vless://uuid-1234@vless.example.com:443?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome&pbk=publickeyxyz&sid=abcd&flow=xtls-rprx-vision&type=tcp#Reality",
		"trojan://mypassword@trojan.example.com:443?security=tls&sni=trojan.example.com&type=ws&path=%2Fws#Trojan",
		"anytls://anypass@any.example.com?security=tls&sni=any.example.com&idle_session_check_interval=30s&idle_session_timeout=40s&min_idle_session=1#AnyTLS",
		"shadowtls://stpass@st.example.com?version=3&security=tls&sni=st.example.com#ShadowTLS",
		"hysteria://hy1.example.com?auth=secret&upmbps=20&downmbps=30&sni=hy1.example.com#HY1",
		"hysteria2://pass123@hy2.example.com:8443?sni=hy2.example.com&obfs=salamander&obfs-password=obfspw#HY2",
		"tuic://uuid-9:password9@tuic.example.com:443?congestion_control=bbr&sni=tuic.example.com#TUIC",
		"naive+https://user:pass@naive.example.com:443?quic=true&sni=naive.example.com#Naive",
		"https://user:pass@http.example.com:443/proxy?sni=http.example.com#HTTP",
		"socks5://user:pass@socks.example.com:1080?network=tcp#Socks",
	}

	for _, uri := range uris {
		t.Run(uri[:strings.Index(uri, "://")], func(t *testing.T) {
			out, err := Parse(uri)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := Encode(out)
			if err != nil {
				t.Fatal(err)
			}
			round, err := Parse(encoded)
			if err != nil {
				t.Fatalf("parse encoded %q: %v", encoded, err)
			}
			for _, key := range []string{"type", "tag", "server", "server_port"} {
				if scalarField(t, round, key) != scalarField(t, out, key) {
					t.Fatalf("%s roundtrip = %q, want %q\nencoded=%s", key, scalarField(t, round, key), scalarField(t, out, key), encoded)
				}
			}
			assertProtocolRoundTrip(t, out, round)
		})
	}
}

func TestEncodeCleansServerCountryOverride(t *testing.T) {
	out := merge.NewOrderedMap()
	out.Set("type", "shadowsocks")
	out.Set("tag", "Relay")
	out.Set("server", "relay.example.com#CN")
	out.Set("server_port", intNumber(8388))
	out.Set("method", "aes-256-gcm")
	out.Set("password", "pw")

	encoded, err := Encode(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "%23CN") || strings.Contains(encoded, "relay.example.com#CN") {
		t.Fatalf("encoded link still contains country override: %s", encoded)
	}
	round, err := Parse(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, round, "server"); got != "relay.example.com" {
		t.Fatalf("server = %q", got)
	}
}

func assertProtocolRoundTrip(t *testing.T, want, got *merge.OrderedMap) {
	t.Helper()
	switch field(t, want, "type") {
	case "shadowsocks":
		for _, key := range []string{"method", "password"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
	case "vmess":
		for _, key := range []string{"uuid", "alter_id"} {
			if scalarField(t, got, key) != scalarField(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, scalarField(t, got, key), scalarField(t, want, key))
			}
		}
	case "vless":
		for _, key := range []string{"uuid", "flow"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
		gotReality := nested(t, nested(t, got, "tls"), "reality")
		wantReality := nested(t, nested(t, want, "tls"), "reality")
		if field(t, gotReality, "public_key") != field(t, wantReality, "public_key") {
			t.Fatalf("reality public_key = %q, want %q", field(t, gotReality, "public_key"), field(t, wantReality, "public_key"))
		}
	case "trojan", "hysteria2":
		if field(t, got, "password") != field(t, want, "password") {
			t.Fatalf("password = %q, want %q", field(t, got, "password"), field(t, want, "password"))
		}
	case "anytls":
		for _, key := range []string{"password", "idle_session_check_interval", "idle_session_timeout", "min_idle_session"} {
			if scalarField(t, got, key) != scalarField(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, scalarField(t, got, key), scalarField(t, want, key))
			}
		}
	case "shadowtls":
		for _, key := range []string{"password", "version"} {
			if scalarField(t, got, key) != scalarField(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, scalarField(t, got, key), scalarField(t, want, key))
			}
		}
	case "hysteria":
		for _, key := range []string{"auth_str", "up_mbps", "down_mbps"} {
			if scalarField(t, got, key) != scalarField(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, scalarField(t, got, key), scalarField(t, want, key))
			}
		}
	case "tuic":
		for _, key := range []string{"uuid", "password", "congestion_control"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
	case "naive":
		for _, key := range []string{"username", "password"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
		if gotQuic, _ := got.Get("quic"); gotQuic != true {
			t.Fatalf("quic = %v", gotQuic)
		}
	case "http":
		for _, key := range []string{"username", "password", "path"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
	case "socks":
		for _, key := range []string{"version", "username", "password", "network"} {
			if field(t, got, key) != field(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, field(t, got, key), field(t, want, key))
			}
		}
	}
}

// TestParsedOutboundsValidateWithKernel builds a full config from parsed
// outbounds and runs the real sing-box `check`, proving the emitted JSON is
// schema-valid (not just field-correct).
func TestParsedOutboundsValidateWithKernel(t *testing.T) {
	path, err := exec.LookPath("sing-box")
	if err != nil {
		t.Skip("sing-box not in PATH")
	}

	ssUserinfo := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:pw"))
	vmessJSON, _ := json.Marshal(map[string]any{
		"ps": "v", "add": "v.example.com", "port": "443", "id": "b831381d-6324-4d53-ad4f-8cda48b30811",
		"aid": "0", "net": "ws", "path": "/p", "host": "v.example.com", "tls": "tls",
	})
	uris := []string{
		"ss://" + ssUserinfo + "@1.2.3.4:8388#SS",
		"vmess://" + base64.StdEncoding.EncodeToString(vmessJSON),
		"vless://b831381d-6324-4d53-ad4f-8cda48b30811@vl.example.com:443?security=tls&sni=vl.example.com&type=ws&path=/w#VL",
		"trojan://pw@tr.example.com:443?security=tls&sni=tr.example.com#TR",
		"anytls://pw@any.example.com?security=tls&sni=any.example.com#ANY",
		"shadowtls://pw@st.example.com?version=3&security=tls&sni=st.example.com#ST",
		"hysteria://hy1.example.com?auth=secret&sni=hy1.example.com&upmbps=20&downmbps=30#HY1",
		"hysteria2://pw@hy.example.com:8443?sni=hy.example.com#HY",
		"tuic://b831381d-6324-4d53-ad4f-8cda48b30811:p@tu.example.com:443?sni=tu.example.com&congestion_control=bbr#TU",
	}

	outbounds := []any{}
	for _, u := range uris {
		m, err := Parse(u)
		if err != nil {
			t.Fatalf("parse %q: %v", u, err)
		}
		outbounds = append(outbounds, m)
	}
	// add a direct so route.final can resolve, plus the parsed nodes
	direct := merge.NewOrderedMap()
	direct.Set("type", "direct")
	direct.Set("tag", "direct")
	outbounds = append(outbounds, direct)

	cfg := merge.NewOrderedMap()
	logBlock := merge.NewOrderedMap()
	logBlock.Set("level", "error")
	cfg.Set("log", logBlock)
	cfg.Set("outbounds", outbounds)

	raw, err := cfg.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	pretty, _ := merge.Indent(raw)

	tmp := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(tmp, pretty, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(path, "check", "-c", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("sing-box check failed on parsed outbounds:\n%s\n--- config ---\n%s", out, pretty)
	}
}
