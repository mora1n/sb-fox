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

func TestShadowsocksOptionalFieldsRoundTrip(t *testing.T) {
	out, err := Parse("ss://" + base64.RawURLEncoding.EncodeToString([]byte("aes-256-gcm:pw")) + "@ss.example.com:443?network=tcp%2Cudp&uot=2&multiplex=%7B%22enabled%22%3Atrue%7D#SS")
	if err != nil {
		t.Fatal(err)
	}
	if got := scalarField(t, out, "network"); got != "tcp,udp" {
		t.Fatalf("network = %q", got)
	}
	if got := scalarField(t, nested(t, out, "udp_over_tcp"), "version"); got != "2" {
		t.Fatalf("udp_over_tcp.version = %q", got)
	}
	link, err := Encode(out)
	if err != nil {
		t.Fatal(err)
	}
	round, err := Parse(link)
	if err != nil {
		t.Fatalf("parse encoded link %q: %v", link, err)
	}
	if got := scalarField(t, nested(t, round, "multiplex"), "enabled"); got != "true" {
		t.Fatalf("multiplex.enabled = %q", got)
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

func TestParseHysteriaUsesSingBox14FieldNames(t *testing.T) {
	uri := "hysteria://hy.example.com:443?auth_str=secret&recv_window_conn=1000&recv_window=2000&disable_mtu_discovery=true&protocol=udp#HY"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"auth_str":              "secret",
		"recv_window_conn":      "1000",
		"recv_window":           "2000",
		"disable_mtu_discovery": "true",
		"network":               "udp",
	} {
		if got := scalarField(t, out, key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	for _, key := range []string{"connection_receive_window", "stream_receive_window", "disable_path_mtu_discovery"} {
		if _, ok := out.Get(key); ok {
			t.Fatalf("unexpected obsolete field %q in %+v", key, out)
		}
	}
}

func TestParseNaiveMemoryFields(t *testing.T) {
	uri := "naive+https://user:pass@naive.example.com:443?stream_receive_window=16MB&quic_session_receive_window=8MB#Naive"
	out, err := Parse(uri)
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, out, "stream_receive_window"); got != "16MB" {
		t.Fatalf("stream_receive_window = %q", got)
	}
	if got := field(t, out, "quic_session_receive_window"); got != "8MB" {
		t.Fatalf("quic_session_receive_window = %q", got)
	}
}

func TestVMessQUICTransportRoundTrip(t *testing.T) {
	raw := map[string]any{
		"ps": "VMess QUIC", "add": "vm.example.com", "port": "443", "id": "00000000-0000-0000-0000-000000000001",
		"aid": "0", "scy": "auto", "net": "quic", "type": "none",
	}
	body, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Parse("vmess://" + base64.StdEncoding.EncodeToString(body))
	if err != nil {
		t.Fatal(err)
	}
	transport := nested(t, out, "transport")
	if got := field(t, transport, "type"); got != "quic" {
		t.Fatalf("transport.type = %q", got)
	}
	link, err := Encode(out)
	if err != nil {
		t.Fatal(err)
	}
	round, err := Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, nested(t, round, "transport"), "type"); got != "quic" {
		t.Fatalf("roundtrip transport.type = %q", got)
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

func TestParseSnell(t *testing.T) {
	tests := []struct {
		name       string
		uri        string
		version    string
		obfs       string
		mode       string
		userKey    string
		wantServer string
	}{
		{
			name:       "v4 userinfo",
			uri:        "snell://my%20psk@snell.example.com:443?version=4&obfs=http&obfs-host=edge.example.com&reuse=true&network=tcp#Snell%204",
			version:    "4",
			obfs:       "http",
			wantServer: "snell.example.com",
		},
		{
			name:       "v5 query psk normalizes to v4",
			uri:        "snell://snell.example.com:443?psk=secret&version=5",
			version:    "4",
			wantServer: "snell.example.com",
		},
		{
			name:       "v6 query psk",
			uri:        "snell://snell.example.com:443?psk=123456789012&version=6&mode=unshaped",
			version:    "6",
			mode:       "unshaped",
			wantServer: "snell.example.com",
		},
		{
			name:       "v6 userkey and query psk",
			uri:        "snell://0123456789abcdef0123456789abcdef@snell-v6.example.com:8888?psk=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef&version=6#snell-v6-userkey",
			version:    "6",
			userKey:    "0123456789abcdef0123456789abcdef",
			wantServer: "snell-v6.example.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := Parse(tt.uri)
			if err != nil {
				t.Fatal(err)
			}
			if field(t, out, "type") != "snell" || field(t, out, "server") != tt.wantServer {
				t.Fatalf("outbound = %+v", out)
			}
			if scalarField(t, out, "version") != tt.version {
				t.Fatalf("version = %s, want %s", scalarField(t, out, "version"), tt.version)
			}
			if tt.obfs != "" && field(t, out, "obfs_mode") != tt.obfs {
				t.Fatalf("obfs_mode = %s, want %s", field(t, out, "obfs_mode"), tt.obfs)
			}
			if tt.mode != "" && field(t, out, "mode") != tt.mode {
				t.Fatalf("mode = %s, want %s", field(t, out, "mode"), tt.mode)
			}
			if tt.userKey != "" && field(t, out, "userkey") != tt.userKey {
				t.Fatalf("userkey = %s, want %s", field(t, out, "userkey"), tt.userKey)
			}
		})
	}
}

func TestParseSnellRejectsInvalidVersionsAndParameters(t *testing.T) {
	for _, uri := range []string{
		"snell://psk@snell.example.com:443#missing-version",
		"snell://psk@snell.example.com:443?version=3",
		"snell://123456789012@snell.example.com:443?version=6&obfs=http",
		"snell://psk@snell.example.com:443?version=6&mode=bad",
		"snell://short@snell.example.com:443?version=6",
		"snell://key@snell.example.com:443?psk=secret&userkey=other&version=4",
	} {
		if _, err := Parse(uri); err == nil {
			t.Fatalf("Parse(%q) succeeded", uri)
		}
	}
}

func TestParseSnellV4TLSObfs(t *testing.T) {
	out, err := Parse("snell://psk@snell.example.com:443?version=4&obfs=tls&obfs-host=example.com#Snell")
	if err != nil {
		t.Fatal(err)
	}
	if got := field(t, out, "obfs_mode"); got != "tls" {
		t.Fatalf("obfs_mode = %q, want tls", got)
	}
	if got := field(t, out, "obfs_host"); got != "example.com" {
		t.Fatalf("obfs_host = %q, want example.com", got)
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
		"anytls://pw@3.3.3.3?security=tls#C\n" +
		"snell://snell.example.com:443?psk=secret&version=5#D"
	blob := base64.StdEncoding.EncodeToString([]byte(links))
	out, err := ParseMany(blob)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 {
		t.Fatalf("parsed %d links, want 4", len(out))
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
  - name: Snell v6
    type: snell
    server: snell.example.com
    port: 17851
    psk: 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
    userkey: 0123456789abcdef0123456789abcdef
    version: 6
    udp: true
    mode: unshaped
  - name: OldSSR
    type: ssr
    server: ssr.example.com
    port: 8388
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 6 {
		t.Fatalf("parsed %d outbounds, want 6", len(out))
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
	if field(t, out[5], "type") != "snell" || scalarField(t, out[5], "version") != "6" || field(t, out[5], "userkey") != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("snell outbound = %+v", out[5])
	}
	if got := stringListField(out[5], "network"); len(got) != 2 || got[0] != "tcp" || got[1] != "udp" {
		t.Fatalf("snell network = %v", got)
	}
}

func TestParseManySurgeProfile(t *testing.T) {
	text := `
[General]
loglevel = notify

[Proxy]
"Snell, V6" = snell, snell.example.com, 17851, psk=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef, version=6, userkey=0123456789abcdef0123456789abcdef, mode=unshaped, reuse=true, network="tcp,udp"
Snell V5 = snell, snell-v5.example.com, 443, psk=snell-v5-secret, version=5, obfs=http, obfs-host=edge.example.com
SS = ss, ss.example.com, 8388, encrypt-method=aes-256-gcm, password=ss-secret
DIRECT = direct

[Proxy Group]
Proxy = select, Snell, DIRECT
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 3 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	if got := field(t, out[0], "tag"); got != "Snell, V6" {
		t.Fatalf("quoted tag = %q", got)
	}
	if field(t, out[0], "type") != "snell" || scalarField(t, out[0], "version") != "6" {
		t.Fatalf("snell v6 outbound = %+v", out[0])
	}
	if got := stringListField(out[0], "network"); len(got) != 2 || got[0] != "tcp" || got[1] != "udp" {
		t.Fatalf("snell v6 network = %v", got)
	}
	if field(t, out[1], "type") != "snell" || scalarField(t, out[1], "version") != "4" || field(t, out[1], "obfs_mode") != "http" {
		t.Fatalf("snell v5 outbound = %+v", out[1])
	}
	if field(t, out[2], "type") != "shadowsocks" || field(t, out[2], "method") != "aes-256-gcm" {
		t.Fatalf("shadowsocks outbound = %+v", out[2])
	}
}

func TestEncodeSnell(t *testing.T) {
	out := merge.NewOrderedMap()
	out.Set("type", "snell")
	out.Set("tag", "Snell v6")
	out.Set("server", "snell.example.com")
	out.Set("server_port", json.Number("17851"))
	out.Set("version", json.Number("6"))
	out.Set("psk", "0123456789abcdef0123456789abcdef")
	out.Set("userkey", "0123456789abcdef0123456789abcdef")
	out.Set("network", []any{"tcp", "udp"})
	out.Set("mode", "unshaped")
	link, err := Encode(out)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(link)
	if err != nil {
		t.Fatalf("parse encoded link %q: %v", link, err)
	}
	if field(t, parsed, "type") != "snell" || scalarField(t, parsed, "version") != "6" || field(t, parsed, "psk") != "0123456789abcdef0123456789abcdef" || field(t, parsed, "userkey") != "0123456789abcdef0123456789abcdef" || field(t, parsed, "mode") != "unshaped" {
		t.Fatalf("parsed encoded Snell = %+v", parsed)
	}
	if got := stringListField(parsed, "network"); len(got) != 2 || got[0] != "tcp" || got[1] != "udp" {
		t.Fatalf("parsed encoded Snell network = %v", got)
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
      strategy: prefer_ipv4
      disable-cache: true
      rewrite_ttl: 60
      client-subnet: 192.0.2.0/24
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
	if got := field(t, resolver, "strategy"); got != "prefer_ipv4" {
		t.Fatalf("domain_resolver.strategy = %q", got)
	}
	if got := scalarField(t, resolver, "disable_cache"); got != "true" {
		t.Fatalf("domain_resolver.disable_cache = %q", got)
	}
	if got := scalarField(t, resolver, "rewrite_ttl"); got != "60" {
		t.Fatalf("domain_resolver.rewrite_ttl = %q", got)
	}
	if got := field(t, resolver, "client_subnet"); got != "192.0.2.0/24" {
		t.Fatalf("domain_resolver.client_subnet = %q", got)
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

func TestParseClashYAMLMapsSingBox14ProtocolFields(t *testing.T) {
	text := `
proxies:
  - name: SS14
    type: ss
    server: ss.example.com
    port: 443
    cipher: aes-256-gcm
    password: pw
    plugin: obfs-local
    plugin-opts: obfs=http
    network: [tcp, udp]
    udp-over-tcp:
      enabled: true
      version: 2
    multiplex:
      enabled: true
      protocol: smux
  - name: VM14
    type: vmess
    server: vm.example.com
    port: 443
    uuid: 00000000-0000-0000-0000-000000000001
    cipher: auto
    network: ws
    ws-opts:
      path: /vm-ws
      headers:
        Host: vm.example.com
    packet-encoding: xudp
    global-padding: true
    authenticated-length: true
  - name: HY214
    type: hysteria2
    server: hy2.example.com
    port: 443
    password: pw
    network: udp
    brutal-debug: true
  - name: TU14
    type: tuic
    server: tu.example.com
    port: 443
    uuid: 00000000-0000-0000-0000-000000000002
    password: pw
    network: udp
    udp-over-stream: true
    zero-rtt-handshake: true
  - name: ANY14
    type: anytls
    server: any.example.com
    port: 443
    password: pw
    client-metadata: test-meta
  - name: NV14
    type: naive
    server: naive.example.com
    port: 443
    username: user
    password: pw
    insecure-concurrency: 2
    extra-headers:
      User-Agent: test
    stream-receive-window: 1000000
    quic: true
    quic-congestion-control: bbr
    quic-session-receive-window: 2000000
  - name: TLS14
    type: http
    server: tls.example.com
    port: 443
    tls: true
    disable-sni: true
    min-version: "1.2"
    max-version: "1.3"
    cipher-suites: [TLS_AES_128_GCM_SHA256]
    curve-preferences: [X25519]
    fragment: true
    fragment-fallback-delay: 500ms
    record-fragment: true
    client-fingerprint: chrome
`
	out, warnings, err := ParseManyWithWarnings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(out) != 7 {
		t.Fatalf("warnings=%v out=%d", warnings, len(out))
	}
	ss := out[0]
	if field(t, ss, "plugin") != "obfs-local" || field(t, ss, "plugin_opts") != "obfs=http" {
		t.Fatalf("shadowsocks plugin fields = %+v", ss)
	}
	if scalarField(t, ss, "network") != "[tcp udp]" {
		t.Fatalf("shadowsocks network = %q", scalarField(t, ss, "network"))
	}
	if scalarField(t, nested(t, ss, "udp_over_tcp"), "version") != "2" {
		t.Fatalf("shadowsocks udp_over_tcp = %+v", nested(t, ss, "udp_over_tcp"))
	}
	vm := out[1]
	if field(t, vm, "packet_encoding") != "xudp" || scalarField(t, vm, "global_padding") != "true" || scalarField(t, vm, "authenticated_length") != "true" {
		t.Fatalf("vmess 1.14 fields = %+v", vm)
	}
	if _, ok := vm.Get("network"); ok {
		t.Fatalf("vmess transport network leaked into outbound network: %+v", vm)
	}
	if transport := nested(t, vm, "transport"); field(t, transport, "type") != "ws" || field(t, transport, "path") != "/vm-ws" {
		t.Fatalf("vmess transport = %+v", transport)
	}
	hy2 := out[2]
	if scalarField(t, hy2, "brutal_debug") != "true" {
		t.Fatalf("hysteria2 1.14 fields = %+v", hy2)
	}
	tu := out[3]
	if scalarField(t, tu, "udp_over_stream") != "true" || scalarField(t, tu, "zero_rtt_handshake") != "true" {
		t.Fatalf("tuic 1.14 fields = %+v", tu)
	}
	if field(t, out[4], "client_metadata") != "test-meta" {
		t.Fatalf("anytls client_metadata = %q", field(t, out[4], "client_metadata"))
	}
	nv := out[5]
	if scalarField(t, nv, "insecure_concurrency") != "2" || scalarField(t, nv, "quic_session_receive_window") != "2000000" {
		t.Fatalf("naive 1.14 fields = %+v", nv)
	}
	tls := nested(t, out[6], "tls")
	if scalarField(t, tls, "disable_sni") != "true" || field(t, tls, "min_version") != "1.2" {
		t.Fatalf("TLS 1.14 fields = %+v", tls)
	}
	if scalarField(t, tls, "fragment") != "true" || scalarField(t, tls, "record_fragment") != "true" {
		t.Fatalf("TLS advanced fields = %+v", tls)
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
		"snell://snell.example.com:17851?psk=0123456789abcdef0123456789abcdef&version=6&userkey=0123456789abcdef0123456789abcdef&network=tcp%2Cudp&mode=unshaped#Snell",
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
	case "snell":
		for _, key := range []string{"version", "psk", "userkey", "mode", "network"} {
			if scalarField(t, got, key) != scalarField(t, want, key) {
				t.Fatalf("%s = %q, want %q", key, scalarField(t, got, key), scalarField(t, want, key))
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
		"snell://snell.example.com:17851?psk=0123456789abcdef0123456789abcdef&version=6&mode=unshaped#SNELL",
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
